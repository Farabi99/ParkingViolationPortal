package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"path/filepath"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/google/uuid"
	
	"parking-portal/pkg/logger"
	"parking-portal/pkg/middleware"
	"parking-portal/pkg/rabbitmq"
	"parking-portal/pkg/shutdown"
	"parking-portal/pkg/types"
)

func main() {
	ctx := context.Background()
	logger.Info(ctx, "Starting Violation Service")

	dbURL := getEnv("DB_URL", "postgres://root:rootpassword@localhost:5432/parking_portal?sslmode=disable")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error(ctx, "Failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec("SET search_path TO violation_schema")
	if err != nil {
		logger.Error(ctx, "Failed to set search_path", "error", err)
		os.Exit(1)
	}

	minioClient, err := minio.New(getEnv("MINIO_ENDPOINT", "localhost:9000"), &minio.Options{
		Creds:  credentials.NewStaticV4(getEnv("MINIO_USER", "minioadmin"), getEnv("MINIO_PASS", "minioadmin"), ""),
		Secure: false,
	})
	if err != nil {
		logger.Error(ctx, "Failed to connect Minio", "error", err)
		os.Exit(1)
	}

	bucketName := "violations"
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err == nil && !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			logger.Error(ctx, "Failed to create bucket", "error", err)
		}
	}

	rmq, err := rabbitmq.Connect(getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer rmq.Close()

	err = rmq.SetupQueue("fine_calculated", "fine_calculated_dlq")
	err = rmq.SetupQueue("payment_processed", "payment_processed_dlq")

	go consumeFines(ctx, rmq, db)
	go consumePayments(ctx, rmq, db)

	r := mux.NewRouter()
	r.Use(middleware.CorrelationID)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20) // 10 MB
		if err != nil {
			http.Error(w, "Parse err", http.StatusBadRequest)
			return
		}

		plate := strings.TrimSpace(r.FormValue("license_plate"))
		vType := strings.TrimSpace(r.FormValue("type"))
		loc := strings.TrimSpace(r.FormValue("location"))
		
		if plate == "" || vType == "" || loc == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		ruleURL := getEnv("RULE_SERVICE_URL", "http://rule_service:8083") + "/active"
		if !strings.Contains(ruleURL, "http") {
			ruleURL = "http://" + ruleURL
		}
		// If running locally, you might need localhost, but in docker-compose it's rule_service:8083
		// We'll rely on env var or fallback
		res, ruleErr := http.Get(ruleURL)
		if ruleErr == nil && res.StatusCode == http.StatusOK {
			var activeRules struct {
				Rules struct {
					BaseAmount map[string]float64 `json:"base_amount"`
				} `json:"rules"`
			}
			json.NewDecoder(res.Body).Decode(&activeRules)
			res.Body.Close()
			if _, exists := activeRules.Rules.BaseAmount[vType]; !exists {
				http.Error(w, "Invalid violation type", http.StatusBadRequest)
				return
			}
		}

		file, handler, err := r.FormFile("photo")
		photoURL := ""
		if err == nil {
			defer file.Close()
			
			// Validate file content type by magic bytes
			buffer := make([]byte, 512)
			n, _ := file.Read(buffer)
			contentType := http.DetectContentType(buffer[:n])
			if !strings.HasPrefix(contentType, "image/") {
				http.Error(w, "Invalid file format, must be an image", http.StatusBadRequest)
				return
			}
			file.Seek(0, 0) // reset file pointer

			fileName := uuid.New().String() + filepath.Ext(handler.Filename)
			
			// Save to local disk temporarily or upload directly from reader
			_, err = minioClient.PutObject(r.Context(), bucketName, fileName, file, handler.Size, minio.PutObjectOptions{ContentType: contentType})
			if err == nil {
				photoURL = fileName
			} else {
				logger.Error(r.Context(), "Minio upload failed", "error", err)
			}
		}

		var id string
		t := time.Now()
		if tsVal := r.FormValue("timestamp"); tsVal != "" {
			if parsed, err := time.Parse(time.RFC3339, tsVal); err == nil {
				t = parsed
			}
		}

		err = db.QueryRowContext(r.Context(),
			"INSERT INTO violations (license_plate, violation_type, location, timestamp, photo_url) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			plate, vType, loc, t, photoURL,
		).Scan(&id)

		if err != nil {
			http.Error(w, "DB err", http.StatusInternalServerError)
			return
		}

		// Publish event
		rmq.Publish(r.Context(), "violation_created", types.ViolationCreatedEvent{
			ViolationID: id,
			LicensePlate: plate,
			Type: vType,
			Timestamp: t,
		})

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "timestamp": t})
	}).Methods("POST")

	r.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		limit := 10

		role := r.Header.Get("X-User-Role")
		userID := r.Header.Get("X-User-ID")

		whereClause := ""
		args := []interface{}{}
		argCount := 1

		if role == "MEMBER" && userID != "" {
			whereClause = fmt.Sprintf("WHERE license_plate IN (SELECT license_plate FROM auth_schema.user_vehicles WHERE user_id = $%d)", argCount)
			args = append(args, userID)
			argCount++
		}

		if cursor != "" {
			if whereClause == "" {
				whereClause = "WHERE"
			} else {
				whereClause += " AND"
			}
			whereClause += fmt.Sprintf(" created_at < (SELECT created_at FROM violations WHERE id = $%d)", argCount)
			args = append(args, cursor)
			argCount++
		}

		limitClause := fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argCount)
		args = append(args, limit)

		query := "SELECT id, license_plate, violation_type, location, timestamp, photo_url, fine_amount, status, applied_rule_set_version FROM violations " + whereClause + limitClause
		
		rows, err := db.QueryContext(r.Context(), query, args...)

		if err != nil {
			http.Error(w, "DB err", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Vol struct {
			ID string `json:"id"`
			LicensePlate string `json:"license_plate"`
			Type string `json:"type"`
			Location string `json:"location"`
			Timestamp time.Time `json:"timestamp"`
			PhotoURL string `json:"photo_url"`
			FineAmount *float64 `json:"fine_amount"`
			Status string `json:"status"`
			AppliedRuleSetVersion *int `json:"applied_rule_set_version"`
		}

		var vols []Vol
		var lastID string
		for rows.Next() {
			var v Vol
			rows.Scan(&v.ID, &v.LicensePlate, &v.Type, &v.Location, &v.Timestamp, &v.PhotoURL, &v.FineAmount, &v.Status, &v.AppliedRuleSetVersion)
			
			if v.PhotoURL != "" {
				// Generate presigned URL
				reqParams := make(url.Values)
				presignedURL, err := minioClient.PresignedGetObject(r.Context(), bucketName, v.PhotoURL, time.Hour, reqParams)
				if err == nil {
					browserURL := getEnv("MINIO_BROWSER_URL", "localhost:9000")
					v.PhotoURL = strings.Replace(presignedURL.String(), getEnv("MINIO_ENDPOINT", "localhost:9000"), browserURL, 1)
				}
			}
			
			vols = append(vols, v)
			lastID = v.ID
		}

		resp := map[string]interface{}{
			"data": vols,
		}
		if len(vols) == limit {
			resp["next_cursor"] = lastID
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}).Methods("GET")

	srv := &http.Server{
		Addr:    ":8082",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Violation service failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown.Graceful(ctx, 5*time.Second, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

func consumeFines(ctx context.Context, rmq *rabbitmq.Client, db *sql.DB) {
	msgs, _ := rmq.Consume("fine_calculated")
	for d := range msgs {
		var ev types.FineCalculatedEvent
		json.Unmarshal(d.Body, &ev)
		
		db.Exec("UPDATE violations SET fine_amount = $1, applied_rule_set_version = $2 WHERE id = $3", ev.FineAmount, ev.AppliedRuleSetVer, ev.ViolationID)
		d.Ack(false)
	}
}

func consumePayments(ctx context.Context, rmq *rabbitmq.Client, db *sql.DB) {
	msgs, _ := rmq.Consume("payment_processed")
	for d := range msgs {
		var ev types.PaymentProcessedEvent
		json.Unmarshal(d.Body, &ev)
		
		if ev.Status == "PAID" {
			db.Exec("UPDATE violations SET status = 'PAID' WHERE id = $1", ev.ViolationID)
			
			var plate string
			db.QueryRow("SELECT license_plate FROM violations WHERE id = $1", ev.ViolationID).Scan(&plate)

			db.Exec("INSERT INTO transactions (license_plate, violation_id, amount, status, external_tx_id) VALUES ($1, $2, $3, $4, $5)",
				plate, ev.ViolationID, ev.Amount, ev.Status, ev.TransactionID)
		}
		d.Ack(false)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

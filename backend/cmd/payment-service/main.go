package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"parking-portal/pkg/idempotency"
	"parking-portal/pkg/logger"
	"parking-portal/pkg/middleware"
	"parking-portal/pkg/rabbitmq"
	"parking-portal/pkg/shutdown"
	"parking-portal/pkg/types"
)

type PaymentRequest struct {
	ViolationID string  `json:"violation_id"`
	Amount      float64 `json:"amount"`
	Scenario    string  `json:"scenario"` // "success" or "failed"
}

type PaymentResponse struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

func main() {
	ctx := context.Background()
	logger.Info(ctx, "Starting Payment Service")

	rmq, err := rabbitmq.Connect(getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer rmq.Close()

	dbURL := getEnv("DB_URL", "postgres://root:rootpassword@localhost:5432/parking_portal?sslmode=disable")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error(ctx, "Failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	checker := idempotency.NewChecker(getEnv("REDIS_URL", "redis://localhost:6379/0"))

	r := mux.NewRouter()
	r.Use(middleware.CorrelationID)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		idemKey := r.Header.Get("Idempotency-Key")
		if idemKey == "" {
			http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
			return
		}

		// Check idempotency
		ok, err := checker.CheckAndSet(r.Context(), idemKey, 24*time.Hour)
		if err != nil {
			http.Error(w, "Idempotency check failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			// Already processed or in progress. Return cached result if complete.
			cached, err := checker.GetResult(r.Context(), idemKey)
			if err == nil && cached != "processing" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(cached))
				return
			}
			http.Error(w, "Request already processed or in progress", http.StatusConflict)
			return
		}

		var req PaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		role := r.Header.Get("X-User-Role")
		userID := r.Header.Get("X-User-ID")

		if role == "MEMBER" {
			var count int
			err := db.QueryRowContext(r.Context(), `
				SELECT count(*) FROM violation_schema.violations v
				JOIN auth_schema.user_vehicles uv ON v.license_plate = uv.license_plate
				WHERE v.id = $1 AND uv.user_id = $2
			`, req.ViolationID, userID).Scan(&count)

			if err != nil || count == 0 {
				http.Error(w, "Forbidden: you do not own this violation", http.StatusForbidden)
				return
			}
		}

		// Mock payment logic
		status := "FAILED"
		txID := uuid.New().String()
		if req.Scenario == "success" {
			status = "PAID"
		}

		resp := PaymentResponse{
			Status:        status,
			TransactionID: txID,
		}

		// Publish event
		rmq.Publish(r.Context(), "payment_processed", types.PaymentProcessedEvent{
			ViolationID:   req.ViolationID,
			TransactionID: txID,
			Amount:        req.Amount,
			Status:        status,
		})

		respBytes, _ := json.Marshal(resp)
		
		// Mark idempotency complete
		checker.MarkComplete(r.Context(), idemKey, string(respBytes), 24*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}).Methods("POST")

	srv := &http.Server{
		Addr:    ":8084",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Payment service failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown.Graceful(ctx, 5*time.Second, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

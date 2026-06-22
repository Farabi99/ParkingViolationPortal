package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"parking-portal/pkg/logger"
	"parking-portal/pkg/middleware"
	"parking-portal/pkg/rabbitmq"
	"parking-portal/pkg/shutdown"
	"parking-portal/pkg/types"
)

type RuleSet struct {
	Version int             `json:"version"`
	Rules   json.RawMessage `json:"rules"`
}

type ParsedRules struct {
	BaseAmount       map[string]float64 `json:"base_amount"`
	TimeMultiplier   []TimeRule         `json:"time_multiplier"`
	RepeatMultiplier []RepeatRule       `json:"repeat_multiplier"`
}

type TimeRule struct {
	Start      string  `json:"start"`
	End        string  `json:"end"`
	Multiplier float64 `json:"multiplier"`
}

type RepeatRule struct {
	PriorUnpaid int     `json:"prior_unpaid"`
	Multiplier  float64 `json:"multiplier"`
}

func main() {
	ctx := context.Background()
	logger.Info(ctx, "Starting Rule Engine Service")

	dbURL := getEnv("DB_URL", "postgres://root:rootpassword@localhost:5432/parking_portal?sslmode=disable")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error(ctx, "Failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec("SET search_path TO rule_schema")
	if err != nil {
		logger.Error(ctx, "Failed to set search_path", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: getEnv("REDIS_ADDR", "localhost:6379"),
	})

	rmq, err := rabbitmq.Connect(getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		logger.Error(ctx, "Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer rmq.Close()

	err = rmq.SetupQueue("violation_created", "violation_created_dlq")
	if err != nil {
		logger.Error(ctx, "Failed to setup queue", "error", err)
	}

	// Start Consumer
	go consumeViolations(ctx, rmq, db, redisClient)

	r := mux.NewRouter()
	r.Use(middleware.CorrelationID)

	r.HandleFunc("/active", func(w http.ResponseWriter, r *http.Request) {
		rs, err := getActiveRuleSet(r.Context(), db, redisClient)
		if err != nil {
			http.Error(w, "Failed to fetch rules", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rs)
	}).Methods("GET")

	r.HandleFunc("/{version:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]

		var rs RuleSet
		err := db.QueryRowContext(r.Context(), "SELECT version, rules FROM rule_sets WHERE version = $1", version).Scan(&rs.Version, &rs.Rules)
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rs)
	}).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var newRules json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&newRules); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		var parsed ParsedRules
		if err := json.Unmarshal(newRules, &parsed); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		minutes := make([]bool, 1440)
		for _, tr := range parsed.TimeMultiplier {
			startT, err1 := time.Parse("15:04", tr.Start)
			endT, err2 := time.Parse("15:04", tr.End)
			if err1 != nil || err2 != nil {
				continue
			}

			startMin := startT.Hour()*60 + startT.Minute()
			endMin := endT.Hour()*60 + endT.Minute()

			hasOverlap := false
			checkAndSet := func(m int) {
				if minutes[m] {
					hasOverlap = true
				}
				minutes[m] = true
			}

			if startMin <= endMin {
				for m := startMin; m < endMin; m++ {
					checkAndSet(m)
				}
			} else {
				for m := startMin; m < 1440; m++ {
					checkAndSet(m)
				}
				for m := 0; m < endMin; m++ {
					checkAndSet(m)
				}
			}

			if hasOverlap {
				http.Error(w, "Time rules cannot overlap", http.StatusBadRequest)
				return
			}
		}

		// Get max version
		var currentVer int
		db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM rule_sets").Scan(&currentVer)
		nextVer := currentVer + 1

		// Insert new
		_, err := db.ExecContext(r.Context(),
			"INSERT INTO rule_sets (version, active_from, rules) VALUES ($1, $2, $3)",
			nextVer, time.Now(), newRules,
		)
		if err != nil {
			http.Error(w, "Failed to save rule", http.StatusInternalServerError)
			return
		}

		// Invalidate cache
		redisClient.Del(r.Context(), "active_ruleset")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"version": nextVer})
	}).Methods("POST")

	srv := &http.Server{
		Addr:    ":8083",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Rule service failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown.Graceful(ctx, 5*time.Second, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

func getActiveRuleSet(ctx context.Context, db *sql.DB, rdb *redis.Client) (*RuleSet, error) {
	// Try cache
	val, err := rdb.Get(ctx, "active_ruleset").Result()
	if err == nil && val != "" {
		var rs RuleSet
		json.Unmarshal([]byte(val), &rs)
		return &rs, nil
	}

	// Fetch from DB
	var rs RuleSet
	err = db.QueryRowContext(ctx, "SELECT version, rules FROM rule_sets ORDER BY version DESC LIMIT 1").Scan(&rs.Version, &rs.Rules)
	if err != nil {
		return nil, err
	}

	// Set cache
	b, _ := json.Marshal(rs)
	rdb.Set(ctx, "active_ruleset", string(b), time.Hour*24)

	return &rs, nil
}

func consumeViolations(ctx context.Context, rmq *rabbitmq.Client, db *sql.DB, rdb *redis.Client) {
	msgs, err := rmq.Consume("violation_created")
	if err != nil {
		logger.Error(ctx, "Failed to consume", "error", err)
		return
	}

	for d := range msgs {
		msgCtx := context.WithValue(context.Background(), logger.CorrelationIDKey, d.CorrelationId)
		logger.Info(msgCtx, "Received violation to calculate fine")

		var ev types.ViolationCreatedEvent
		if err := json.Unmarshal(d.Body, &ev); err != nil {
			logger.Error(msgCtx, "Invalid message format", "error", err)
			d.Nack(false, false) // send to DLQ
			continue
		}

		rs, err := getActiveRuleSet(msgCtx, db, rdb)
		if err != nil {
			logger.Error(msgCtx, "Failed to get active rule set", "error", err)
			d.Nack(false, true) // requeue
			continue
		}

		var parsed ParsedRules
		json.Unmarshal(rs.Rules, &parsed)

		// 1. Base Amount
		base, ok := parsed.BaseAmount[ev.Type]
		if !ok {
			logger.Error(msgCtx, "Unknown violation type", "type", ev.Type)
			d.Nack(false, false) // send to DLQ
			continue
		}

		// 2. Time Multiplier
		timeMult := 1.0
		violationHour := ev.Timestamp.Local().Format("15:04")
		for _, tr := range parsed.TimeMultiplier {
			if isTimeInWindow(violationHour, tr.Start, tr.End) {
				timeMult = tr.Multiplier
				break
			}
		}

		// 3. Repeat Multiplier
		priorUnpaid := fetchPriorUnpaid(msgCtx, db, ev.LicensePlate, ev.Timestamp)
		
		// Sort by prior unpaid ascending to ensure the highest applicable threshold overwrites previous ones
		sortedRepeatRules := make([]RepeatRule, len(parsed.RepeatMultiplier))
		copy(sortedRepeatRules, parsed.RepeatMultiplier)
		for i := 0; i < len(sortedRepeatRules); i++ {
			for j := i + 1; j < len(sortedRepeatRules); j++ {
				if sortedRepeatRules[i].PriorUnpaid > sortedRepeatRules[j].PriorUnpaid {
					sortedRepeatRules[i], sortedRepeatRules[j] = sortedRepeatRules[j], sortedRepeatRules[i]
				}
			}
		}

		repeatMult := 1.0
		for _, rr := range sortedRepeatRules {
			if priorUnpaid >= rr.PriorUnpaid {
				repeatMult = rr.Multiplier
			}
		}

		fine := base * timeMult * repeatMult

		// Publish result
		out := types.FineCalculatedEvent{
			ViolationID:       ev.ViolationID,
			FineAmount:        fine,
			AppliedRuleSetVer: rs.Version,
		}
		
		rmq.Publish(msgCtx, "fine_calculated", out)
		d.Ack(false)
	}
}

func isTimeInWindow(t, start, end string) bool {
	// Simple string comparison works for HH:MM if standard
	if start <= end {
		return t >= start && t <= end
	}
	// Crosses midnight e.g. 22:00 to 06:00
	return t >= start || t <= end
}

func fetchPriorUnpaid(ctx context.Context, db *sql.DB, licensePlate string, currentViolationTimestamp time.Time) int {
	var count int
	ninetyDaysAgo := currentViolationTimestamp.AddDate(0, 0, -90)
	err := db.QueryRowContext(ctx, "SELECT count(*) FROM violation_schema.violations WHERE license_plate = $1 AND status = 'UNPAID' AND timestamp < $2 AND timestamp >= $3", licensePlate, currentViolationTimestamp, ninetyDaysAgo).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

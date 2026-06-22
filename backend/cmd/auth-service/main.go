package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/lib/pq"
	"parking-portal/pkg/logger"
	"parking-portal/pkg/middleware"
	"parking-portal/pkg/shutdown"
)

var jwtSecret = []byte(getEnv("JWT_SECRET", "super_secret_key_change_in_prod"))

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Roles    string `json:"roles"`
}

func main() {
	ctx := context.Background()
	logger.Info(ctx, "Starting Auth Service")

	dbURL := getEnv("DB_URL", "postgres://root:rootpassword@localhost:5432/parking_portal?sslmode=disable")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error(ctx, "Failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Ensure we only use auth_schema
	_, err = db.Exec("SET search_path TO auth_schema")
	if err != nil {
		logger.Error(ctx, "Failed to set search_path", "error", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.Use(middleware.CorrelationID)

	rdb := redis.NewClient(&redis.Options{
		Addr: getEnv("REDIS_ADDR", "localhost:6379"),
	})

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var creds Credentials
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		rlKey := "login_attempts:" + creds.Username
		attempts, err := rdb.Get(r.Context(), rlKey).Int()
		if err == nil && attempts >= 5 {
			http.Error(w, "Too many login attempts", http.StatusTooManyRequests)
			return
		}

		var u User
		var hashedpass string
		err = db.QueryRow("SELECT id, username, hashedpass, roles FROM users WHERE username = $1", creds.Username).Scan(&u.ID, &u.Username, &hashedpass, &u.Roles)
		if err != nil {
			rdb.Incr(r.Context(), rlKey)
			rdb.Expire(r.Context(), rlKey, time.Minute)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(hashedpass), []byte(creds.Password))
		if err != nil {
			rdb.Incr(r.Context(), rlKey)
			rdb.Expire(r.Context(), rlKey, time.Minute)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		rdb.Del(r.Context(), rlKey)

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":  u.ID,
			"username": u.Username,
			"roles":    u.Roles,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  time.Now().Add(time.Hour * 24),
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"role": u.Roles})
	}).Methods("POST")

	r.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour),
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")

	srv := &http.Server{
		Addr:    ":8081",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Auth service failed", "error", err)
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

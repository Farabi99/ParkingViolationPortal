package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"parking-portal/pkg/logger"
	"parking-portal/pkg/middleware"
	"parking-portal/pkg/shutdown"
)

func main() {
	ctx := context.Background()
	logger.Info(ctx, "Starting API Gateway")

	// Get target URLs from env or use defaults (for local docker-compose)
	authURL := getEnv("AUTH_SERVICE_URL", "http://localhost:8081")
	violationURL := getEnv("VIOLATION_SERVICE_URL", "http://localhost:8082")
	ruleURL := getEnv("RULE_SERVICE_URL", "http://localhost:8083")
	paymentURL := getEnv("PAYMENT_SERVICE_URL", "http://localhost:8084")

	var jwtSecret = []byte(getEnv("JWT_SECRET", "super_secret_key_change_in_prod"))

	validateJWT := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/auth/login") {
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/auth/logout") {
				next.ServeHTTP(w, r)
				return
			}
			
			var tokenString string
			cookie, err := r.Cookie("token")
			if err == nil {
				tokenString = cookie.Value
			} else {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenString == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return jwtSecret, nil
			})

			if err != nil || !token.Valid {
				logger.Error(r.Context(), "JWT validation failed", "error", err, "token", tokenString)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				r.Header.Set("X-User-ID", fmt.Sprintf("%v", claims["user_id"]))
				r.Header.Set("X-User-Role", fmt.Sprintf("%v", claims["roles"]))
			}

			next.ServeHTTP(w, r)
		})
	}

	r := mux.NewRouter()
	r.Use(middleware.CorrelationID)
	r.Use(validateJWT)

	// Setup Reverse Proxies
	setupProxy(r, "/api/auth", authURL)
	setupProxy(r, "/api/violations", violationURL)
	setupProxy(r, "/api/transactions", violationURL)
	setupProxy(r, "/api/rules", ruleURL)
	setupProxy(r, "/api/payment", paymentURL)

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "API Gateway failed to start", "error", err)
			os.Exit(1)
		}
	}()

	shutdown.Graceful(ctx, 5*time.Second, func(ctx context.Context) error {
		logger.Info(ctx, "Shutting down API Gateway HTTP server")
		return srv.Shutdown(ctx)
	})
}

func setupProxy(r *mux.Router, pathPrefix, targetURL string) {
	url, err := url.Parse(targetURL)
	if err != nil {
		panic("invalid target url: " + targetURL)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)

	// Modify the request before it's sent to the target
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// We can strip the prefix or just forward it.
		// For simplicity, we forward the exact path and the downstream service handles it.
		// If we want downstream to mount at root, we would strip prefix here.
		// Let's strip the prefix so downstream services can just handle `/login` instead of `/api/auth/login`.
		req.URL.Path = strings.TrimPrefix(req.URL.Path, pathPrefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}

	r.PathPrefix(pathPrefix).Handler(proxy)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

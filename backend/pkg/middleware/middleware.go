package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"parking-portal/pkg/logger"
)

func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(string(logger.CorrelationIDKey))
		if reqID == "" {
			reqID = uuid.New().String()
		}

		// Add to context
		ctx := context.WithValue(r.Context(), logger.CorrelationIDKey, reqID)
		r = r.WithContext(ctx)

		// Forward header
		w.Header().Set(string(logger.CorrelationIDKey), reqID)

		next.ServeHTTP(w, r)
	})
}

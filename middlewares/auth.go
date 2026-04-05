package middlewares

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jojipanackal/rigo-api/models"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(authModel *models.AuthModel) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metadata, err := authModel.ExtractTokenMetadata(r)
			if err != nil {
				log.Printf("AuthMiddleware: token error: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}

			userId, err := authModel.FetchAuth(metadata)
			if err != nil {
				log.Printf("AuthMiddleware: redis fetch error: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "session expired"})
				return
			}

			log.Printf("AuthMiddleware: user %d authenticated", userId)
			ctx := context.WithValue(r.Context(), UserIDKey, userId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) int64 {
	if userId, ok := ctx.Value(UserIDKey).(int64); ok {
		return userId
	}
	return 0
}

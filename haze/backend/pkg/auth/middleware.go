package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/cotton-msg/haze/backend/pkg/utils"
)

type contextKey string

const ClaimsKey contextKey = "claims"

func JWTMiddleware(cfg *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.ErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				utils.ErrorResponse(w, http.StatusUnauthorized, "invalid authorization header")
				return
			}

			claims, err := ValidateAccessToken(cfg, parts[1])
			if err != nil {
				utils.ErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WSJWTMiddleware проверяет токен из query-параметра token (WebSocket не может слать заголовки).
func WSJWTMiddleware(cfg *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			claims, err := ValidateAccessToken(cfg, token)
			if err != nil {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

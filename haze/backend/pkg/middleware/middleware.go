package middleware

import (
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	logger := log.New(os.Stdout, "[middleware] ", log.LstdFlags)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", rid)
		r.Header.Set("X-Request-ID", rid)
		logger.Printf("request: %s %s [%s]", r.Method, r.URL.Path, rid)
		next.ServeHTTP(w, r)
	})
}

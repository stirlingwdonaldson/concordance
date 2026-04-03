package router

import (
	"log"
	"net/http"
	"time"

	"concordance/services/api-go/internal/http/handlers"
)

func New(health handlers.HealthHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.GetHealth)
	mux.HandleFunc("GET /ready", health.GetReady)

	return withRequestLog(mux)
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s duration_ms=%d", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

package middleware

import (
	"net/http"
	"os"

	"github.com/go-chi/cors"
)

func CORS() func(next http.Handler) http.Handler {
	allowedOrigins := []string{"http://localhost:*", "http://127.0.0.1:*", "https://localhost:*"}
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "HX-Request", "HX-Target", "HX-Current-URL", "HX-Trigger", "HX-Trigger-Name"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

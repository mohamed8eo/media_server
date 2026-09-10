package middleware

import (
	"context"
	"net/http"
	"os"

	"mediaserver/internal/auth"
	"mediaserver/internal/database"

	"github.com/google/uuid"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(db database.Service) func(http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authenticateRequest(w, r, db, jwtService)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GuestMiddleware(db database.Service) func(http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := authenticateRequest(w, r, db, jwtService); ok {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UIAuthMiddleware(db database.Service) func(http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authenticateRequest(w, r, db, jwtService)
			if !ok {
				http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateRequest accepts a valid access token, or silently refreshes
// from the refresh_token cookie when the access token is missing/expired.
func authenticateRequest(w http.ResponseWriter, r *http.Request, db database.Service, jwtService *auth.JWT) (uuid.UUID, bool) {
	if cookie, err := r.Cookie("access_token"); err == nil && cookie != nil && cookie.Value != "" {
		if claims, err := jwtService.ValidateToken(cookie.Value); err == nil && claims.Type == "access" {
			return claims.UserID, true
		}
	}

	userID, _, ok := auth.RefreshAccessFromCookie(w, r, db, jwtService)
	return userID, ok
}

package middleware

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"mediaserver/internal/auth"
	"mediaserver/internal/database"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(next http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				http.Error(w, "missing access token", http.StatusUnauthorized)
				return
			}
			http.Error(w, "failed to read access token", http.StatusBadRequest)
			return
		}

		claims, err := jwtService.ValidateToken(cookie.Value)
		if err != nil {
			http.Error(w, "invalid or expired access token", http.StatusUnauthorized)
			return
		}

		if claims.Type != "access" {
			http.Error(w, "invalid token type", http.StatusUnauthorized)
			return

		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GuestMiddleware(db database.Service) func(http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie("access_token"); err == nil && cookie != nil && cookie.Value != "" {
				if claims, err := jwtService.ValidateToken(cookie.Value); err == nil && claims.Type == "access" {
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
			}

			if cookie, err := r.Cookie("refresh_token"); err == nil && cookie != nil && cookie.Value != "" {
				if claims, err := jwtService.ValidateToken(cookie.Value); err == nil && claims.Type == "refresh" {
					if storedToken, err := db.GetRefreshToken(cookie.Value); err == nil && !storedToken.Revoked && time.Now().Before(storedToken.ExpiresAt) {
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func UIAuthMiddleware(db database.Service) func(http.Handler) http.Handler {
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie("access_token"); err == nil && cookie != nil && cookie.Value != "" {
				if claims, err := jwtService.ValidateToken(cookie.Value); err == nil && claims.Type == "access" {
					ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			if cookie, err := r.Cookie("refresh_token"); err == nil && cookie != nil && cookie.Value != "" {
				if claims, err := jwtService.ValidateToken(cookie.Value); err == nil && claims.Type == "refresh" {
					if storedToken, err := db.GetRefreshToken(cookie.Value); err == nil && !storedToken.Revoked && time.Now().Before(storedToken.ExpiresAt) {
						if accessToken, _, err := jwtService.GenerateAccessToken(claims.UserID); err == nil {
							http.SetCookie(w, &http.Cookie{
								Name:     "access_token",
								Value:    accessToken,
								Path:     "/",
								HttpOnly: true,
								Secure:   os.Getenv("APP_ENV") != "local",
								SameSite: http.SameSiteLaxMode,
								MaxAge:   int(auth.AccessTokenDuration.Seconds()),
							})
							ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}

			http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		})
	}
}

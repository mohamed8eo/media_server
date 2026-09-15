package auth

import (
	"net/http"
	"strings"
	"time"

	"mediaserver/internal/database"

	"github.com/google/uuid"
)

// RefreshAccessFromCookie validates the refresh_token cookie against the DB
// and issues a new access_token cookie. Returns the user ID and new access token.
func RefreshAccessFromCookie(w http.ResponseWriter, r *http.Request, db database.Service, jwtService *JWT) (userID uuid.UUID, accessToken string, ok bool) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie == nil || cookie.Value == "" {
		return uuid.Nil, "", false
	}

	claims, err := jwtService.ValidateToken(cookie.Value)
	if err != nil || claims.Type != "refresh" {
		return uuid.Nil, "", false
	}

	storedToken, err := db.GetRefreshToken(cookie.Value)
	if err != nil || storedToken.Revoked || time.Now().After(storedToken.ExpiresAt) {
		return uuid.Nil, "", false
	}

	accessToken, _, err = jwtService.GenerateAccessToken(claims.UserID)
	if err != nil {
		return uuid.Nil, "", false
	}

	SetAuthCookie(w, r, "access_token", accessToken, int(AccessTokenDuration.Seconds()))
	return claims.UserID, accessToken, true
}

// SetAuthCookie writes an HttpOnly auth cookie. The Secure flag follows the
// actual request scheme: TLS (or the X-Forwarded-Proto header from a reverse
// proxy) marks the cookie Secure, while plain HTTP keeps it usable on any
// origin (localhost, LAN IP, etc.).
func SetAuthCookie(w http.ResponseWriter, r *http.Request, tokenName, token string, maxAge int) {
	secure := r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
	http.SetCookie(w, &http.Cookie{
		Name:     tokenName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

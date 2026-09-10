package auth

import (
	"net/http"
	"os"
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

	SetAuthCookie(w, "access_token", accessToken, int(AccessTokenDuration.Seconds()))
	return claims.UserID, accessToken, true
}

// SetAuthCookie writes an HttpOnly auth cookie.
func SetAuthCookie(w http.ResponseWriter, tokenName, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") != "local",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

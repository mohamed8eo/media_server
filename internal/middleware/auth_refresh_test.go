package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"mediaserver/internal/auth"
	"mediaserver/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestUIAuthMiddlewareSilentRefresh(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-for-refresh")
	os.Setenv("BLUEPRINT_DB_URL", "file:uiauthrefresh?mode=memory&cache=shared")
	os.Setenv("APP_ENV", "local")

	// Reset singleton by using unique memory db - database.New may reuse previous instance
	db := database.New()
	jwtService := auth.NewJWT(os.Getenv("JWT_SECRET"))

	userID, err := db.CreateUser("refresh-ui@example.com", "hash")
	if err != nil {
		// maybe singleton from other test - try get by email path differently
		t.Fatalf("CreateUser: %v", err)
	}

	refreshToken, refreshExp, err := jwtService.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.StoreRefreshToken(userID, refreshToken, refreshExp); err != nil {
		t.Fatal(err)
	}

	expiredAccess := makeExpiredAccessToken(t, jwtService, userID)

	handler := UIAuthMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(UserIDKey).(uuid.UUID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(uid.String()))
	}))

	t.Run("only_refresh_cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d location=%s body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
		}
		if len(rr.Result().Cookies()) == 0 {
			t.Fatal("expected new access_token cookie")
		}
	})

	t.Run("expired_access_plus_refresh", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: expiredAccess})
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d location=%s body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
		}
	})

	t.Run("no_cookies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("want 303, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "/sign-in" {
			t.Fatalf("want /sign-in, got %s", rr.Header().Get("Location"))
		}
	})
}

func makeExpiredAccessToken(t *testing.T, j *auth.JWT, userID uuid.UUID) string {
	t.Helper()
	now := time.Now().Add(-time.Hour)
	claims := auth.Claims{
		UserID: userID,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	// ExpiresAt already in the past relative to now... wait now is -1h, expires at -1h+1m still past
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.Secret)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

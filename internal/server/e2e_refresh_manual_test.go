package server_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"mediaserver/internal/auth"
	"mediaserver/internal/database"
	"mediaserver/internal/server"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestE2ESilentRefreshAfterAccessExpiry(t *testing.T) {
	os.Setenv("JWT_SECRET", "e2e-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:e2e_refresh_manual?mode=memory&cache=shared")
	database.Reset()

	s := server.NewServer()
	ts := httptest.NewServer(s.Handler)
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	form := url.Values{}
	form.Set("email", "e2e-refresh@example.com")
	form.Set("password", "SecurePass1!")
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}

	u, _ := url.Parse(ts.URL)
	var refreshVal string
	for _, c := range jar.Cookies(u) {
		t.Logf("cookie %s len=%d", c.Name, len(c.Value))
		if c.Name == "refresh_token" {
			refreshVal = c.Value
		}
	}
	if refreshVal == "" {
		t.Fatal("missing refresh_token cookie after register")
	}

	jwtService := auth.NewJWT("e2e-secret")
	claims, err := jwtService.ValidateToken(refreshVal)
	if err != nil {
		t.Fatal(err)
	}
	expired := mustExpiredAccess(t, jwtService, claims.UserID)

	t.Run("expired_access_plus_refresh", func(t *testing.T) {
		jar2, _ := cookiejar.New(nil)
		c2 := &http.Client{Jar: jar2, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		jar2.SetCookies(u, []*http.Cookie{
			{Name: "access_token", Value: expired, Path: "/"},
			{Name: "refresh_token", Value: refreshVal, Path: "/"},
		})
		resp, err := c2.Get(ts.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d location=%s body=%s set-cookie=%v", resp.StatusCode, resp.Header.Get("Location"), b, resp.Header.Values("Set-Cookie"))
		}
	})

	t.Run("only_refresh", func(t *testing.T) {
		jar2, _ := cookiejar.New(nil)
		c2 := &http.Client{Jar: jar2, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		jar2.SetCookies(u, []*http.Cookie{
			{Name: "refresh_token", Value: refreshVal, Path: "/"},
		})
		resp, err := c2.Get(ts.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d location=%s body=%s set-cookie=%v", resp.StatusCode, resp.Header.Get("Location"), b, resp.Header.Values("Set-Cookie"))
		}
	})

	stored, err := database.New().GetRefreshToken(refreshVal)
	if err != nil {
		t.Fatalf("GetRefreshToken: %v", err)
	}
	if stored.Revoked || time.Now().After(stored.ExpiresAt) {
		t.Fatalf("stored token invalid: %+v", stored)
	}
}

func mustExpiredAccess(t *testing.T, j *auth.JWT, userID uuid.UUID) string {
	t.Helper()
	now := time.Now().Add(-2 * time.Hour)
	claims := auth.Claims{
		UserID: userID,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(j.Secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

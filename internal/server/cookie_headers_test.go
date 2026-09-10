package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"mediaserver/internal/database"
	"mediaserver/internal/server"
)

func TestLoginSetsBothAuthCookies(t *testing.T) {
	os.Setenv("JWT_SECRET", "cookie-header-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:cookie_headers_db?mode=memory&cache=shared")
	database.Reset()

	s := server.NewServer()
	ts := httptest.NewServer(s.Handler)
	defer ts.Close()

	form := url.Values{}
	form.Set("email", "cookies2@example.com")
	form.Set("password", "SecurePass1!")
	resp, err := http.Post(ts.URL+"/api/auth/register", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	t.Logf("status=%d raw set-cookie=%v", resp.StatusCode, resp.Header.Values("Set-Cookie"))
	var hasAccess, hasRefresh bool
	for _, c := range cookies {
		t.Logf("cookie name=%s maxage=%d len=%d", c.Name, c.MaxAge, len(c.Value))
		if c.Name == "access_token" {
			hasAccess = true
		}
		if c.Name == "refresh_token" {
			hasRefresh = true
			if c.MaxAge > 0 && c.MaxAge < 86400 {
				t.Errorf("refresh maxage too small: %d", c.MaxAge)
			}
		}
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("register failed: %d", resp.StatusCode)
	}
	if !hasAccess || !hasRefresh {
		t.Fatalf("missing cookies access=%v refresh=%v", hasAccess, hasRefresh)
	}
}

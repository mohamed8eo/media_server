package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"mediaserver/internal/database"
	"mediaserver/internal/middleware"

	"github.com/google/uuid"
)

func TestHandler(t *testing.T) {
	s := &Server{}
	server := httptest.NewServer(http.HandlerFunc(s.HelloWorldHandler))
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request to server. Err: %v", err)
	}
	defer resp.Body.Close()
	// Assertions
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}
	expected := "{\"message\":\"Hello World\"}"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body. Err: %v", err)
	}
	if expected != string(body) {
		t.Errorf("expected response body to be %v; got %v", expected, string(body))
	}
}

func TestAuthMiddlewareUnauthorized(t *testing.T) {
	os.Setenv("JWT_SECRET", "routes-test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:routes_auth_mw?mode=memory&cache=shared")
	database.Reset()

	s := &Server{db: database.New()}
	handler := s.RegisterRoutes()
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/me")
	if err != nil {
		t.Fatalf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized; got %v", resp.StatusCode)
	}
}

func TestWatchHandlerRejectsMalformedID(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/watch/not-a-uuid", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	res := httptest.NewRecorder()

	s.WatchHandler(res, req.WithContext(ctx))

	if res.Code != http.StatusNotFound {
		t.Errorf("expected malformed watch ID to return 404, got %d", res.Code)
	}
	if body := res.Body.String(); !strings.Contains(body, "Video unavailable") {
		t.Errorf("expected unavailable page, got %q", body)
	}
}

func TestWatchRouteRequiresAuthentication(t *testing.T) {
	s := &Server{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/watch/8cf9f1bc-4db1-4b08-b4c7-9a9d772b8819", nil)

	s.RegisterRoutes().ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Errorf("expected unauthenticated watch route to redirect, got %d", res.Code)
	}
	if location := res.Header().Get("Location"); location != "/sign-in" {
		t.Errorf("expected sign-in redirect, got %q", location)
	}
}

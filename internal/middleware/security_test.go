package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d", rr.Code)
	}

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("want X-Content-Type-Options nosniff, got %s", got)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("want X-Frame-Options SAMEORIGIN, got %s", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("want Referrer-Policy strict-origin-when-cross-origin, got %s", got)
	}
	if got := rr.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}
}

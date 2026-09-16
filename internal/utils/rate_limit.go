package utils

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*Client
	r       rate.Limit
	burst   int
}

func NewRateLimit(r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*Client),
		r:       r,
		burst:   burst,
	}
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		rl.mu.Lock()

		client, exists := rl.clients[ip]
		if !exists {
			client = &Client{
				limiter: rate.NewLimiter(rl.r, rl.burst),
			}

			rl.clients[ip] = client
		}

		client.lastSeen = time.Now()
		allowed := client.limiter.Allow()

		rl.mu.Unlock()

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)

			w.Write([]byte(`{
				"error": "too many requests, please try again later"
			}`))

			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")

		return strings.TrimSpace(parts[0])
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, client := range rl.clients {
			if time.Since(client.lastSeen) > 10*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

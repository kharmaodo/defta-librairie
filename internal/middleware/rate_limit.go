package middleware

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const maxRateLimitClients = 10000

type rateBucket struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]rateBucket
	limit   int
	window  time.Duration
	now     func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) (*RateLimiter, error) {
	if limit < 1 || window < time.Second || window > time.Hour {
		return nil, errors.New("rate limit must be positive and window between 1 second and 1 hour")
	}
	return &RateLimiter{clients: make(map[string]rateBucket), limit: limit, window: window, now: time.Now}, nil
}

func (l *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := l.now().UTC()
		key := clientIP(r)
		allowed, retryAfter := l.allow(key, now)
		if !allowed {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate_limited", "message": "Too many requests",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, exists := l.clients[key]
	if !exists && len(l.clients) >= maxRateLimitClients {
		for client, candidate := range l.clients {
			if !now.Before(candidate.resetAt) {
				delete(l.clients, client)
			}
		}
		if len(l.clients) >= maxRateLimitClients {
			return false, l.window
		}
	}
	if !exists || !now.Before(bucket.resetAt) {
		l.clients[key] = rateBucket{count: 1, resetAt: now.Add(l.window)}
		return true, 0
	}
	if bucket.count >= l.limit {
		return false, bucket.resetAt.Sub(now)
	}
	bucket.count++
	l.clients[key] = bucket
	return true, 0
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

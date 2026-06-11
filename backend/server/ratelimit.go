package server

// ratelimit.go — token-bucket rate limiter implemented with only stdlib +
// existing dependencies (no new external packages required).
//
// The limiter uses a per-key token bucket stored in a sync.Map.  Each bucket
// refills at a fixed rate and holds a maximum burst of tokens.  This is safe
// for concurrent use and has O(1) amortised cost per request.

import (
	"net/http"
	"sync"
	"time"
)

// bucket is a single token-bucket entry.
type bucket struct {
	mu       sync.Mutex
	tokens   float64
	lastTime time.Time
}

// allow returns true if a token could be consumed from the bucket.
func (b *bucket) allow(rate float64, burst float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now

	// Refill tokens proportional to elapsed time.
	b.tokens += elapsed * rate
	if b.tokens > burst {
		b.tokens = burst
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter holds per-key buckets.
type RateLimiter struct {
	mu      sync.Mutex
	buckets sync.Map
	rate    float64 // tokens per second
	burst   float64 // maximum burst
}

// NewRateLimiter creates a limiter that allows `rate` requests per second
// with an initial burst of `burst`.
func NewRateLimiter(rate, burst float64) *RateLimiter {
	rl := &RateLimiter{rate: rate, burst: burst}
	// Periodically purge stale buckets to avoid unbounded growth.
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getBucket(key string) *bucket {
	v, _ := rl.buckets.LoadOrStore(key, &bucket{
		tokens:   rl.burst,
		lastTime: time.Now(),
	})
	return v.(*bucket)
}

// Allow returns true when the request identified by key is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	return rl.getBucket(key).allow(rl.rate, rl.burst)
}

// cleanupLoop removes buckets that have been idle for more than 10 minutes.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.buckets.Range(func(k, v interface{}) bool {
			b := v.(*bucket)
			b.mu.Lock()
			idle := b.lastTime.Before(cutoff)
			b.mu.Unlock()
			if idle {
				rl.buckets.Delete(k)
			}
			return true
		})
	}
}

// --------------------------------------------------------------------------
// HTTP middleware helpers
// --------------------------------------------------------------------------

// realIP extracts the most likely real client IP from common proxy headers,
// falling back to RemoteAddr.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For may be "client, proxy1, proxy2" — take the first.
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// authRateLimiter is the limiter used for registration and login endpoints.
// 5 requests per second, burst of 10.
var authRateLimiter = NewRateLimiter(5, 10)

// rateLimitAuth is middleware that rate-limits authentication endpoints
// (register + login) by client IP.
func rateLimitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !authRateLimiter.Allow(ip) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// wsRateLimiter limits WebSocket upgrade attempts per IP.
// 10 connections per second, burst of 20.
var wsRateLimiter = NewRateLimiter(10, 20)

// rateLimitWS is middleware that rate-limits WebSocket upgrade requests by IP.
func rateLimitWS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !wsRateLimiter.Allow(ip) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// maxBodyBytes wraps a handler so that request bodies are capped at n bytes.
// Requests with bodies larger than the limit receive 413 Request Entity Too Large.
func maxBodyBytes(n int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

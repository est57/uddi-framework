package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/response"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func LimitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	buckets   map[string]rateBucket
	now       func() time.Time
	lastSweep time.Time
}

type rateBucket struct {
	count     int
	resetAt   time.Time
	updatedAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]rateBucket),
		now:     time.Now,
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Retry-After", secondsString(l.window))
			response.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) bool {
	if l.limit <= 0 || l.window <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= l.window {
		for bucketKey, bucket := range l.buckets {
			if now.Sub(bucket.updatedAt) >= l.window {
				delete(l.buckets, bucketKey)
			}
		}
		l.lastSweep = now
	}

	bucket := l.buckets[key]
	if bucket.resetAt.IsZero() || !now.Before(bucket.resetAt) {
		l.buckets[key] = rateBucket{
			count:     1,
			resetAt:   now.Add(l.window),
			updatedAt: now,
		}
		return true
	}

	if bucket.count >= l.limit {
		bucket.updatedAt = now
		l.buckets[key] = bucket
		return false
	}

	bucket.count++
	bucket.updatedAt = now
	l.buckets[key] = bucket
	return true
}

func clientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func secondsString(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

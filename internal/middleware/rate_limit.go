package middleware

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	maxIPs = 100
)

// rateEntry tracks attempts for a single IP.
type rateEntry struct {
	attempts    int
	windowStart time.Time
	lockedUntil time.Time
}

// pathMatcher returns true if the request should be rate limited.
type pathMatcher func(r *http.Request) bool

// rateLimiter holds the in-memory rate limiting state.
type rateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rateEntry
	maxAttempts int
	window      time.Duration
	lockout     time.Duration
	reqCount   int
	matches     pathMatcher
}

// RateLimit returns middleware that limits login attempts per IP.
// maxAttempts is the number of allowed attempts within window.
// After exceeding, the IP is locked out for lockout duration.
// Only applies to POST /api/v1/auth/login.
func RateLimit(maxAttempts int, window, lockout time.Duration) func(http.Handler) http.Handler {
	return endpointRateLimit(maxAttempts, window, lockout, func(r *http.Request) bool {
		return r.URL.Path == "/api/v1/auth/login" && r.Method == "POST"
	})
}

// EndpointRateLimit returns middleware that rate-limits requests per IP
// for the given path and method combination.
func EndpointRateLimit(path, method string, maxAttempts int, window, lockout time.Duration) func(http.Handler) http.Handler {
	return endpointRateLimit(maxAttempts, window, lockout, func(r *http.Request) bool {
		return r.URL.Path == path && r.Method == method
	})
}

// endpointRateLimit creates a rate limiter with a custom matcher.
func endpointRateLimit(maxAttempts int, window, lockout time.Duration, matcher pathMatcher) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		entries:     make(map[string]*rateEntry),
		maxAttempts: maxAttempts,
		window:      window,
		lockout:     lockout,
		matches:     matcher,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.matches(r) {
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r.RemoteAddr)

			rl.mu.Lock()
			defer rl.mu.Unlock()

			now := time.Now()

			rl.reqCount++
			if rl.reqCount%100 == 0 {
				rl.prune(now)
			}

			entry, exists := rl.entries[ip]
			if !exists {
				if len(rl.entries) >= maxIPs {
					rl.evictOldest(now)
				}
				entry = &rateEntry{
					windowStart: now,
				}
				rl.entries[ip] = entry
			}

			// Check if currently locked out.
			if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
				retryAfter := int(entry.lockedUntil.Sub(now).Seconds()) + 1
				rl.writeLimited(w, retryAfter)
				return
			}

			// Reset window if expired.
			if now.Sub(entry.windowStart) > rl.window {
				entry.attempts = 0
				entry.windowStart = now
			}

			// Check attempt count.
			if entry.attempts >= rl.maxAttempts {
				entry.lockedUntil = now.Add(rl.lockout)
				entry.attempts = 0
				entry.windowStart = now
				retryAfter := int(rl.lockout.Seconds())
				rl.writeLimited(w, retryAfter)
				return
			}

			entry.attempts++
			next.ServeHTTP(w, r)
		})
	}
}

// extractIP strips the port from a RemoteAddr string.
func extractIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

// prune removes expired entries from the map.
func (rl *rateLimiter) prune(now time.Time) {
	for ip, entry := range rl.entries {
		expired := true
		if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
			expired = false
		}
		if now.Sub(entry.windowStart) <= rl.window {
			expired = false
		}
		if expired {
			delete(rl.entries, ip)
		}
	}
}

// evictOldest removes the entry with the earliest windowStart.
func (rl *rateLimiter) evictOldest(now time.Time) {
	var oldestIP string
	var oldestTime time.Time
	first := true
	for ip, entry := range rl.entries {
		// Skip locked entries — they must remain until expiry.
		if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
			continue
		}
		if first || entry.windowStart.Before(oldestTime) {
			oldestIP = ip
			oldestTime = entry.windowStart
			first = false
		}
	}
	if oldestIP != "" {
		delete(rl.entries, oldestIP)
	}
}

// writeLimited sends a 429 Too Many Requests response.
func (rl *rateLimiter) writeLimited(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"success":false,"message":"too many requests"}`))
}

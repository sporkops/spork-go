package spork

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimit is a snapshot of the server-reported rate-limit state from the
// most recent response. Fields are zero when the server did not return the
// corresponding header (e.g., on errors before the rate-limiter ran).
type RateLimit struct {
	// Limit is the total number of requests allowed per window.
	Limit int
	// Remaining is the number of requests the caller has left in the window.
	Remaining int
	// Reset is the wall-clock time at which the window resets. If the server
	// returned a Unix timestamp it is interpreted in UTC.
	Reset time.Time
	// ObservedAt is the local time at which this snapshot was taken.
	ObservedAt time.Time
}

// parseRateLimit extracts a RateLimit from response headers. Returns the zero
// value when no rate-limit headers are present.
func parseRateLimit(h http.Header) RateLimit {
	if h == nil {
		return RateLimit{}
	}
	lim := RateLimit{ObservedAt: time.Now()}
	if v := h.Get("X-Ratelimit-Limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lim.Limit = n
		}
	}
	if v := h.Get("X-Ratelimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lim.Remaining = n
		}
	}
	if v := h.Get("X-Ratelimit-Reset"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			lim.Reset = time.Unix(ts, 0).UTC()
		}
	}
	return lim
}

// rateLimitStore is a thread-safe cell holding the most recent RateLimit.
type rateLimitStore struct {
	mu  sync.RWMutex
	rl  RateLimit
	set bool
}

func (s *rateLimitStore) store(rl RateLimit) {
	if rl == (RateLimit{}) {
		return
	}
	s.mu.Lock()
	s.rl = rl
	s.set = true
	s.mu.Unlock()
}

func (s *rateLimitStore) load() (RateLimit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rl, s.set
}

package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/TheZupZup/Rivo/services/api/internal/httpapi/handlers"
)

// idleBucketTTL is how long a client's bucket survives without traffic. Buckets are
// only swept when the limiter is used, so an idle server keeps no timers.
const idleBucketTTL = 10 * time.Minute

// sweepInterval bounds how often the limiter walks its whole map.
const sweepInterval = time.Minute

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// rateLimiter is a per-client token bucket.
//
// It is keyed by remote address rather than by authenticated user on purpose: it
// runs before authentication so that unauthenticated traffic cannot make the API
// hit the database once per request. Clients sharing an address share a bucket,
// which is an accepted trade for v0.1.
type rateLimiter struct {
	mutex           sync.Mutex
	buckets         map[string]*bucket
	burst           float64
	refillPerSecond float64
	lastSweep       time.Time
	now             func() time.Time
}

func newRateLimiter(burst, refillPerMinute int) *rateLimiter {
	return &rateLimiter{
		buckets:         make(map[string]*bucket),
		burst:           float64(burst),
		refillPerSecond: float64(refillPerMinute) / 60,
		now:             time.Now,
	}
}

func (limiter *rateLimiter) allow(key string) bool {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	limiter.sweep(now)

	existing, found := limiter.buckets[key]
	if !found {
		limiter.buckets[key] = &bucket{tokens: limiter.burst - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(existing.lastSeen).Seconds()
	existing.tokens = min(limiter.burst, existing.tokens+elapsed*limiter.refillPerSecond)
	existing.lastSeen = now

	if existing.tokens < 1 {
		return false
	}

	existing.tokens--
	return true
}

func (limiter *rateLimiter) sweep(now time.Time) {
	if now.Sub(limiter.lastSweep) < sweepInterval {
		return
	}
	limiter.lastSweep = now

	for key, tracked := range limiter.buckets {
		if now.Sub(tracked.lastSeen) > idleBucketTTL {
			delete(limiter.buckets, key)
		}
	}
}

func withRateLimit(limiter *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !limiter.allow(clientKey(request)) {
			w.Header().Set("Retry-After", "60")
			handlers.WriteError(w, http.StatusTooManyRequests, "too many requests, slow down")
			return
		}

		next.ServeHTTP(w, request)
	})
}

// clientKey deliberately ignores X-Forwarded-For: any client can set it, so
// trusting it would let a single caller mint unlimited buckets. A deployment behind
// a proxy must terminate that header at the proxy and pass a trusted value instead.
func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}

	return host
}

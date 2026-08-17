package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenRefuses(t *testing.T) {
	limiter := newRateLimiter(3, 60)
	limiter.now = func() time.Time { return time.Unix(0, 0) }

	for attempt := 1; attempt <= 3; attempt++ {
		if !limiter.allow("client") {
			t.Fatalf("expected attempt %d to be inside the burst", attempt)
		}
	}

	if limiter.allow("client") {
		t.Fatal("expected the fourth attempt to be refused")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter := newRateLimiter(1, 60) // one token per second
	now := time.Unix(0, 0)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("client") {
		t.Fatal("expected the first request to pass")
	}
	if limiter.allow("client") {
		t.Fatal("expected the second request to be refused immediately")
	}

	now = now.Add(2 * time.Second)
	if !limiter.allow("client") {
		t.Fatal("expected a refilled token after two seconds")
	}
}

func TestRateLimiterKeepsClientsIndependent(t *testing.T) {
	limiter := newRateLimiter(1, 60)
	limiter.now = func() time.Time { return time.Unix(0, 0) }

	if !limiter.allow("first") {
		t.Fatal("expected the first client to pass")
	}
	if !limiter.allow("second") {
		t.Fatal("expected a second client to have its own bucket")
	}
	if limiter.allow("first") {
		t.Fatal("expected the first client to still be limited")
	}
}

func TestRateLimiterForgetsIdleClients(t *testing.T) {
	limiter := newRateLimiter(1, 60)
	now := time.Unix(0, 0)
	limiter.now = func() time.Time { return now }

	limiter.allow("client")
	if len(limiter.buckets) != 1 {
		t.Fatalf("expected one tracked bucket, got %d", len(limiter.buckets))
	}

	now = now.Add(idleBucketTTL + sweepInterval)
	limiter.allow("other")

	if _, tracked := limiter.buckets["client"]; tracked {
		t.Fatal("expected the idle bucket to be swept so the map cannot grow without bound")
	}
}

func TestWithRateLimitAnswers429(t *testing.T) {
	limiter := newRateLimiter(1, 60)
	limiter.now = func() time.Time { return time.Unix(0, 0) }

	handler := withRateLimit(limiter, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/reports", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected the first request through, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/reports", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("expected a Retry-After header so a client knows when to come back")
	}
}

func TestClientKeyIgnoresForwardedForHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/reports", nil)
	request.RemoteAddr = "203.0.113.7:54321"
	request.Header.Set("X-Forwarded-For", "198.51.100.1")

	// Honouring the header would let one caller mint a fresh bucket per request.
	if key := clientKey(request); key != "203.0.113.7" {
		t.Fatalf("expected the peer address, got %q", key)
	}
}

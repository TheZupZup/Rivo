package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheZupZup/Rivo/services/api/internal/config"
	"github.com/TheZupZup/Rivo/services/api/internal/platform"
)

func testRouter() http.Handler {
	return NewRouter(Dependencies{
		Authenticator: testAuthenticator(),
		// Unauthenticated requests never reach the service, so it needs no stores.
		ReportService:  platform.ReportService{},
		AllowedOrigin:  "http://localhost:3000",
		MaxUploadBytes: 1024,
		RateLimit:      config.RateLimit{Burst: 20, RefillPerMinute: 60},
	})
}

func TestHealthIsPublic(t *testing.T) {
	response := httptest.NewRecorder()
	testRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected a readiness probe to work without a credential, got %d", response.Code)
	}
}

func TestWriteRoutesRequireAToken(t *testing.T) {
	router := testRouter()

	for _, target := range []string{"/api/videos", "/api/reports"} {
		t.Run(target, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, target, strings.NewReader("{}")))

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected %s to be closed to anonymous callers, got %d", target, response.Code)
			}
		})
	}
}

func TestPreflightAdvertisesTheAuthorizationHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/reports", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()

	testRouter().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for a preflight from the configured origin, got %d", response.Code)
	}
	if allowed := response.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(allowed, "Authorization") {
		t.Fatalf("the browser cannot send a bearer token unless it is allowed, got %q", allowed)
	}
}

func TestPreflightFromAnotherOriginIsNotApproved(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/reports", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()

	testRouter().ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS approval for an unconfigured origin")
	}
}

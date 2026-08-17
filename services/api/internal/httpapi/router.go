package httpapi

import (
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	"github.com/TheZupZup/Rivo/services/api/internal/config"
	"github.com/TheZupZup/Rivo/services/api/internal/httpapi/handlers"
	"github.com/TheZupZup/Rivo/services/api/internal/platform"
	platformvideo "github.com/TheZupZup/Rivo/services/api/internal/platform/video"
	"github.com/TheZupZup/Rivo/services/api/internal/storage"
)

type Dependencies struct {
	VideoStore     storage.VideoStore
	Authenticator  auth.Authenticator
	ReportService  platform.ReportService
	AllowedOrigin  string
	MaxUploadBytes int64
	RateLimit      config.RateLimit
}

func NewRouter(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()

	// Health is public: a readiness probe must not need a credential.
	mux.HandleFunc("GET /healthz", handlers.Health)

	limiter := newRateLimiter(dependencies.RateLimit.Burst, dependencies.RateLimit.RefillPerMinute)

	// Rate limiting runs before authentication so that unauthenticated traffic
	// cannot force one database lookup per request.
	protect := func(handler http.HandlerFunc) http.Handler {
		return withRateLimit(limiter, requireIdentity(dependencies.Authenticator, handler))
	}

	videoUploadHandler := handlers.NewVideoUploadHandler(
		platformvideo.NewUploadService(dependencies.VideoStore),
		dependencies.MaxUploadBytes,
	)
	mux.Handle("POST /api/videos", protect(videoUploadHandler.Upload))

	reportHandler := handlers.NewReportHandler(dependencies.ReportService)
	mux.Handle("POST /api/reports", protect(reportHandler.Submit))

	return withAllowedOrigin(dependencies.AllowedOrigin, mux)
}

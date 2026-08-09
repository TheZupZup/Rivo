package httpapi

import (
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/httpapi/handlers"
	platformvideo "github.com/TheZupZup/Rivo/services/api/internal/platform/video"
	"github.com/TheZupZup/Rivo/services/api/internal/storage"
)

type Dependencies struct {
	VideoStore     storage.VideoStore
	AllowedOrigin  string
	MaxUploadBytes int64
}

func NewRouter(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Health)

	videoUploadHandler := handlers.NewVideoUploadHandler(
		platformvideo.NewUploadService(dependencies.VideoStore),
		dependencies.MaxUploadBytes,
	)
	mux.HandleFunc("POST /api/videos", videoUploadHandler.Upload)

	return withAllowedOrigin(dependencies.AllowedOrigin, mux)
}

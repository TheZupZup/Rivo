package httpapi

import (
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/httpapi/handlers"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Health)
	return mux
}

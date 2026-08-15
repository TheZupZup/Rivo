package main

import (
	"log"
	"net/http"
	"time"

	"github.com/TheZupZup/Rivo/services/api/internal/config"
	"github.com/TheZupZup/Rivo/services/api/internal/httpapi"
	"github.com/TheZupZup/Rivo/services/api/internal/storage/local"
)

func main() {
	cfg := config.Load()

	videoStore, err := local.New(cfg.VideoStoragePath)
	if err != nil {
		log.Fatalf("initialize video storage: %v", err)
	}

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			VideoStore:    videoStore,
			AllowedOrigin: cfg.WebOrigin,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", cfg.HTTPAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

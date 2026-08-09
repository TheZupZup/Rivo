package main

import (
	"log"
	"net/http"
	"time"

	"github.com/TheZupZup/Rivo/services/api/internal/config"
	"github.com/TheZupZup/Rivo/services/api/internal/httpapi"
)

func main() {
	cfg := config.Load()

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", cfg.HTTPAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

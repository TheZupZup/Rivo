package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	"github.com/TheZupZup/Rivo/services/api/internal/config"
	"github.com/TheZupZup/Rivo/services/api/internal/httpapi"
	"github.com/TheZupZup/Rivo/services/api/internal/platform"
	"github.com/TheZupZup/Rivo/services/api/internal/storage/local"
	"github.com/TheZupZup/Rivo/services/api/internal/storage/postgres"
)

// shutdownGrace is long enough for a large upload in flight to finish writing.
const shutdownGrace = 30 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	videoStore, err := local.New(cfg.VideoStoragePath)
	if err != nil {
		return err
	}

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router := httpapi.NewRouter(httpapi.Dependencies{
		VideoStore:    videoStore,
		Authenticator: auth.NewAuthenticator(postgres.NewTokenStore(pool)),
		ReportService: platform.NewReportService(
			postgres.NewRevisionStore(pool),
			postgres.NewReportStore(pool),
			platform.NewReportEligibilityService(postgres.NewRuleStore(pool)),
		),
		AllowedOrigin:  cfg.WebOrigin,
		MaxUploadBytes: cfg.MaxUploadBytes,
		RateLimit:      cfg.UploadRateLimit,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: router,
		// No WriteTimeout: it would cap how long an upload may take, and the body
		// size limit already bounds the work a single request can cause.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("API listening on %s", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		log.Print("shutdown requested, draining connections")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return err
	}

	return <-serverErrors
}

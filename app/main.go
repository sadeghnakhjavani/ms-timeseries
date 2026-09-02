package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iranrates/marketick/internal/api"
	"github.com/iranrates/marketick/internal/config"
	chstore "github.com/iranrates/marketick/internal/clickhouse"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	store, err := chstore.NewClient(cfg)
	if err != nil {
		log.Fatalf("clickhouse connection failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.StartupChecks(ctx); err != nil {
		log.Fatalf("startup checks failed: %v", err)
	}

	auth := api.NewAuthenticator(cfg.APIKey)
	pagination := api.NewPaginationSettings(cfg.PageDefaultLimit, cfg.PageMaxLimit)
	handler := api.NewRouter(store, auth, pagination)

	addr := fmt.Sprintf(":%d", cfg.AppPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("marketick listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

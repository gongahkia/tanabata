package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpapi "github.com/gongahkia/tanabata/api/internal/api"
	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/observability"
)

func main() {
	catalogPath := envOrDefault("CATALOG_PATH", filepath.Join("data", "catalog.sqlite"))
	port := envOrDefault("PORT", "8080")

	store, err := catalog.Open(catalogPath)
	if err != nil {
		log.Fatalf("open catalog: %v", err)
	}
	defer store.Close()

	telemetry, err := observability.New("tanabata-api")
	if err != nil {
		log.Fatalf("initialize telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(shutdownCtx)
	}()

	server := httpapi.NewServer(store, telemetry)
	router := server.Router()

	go func() {
		if err := router.Run(":" + port); err != nil {
			log.Fatalf("start server: %v", err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

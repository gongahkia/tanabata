package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpapi "github.com/gongahkia/tanabata/api/internal/api"
	"github.com/gongahkia/tanabata/api/internal/buildinfo"
	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/observability"
)

func main() {
	versionFlag := flag.Bool("version", false, "print build metadata and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(buildinfo.Summary())
		return
	}

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

	server, err := httpapi.NewServer(store, telemetry)
	if err != nil {
		log.Fatalf("initialize API server: %v", err)
	}
	router := server.Router()
	httpServer := newHTTPServer(port, router)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("start server: %v", err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown server: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

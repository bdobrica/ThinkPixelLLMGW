package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm_gateway/internal/config"
	"llm_gateway/internal/httpapi"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create router with all dependencies
	mux, deps, err := httpapi.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to build router: %v", err)
	}

	// Create HTTP server
	addr := ":" + cfg.HTTPPort
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("LLM Gateway listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Shutdown request logger to flush remaining buffered logs
	if deps.RequestLogger != nil {
		deps.RequestLogger.Shutdown()
	}

	// Shutdown S3 logging sink to flush remaining logs to S3
	if deps.Logger != nil {
		if err := deps.Logger.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown logging sink: %v", err)
		}
	}

	// Shutdown billing service to sync final data
	if billingService, ok := deps.Billing.(interface{ Shutdown(context.Context) error }); ok {
		_ = billingService.Shutdown(ctx)
	}

	// Close provider registry (which closes all providers)
	if registry, ok := deps.Providers.(interface{ Close() error }); ok {
		_ = registry.Close()
	}

	log.Println("Server exited")
}

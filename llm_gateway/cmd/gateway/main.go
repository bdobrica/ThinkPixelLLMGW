package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm_gateway/internal/config"
	"llm_gateway/internal/httpapi"
	"llm_gateway/internal/utils"
)

var gatewayLogger = utils.NewLogger("gateway-main", utils.Info)

func main() {
	const (
		inFlightGracePeriod = 5 * time.Second
		finalFlushTTL       = 30 * time.Second
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		gatewayLogger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create router with all dependencies
	mux, deps, err := httpapi.NewRouter(cfg)
	if err != nil {
		gatewayLogger.Error("failed to build router", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	addr := ":" + cfg.HTTPPort
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPServer.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPServer.ReadTimeout,
		WriteTimeout:      cfg.HTTPServer.WriteTimeout,
		IdleTimeout:       cfg.HTTPServer.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		gatewayLogger.Info("LLM Gateway listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gatewayLogger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	gatewayLogger.Info("shutting down server")
	gatewayLogger.Info("allowing in-flight requests to complete", "grace_period", inFlightGracePeriod)
	time.Sleep(inFlightGracePeriod)

	// Stop keep-alive reuse while shutting down.
	server.SetKeepAlivesEnabled(false)

	// Gracefully stop accepting new requests and wait for active handlers.
	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
	defer serverShutdownCancel()

	if err := server.Shutdown(serverShutdownCtx); err != nil {
		gatewayLogger.Error("server forced to shutdown", "error", err)
		if closeErr := server.Close(); closeErr != nil {
			gatewayLogger.Error("failed to close active server connections", "error", closeErr)
		}
	}

	// Use a separate context so flush operations are not canceled by server shutdown timeout.
	flushCtx, flushCancel := context.WithTimeout(context.Background(), finalFlushTTL)
	defer flushCancel()

	// Shutdown request logger to flush remaining buffered logs
	if deps.RequestLogger != nil {
		deps.RequestLogger.Shutdown()
	}

	// Shutdown S3 logging sink to flush remaining logs to S3
	if deps.Logger != nil {
		if err := deps.Logger.Shutdown(flushCtx); err != nil {
			gatewayLogger.Error("failed to shutdown logging sink", "error", err)
		}
	}

	// Shutdown billing service to sync final data
	if billingService, ok := deps.Billing.(interface{ Shutdown(context.Context) error }); ok {
		if err := billingService.Shutdown(flushCtx); err != nil {
			gatewayLogger.Error("failed to shutdown billing service", "error", err)
		}
	}

	// Close provider registry (which closes all providers)
	if registry, ok := deps.Providers.(interface{ Close() error }); ok {
		_ = registry.Close()
	}

	gatewayLogger.Info("server exited")
}

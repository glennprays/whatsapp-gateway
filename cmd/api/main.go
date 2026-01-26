package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/infrastructure"
	"github.com/google/uuid"
)

func main() {
	// Generate trace ID for startup logs
	traceID := uuid.New().String()

	// Create temporary logger for startup (before app initialization)
	tempCfg := log.Config{
		Service:      "whatsapp-gateway",
		Env:          "development",
		Level:        log.InfoLevel,
		Output:       log.OutputStdout,
		EnableCaller: false,
	}
	tempLogger, err := log.New(tempCfg)
	if err != nil {
		panic("Failed to create temporary logger: " + err.Error())
	}

	tempLogger.Info(traceID, "Starting WhatsApp Gateway", nil)
	tempLogger.Info(traceID, "Loading configuration and initializing dependencies", nil)

	cfg, err := config.Load()
	if err != nil {
		tempLogger.Fatal(traceID, "Failed to load configuration", []log.Field{log.Error(err)})
	}
	docs.NewSwagger(cfg)

	tempLogger.Info(traceID, "Initializing application with Wire", nil)
	app, cleanup, err := infrastructure.InitializeApp()
	if err != nil {
		tempLogger.Fatal(traceID, "Failed to initialize application", []log.Field{log.Error(err)})
	}
	defer cleanup()

	app.Logger.Info(traceID, "Starting server", []log.Field{log.String("port", cfg.Port)})

	go func() {
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Logger.Fatal(traceID, "HTTP server ListenAndServe failed", []log.Field{log.Error(err)})
		}
		app.Logger.Info(traceID, "HTTP server stopped", nil)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.Logger.Info(traceID, "Shutdown signal received, initiating graceful shutdown", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Server.Shutdown(ctx); err != nil {
		app.Logger.Fatal(traceID, "Server forced to shutdown", []log.Field{log.Error(err)})
	}

	app.Logger.Info(traceID, "Server exiting gracefully", nil)
}

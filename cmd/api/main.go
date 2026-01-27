package main

import (
	"context"
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
	app, cleanup, err := infrastructure.InitializeApp()
	if err != nil {
		panic("Failed to initialize application: " + err.Error())
	}
	defer cleanup()
	docs.NewSwagger(traceID, app.Config, app.Logger)

	app.Logger.Info(traceID, "Starting server", map[string]any{
		"port": app.Config.Port,
	})

	go func() {
		if err := app.FiberApp.Listen(":" + app.Config.Port); err != nil {
			app.Logger.Fatal(traceID, "HTTP server Listen failed", []log.Field{log.Error(err)})
		}
		app.Logger.Info(traceID, "HTTP server stopped", nil)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.Logger.Info(traceID, "Shutdown signal received, initiating graceful shutdown", nil)

	if app.Config.Env != config.DEV {
		timeoutSeconds := 10
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		if err := app.FiberApp.ShutdownWithContext(ctx); err != nil {
			app.Logger.Fatal(traceID, "Server forced to shutdown", []log.Field{log.Error(err)})
		}
	}

	app.Logger.Info(traceID, "Server exiting gracefully", nil)
}

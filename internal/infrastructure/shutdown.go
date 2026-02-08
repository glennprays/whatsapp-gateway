package infrastructure

import (
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
)

// ShutdownManager orchestrates graceful shutdown of all application components
type ShutdownManager struct {
	app     *App
	server  *Server
	traceID string
	logger  *log.Logger
	config  *config.Config
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(app *App, server *Server, traceID string) *ShutdownManager {
	return &ShutdownManager{
		app:     app,
		server:  server,
		traceID: traceID,
		logger:  app.Logger,
		config:  app.Config,
	}
}

// Shutdown performs graceful shutdown of all components in the correct order
func (sm *ShutdownManager) Shutdown() error {
	sm.logger.Info(sm.traceID, "Initiating graceful shutdown", nil)

	// Step 1: Shutdown queue workers first (stop accepting new work)
	if err := sm.shutdownWorkers(); err != nil {
		sm.logger.Error(sm.traceID, "Worker shutdown encountered errors", nil, log.Error(err))
	}

	// Step 2: Shutdown HTTP server (stop accepting new requests)
	if err := sm.shutdownHTTPServer(); err != nil {
		sm.logger.Error(sm.traceID, "HTTP server shutdown encountered errors", nil, log.Error(err))
	}

	// Step 3: Final cleanup (close DB connections, flush logs, etc.)
	sm.cleanup()

	sm.logger.Info(sm.traceID, "Graceful shutdown completed", nil)
	return nil
}

// shutdownWorkers gracefully shuts down queue workers
func (sm *ShutdownManager) shutdownWorkers() error {
	if sm.app.Workers == nil {
		return nil
	}

	sm.logger.Info(sm.traceID, "Shutting down queue workers...", nil)

	timeout := 10 * time.Second
	if err := sm.app.Workers.Shutdown(timeout); err != nil {
		return err
	}

	sm.logger.Info(sm.traceID, "Queue workers shut down successfully", nil)
	return nil
}

// shutdownHTTPServer gracefully shuts down the HTTP server
func (sm *ShutdownManager) shutdownHTTPServer() error {
	// Only perform graceful shutdown in production
	// In development, allow immediate shutdown
	if sm.config.Env == config.DEV {
		sm.logger.Info(sm.traceID, "Development mode: skipping graceful HTTP shutdown", nil)
		return nil
	}

	timeout := 10 * time.Second
	return sm.server.Shutdown(sm.traceID, timeout)
}

// cleanup performs final cleanup operations
func (sm *ShutdownManager) cleanup() {
	sm.logger.Info(sm.traceID, "Performing final cleanup...", nil)

	// Sync logger (flush any buffered logs)
	if err := sm.logger.Sync(); err != nil {
		// Ignore sync errors on stdout/stderr (common on Linux)
		// These are harmless and expected
	}

	sm.logger.Info(sm.traceID, "Cleanup completed", nil)
}

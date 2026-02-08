package infrastructure

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/google/uuid"
)

// Runtime manages the application lifecycle
type Runtime struct {
	app           *App
	server        *Server
	shutdownMgr   *ShutdownManager
	traceID       string
	signalChannel chan os.Signal
}

// NewRuntime creates a new runtime instance
func NewRuntime(app *App) (*Runtime, error) {
	traceID := uuid.New().String()

	// Initialize Swagger documentation
	docs.NewSwagger(traceID, app.Config, app.Logger)

	// Create HTTP server
	server := NewServer(app.FiberApp, app.Config.Port, app.Logger)

	// Create shutdown manager
	shutdownMgr := NewShutdownManager(app, server, traceID)

	// Setup signal handling
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	return &Runtime{
		app:           app,
		server:        server,
		shutdownMgr:   shutdownMgr,
		traceID:       traceID,
		signalChannel: signalChannel,
	}, nil
}

// Run starts the application and blocks until shutdown signal is received
func (r *Runtime) Run() error {
	r.app.Logger.Info(r.traceID, "Application starting", nil)

	// Start HTTP server
	serverErrChan := r.server.Start(r.traceID)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrChan:
		if err != nil {
			r.app.Logger.Fatal(r.traceID, "Server error, shutting down", err)
			return err
		}
		// Server stopped gracefully
		return nil

	case sig := <-r.signalChannel:
		r.app.Logger.Info(r.traceID, "Shutdown signal received", map[string]any{
			"signal": sig.String(),
		})

		// Perform graceful shutdown
		return r.shutdownMgr.Shutdown()
	}
}

// Run is the main entry point for starting the application
// This is called from cmd/api/main.go
func Run() error {
	// Initialize application with all dependencies
	app, cleanup, err := InitializeApp()
	if err != nil {
		return err
	}
	defer cleanup()

	// Create and start runtime
	runtime, err := NewRuntime(app)
	if err != nil {
		return err
	}

	return runtime.Run()
}

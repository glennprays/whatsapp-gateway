package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/glennprays/log"
	"github.com/gofiber/fiber/v2"
)

// Server wraps the Fiber HTTP server with lifecycle management
type Server struct {
	app    *fiber.App
	port   string
	logger *log.Logger
}

// NewServer creates a new HTTP server instance
func NewServer(app *fiber.App, port string, logger *log.Logger) *Server {
	return &Server{
		app:    app,
		port:   port,
		logger: logger,
	}
}

// Start begins listening for HTTP requests in a goroutine
// Returns a channel that will be closed when the server stops
func (s *Server) Start(traceID string) <-chan error {
	errChan := make(chan error, 1)

	s.logger.Info(traceID, "Starting HTTP server", map[string]any{
		"port": s.port,
	})

	go func() {
		if err := s.app.Listen(":" + s.port); err != nil {
			s.logger.Error(traceID, "HTTP server Listen failed", nil, log.Error(err))
			errChan <- err
			return
		}
		s.logger.Info(traceID, "HTTP server stopped", nil)
		close(errChan)
	}()

	return errChan
}

// Shutdown gracefully shuts down the HTTP server with a timeout
func (s *Server) Shutdown(traceID string, timeout time.Duration) error {
	s.logger.Info(traceID, "Shutting down HTTP server...", nil)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.app.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}

	s.logger.Info(traceID, "HTTP server shut down successfully", nil)
	return nil
}

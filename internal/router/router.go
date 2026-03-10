package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

var (
	cfg            *config.Config
	basePath       string
	authMiddleware *middleware.AuthMiddleware
	logger         *log.Logger
)

func SetupRouter(
	conf *config.Config,
	traceIDMw fiber.Handler,
	authMw *middleware.AuthMiddleware,
	h *handler.Handler,
	lgr *log.Logger,
	queue domainQueue.MessageQueue,
	storage domainStorage.Storage,
) *fiber.App {
	cfg = conf
	basePath = cfg.BasePath
	authMiddleware = authMw
	logger = lgr

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          middleware.ErrorHandler(),
	})

	// Apply trace ID middleware first (must be before any other middleware)
	app.Use(traceIDMw)

	// Apply recovery and default middleware
	app.Use(recover.New())

	app.Use(middleware.NewHTTPLogger(logger))

	api := app.Group(basePath)

	api.Get("/health", func(c *fiber.Ctx) error {
		traceID := middleware.GetTraceID(c)
		logger.Info(traceID, "Health check endpoint accessed", nil)

		response := fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"trace_id":  traceID,
		}

		// Add queue health check if RabbitMQ is enabled
		if cfg.RabbitMQEnabled && queue != nil {
			response["queue"] = fiber.Map{
				"enabled":   true,
				"connected": queue.IsHealthy(),
			}

			if !queue.IsHealthy() {
				response["status"] = "degraded"
			}
		}

		return c.Status(http.StatusOK).JSON(response)
	})

	// Serve llms.txt for AI assistant context
	api.Get("/llms.txt", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendFile("./llms.txt")
	})

	if cfg.EnableDocumentation {
		traceID := fmt.Sprintf("DOCS-INIT:%s", uuid.New().String())
		logger.Info(traceID, "Documentation is enabled, initializing Documentation routes", nil)
		initDocumentationRoutes(app)
	}

	api.Post("/register", h.AuthHandler.Register)

	initWhatsappRoutes(api, h)
	initWebhookRoutes(api, h)
	initMessageRoutes(api, h)

	// Register storage routes
	RegisterStorageRoutes(app, h.StorageHandler, cfg.StorageAPIPath)

	return app
}

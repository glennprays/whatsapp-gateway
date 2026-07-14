package router

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/google/uuid"
)

// NewHtmlEngine creates a new HTML template engine for Fiber
func NewHtmlEngine() *html.Engine {
	engine := html.New("./docs/ui", ".html")
	return engine
}

func SetupRouter(
	cfg *config.Config,
	traceIDMw fiber.Handler,
	authMw *middleware.AuthMiddleware,
	h *handler.Handler,
	lgr *log.Logger,
	queue domainQueue.MessageQueue,
	storage domainStorage.Storage,
	db *sql.DB,
) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          middleware.ErrorHandler(),
		Views:                 NewHtmlEngine(),
	})

	// Apply trace ID middleware first (must be before any other middleware)
	app.Use(traceIDMw)

	// Apply recovery and default middleware
	app.Use(recover.New())

	app.Use(middleware.NewHTTPLogger(lgr))

	api := app.Group(cfg.BasePath)

	api.Get("/health", func(c *fiber.Ctx) error {
		traceID := middleware.GetTraceID(c)
		lgr.Info(traceID, "Health check endpoint accessed", nil)

		status := "ok"
		response := fiber.Map{
			"timestamp": time.Now().Format(time.RFC3339),
			"trace_id":  traceID,
		}

		// Database health check
		if db != nil {
			dbHealthy := db.PingContext(c.Context()) == nil
			response["database"] = fiber.Map{"connected": dbHealthy}
			if !dbHealthy {
				status = "unhealthy"
			}
		}

		// Queue health check
		if cfg.RabbitMQEnabled && queue != nil {
			queueHealthy := queue.IsHealthy()
			response["queue"] = fiber.Map{
				"enabled":   true,
				"connected": queueHealthy,
			}
			if !queueHealthy && status == "ok" {
				status = "degraded"
			}
		}

		response["status"] = status
		httpStatus := http.StatusOK
		if status == "unhealthy" {
			httpStatus = http.StatusServiceUnavailable
		}
		return c.Status(httpStatus).JSON(response)
	})

	// Liveness + readiness probes at the ROOT path (k8s / load-balancer
	// convention), separate from the BasePath-scoped /health alias above.
	//
	// Liveness: the process is up and serving. It deliberately reflects neither
	// DB, queue, nor WhatsApp session state — a live process that momentarily
	// can't reach its DB should not be killed, only drained (via readiness).
	app.Get("/health/live", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{"status": "alive"})
	})

	// Readiness: safe to route traffic to. DB reachable + (if enabled) queue
	// healthy. Deliberately NOT coupled to WhatsApp session health — a node
	// whose sessions are all logged out can still serve /register and QR
	// pairing, so it must stay in rotation.
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		ready := true
		response := fiber.Map{"timestamp": time.Now().Format(time.RFC3339)}

		if db != nil {
			dbHealthy := db.PingContext(c.Context()) == nil
			response["database"] = fiber.Map{"connected": dbHealthy}
			if !dbHealthy {
				ready = false
			}
		}

		if cfg.RabbitMQEnabled && queue != nil {
			queueHealthy := queue.IsHealthy()
			response["queue"] = fiber.Map{"enabled": true, "connected": queueHealthy}
			if !queueHealthy {
				ready = false
			}
		}

		httpStatus := http.StatusOK
		response["status"] = "ready"
		if !ready {
			httpStatus = http.StatusServiceUnavailable
			response["status"] = "not_ready"
		}
		return c.Status(httpStatus).JSON(response)
	})

	// Serve llms.txt for AI assistant context
	api.Get("/llms.txt", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendFile("./llms.txt")
	})

	if cfg.EnableDocumentation {
		traceID := fmt.Sprintf("DOCS-INIT:%s", uuid.New().String())
		lgr.Info(traceID, "Documentation is enabled, initializing Documentation routes", nil)
		initDocumentationRoutes(app, cfg)
	}

	api.Post("/register", middleware.NewRegisterRateLimiter(cfg), h.AuthHandler.Register)

	initWhatsappRoutes(api, h, authMw)
	initWebhookRoutes(api, h, authMw)
	initMessageRoutes(api, h, authMw)
	initContactRoutes(api, h, authMw)

	// Register storage routes
	RegisterStorageRoutes(app, h.StorageHandler, cfg.StorageAPIPath)

	return app
}

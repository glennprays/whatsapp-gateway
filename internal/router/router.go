package router

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
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
) *fiber.App {
	cfg = conf
	basePath = cfg.BasePath
	authMiddleware = authMw
	logger = lgr

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Apply trace ID middleware first (must be before any other middleware)
	app.Use(traceIDMw)

	// Apply recovery and default middleware
	app.Use(recover.New())

	api := app.Group(basePath)

	api.Get("/health", func(c *fiber.Ctx) error {
		traceID := middleware.GetTraceID(c)
		logger.Info(traceID, "Health check endpoint accessed", nil)
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"trace_id":  traceID,
		})
	})

	if cfg.EnableSwagger {
		// Generate a unique trace ID for swagger initialization (no context available yet)
		traceID := "swagger-init-" + cfg.Port
		logger.Info(traceID, "Swagger is enabled, initializing Swagger routes", nil)
		initSwaggerRoutes(app)
	}

	api.Post("/register", h.AuthHandler.Register)

	initWhatsappRoutes(api, h)
	initWebhookRoutes(api, h)

	// Catch-all for 404 Not Found (must be registered LAST)
	app.Use(func(c *fiber.Ctx) error {
		traceID := middleware.GetTraceID(c)
		err := errDomain.NewError(errDomain.ErrNotFound, errors.New("the requested resource could not found"))
		logger.Warn(traceID, "Resource not found", []log.Field{
			log.String("path", c.Path()),
			log.String("method", c.Method()),
		})
		return c.Status(http.StatusNotFound).JSON(httperror.FromError(err))
	})

	return app
}

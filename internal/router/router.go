package router

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	traceIDMw gin.HandlerFunc,
	authMw *middleware.AuthMiddleware,
	h *handler.Handler,
	lgr *log.Logger,
) *gin.Engine {
	cfg = conf
	basePath = cfg.BasePath
	authMiddleware = authMw
	logger = lgr

	router := gin.New()

	// Apply trace ID middleware first (must be before any other middleware)
	router.Use(traceIDMw)

	// Apply recovery and default middleware
	router.Use(gin.Recovery())

	api := router.Group(basePath)

	api.GET("/health", func(c *gin.Context) {
		traceID := middleware.GetTraceID(c)
		logger.Info(traceID, "Health check endpoint accessed", nil)
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"trace_id":  traceID,
		})
	})

	if cfg.EnableSwagger {
		// Generate a unique trace ID for swagger initialization (no context available yet)
		traceID := "swagger-init-" + cfg.Port
		logger.Info(traceID, "Swagger is enabled, initializing Swagger routes", nil)
		initSwaggerRoutes(router)
	}

	api.POST("/register", h.AuthHandler.Register)

	initWhatsappRoutes(api, h)
	initWebhookRoutes(api, h)

	router.NoRoute(func(c *gin.Context) {
		traceID := middleware.GetTraceID(c)
		err := errDomain.NewError(errDomain.ErrNotFound, errors.New("the requested resource could not found"))
		logger.Warn(traceID, "Resource not found", []log.Field{
			log.String("path", c.Request.URL.Path),
			log.String("method", c.Request.Method),
		})
		c.JSON(http.StatusNotFound, httperror.FromError(err))
	})

	router.NoMethod(func(c *gin.Context) {
		traceID := middleware.GetTraceID(c)
		err := errDomain.NewError(errDomain.ErrMethodNotAllowed, errors.New("the requested method is not allowed"))
		logger.Warn(traceID, "Method not allowed", []log.Field{
			log.String("path", c.Request.URL.Path),
			log.String("method", c.Request.Method),
		})
		c.JSON(http.StatusMethodNotAllowed, httperror.FromError(err))
	})

	return router
}

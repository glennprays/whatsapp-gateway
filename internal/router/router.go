package router

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
)

func SetupRouter(
	conf *config.Config,
	authMw *middleware.AuthMiddleware,
	h *handler.Handler,
) *gin.Engine {
	cfg = conf
	basePath = cfg.BasePath
	authMiddleware = authMw

	router := gin.Default()
	api := router.Group(basePath)

	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Format(time.RFC3339)})
	})

	if cfg.EnableSwagger {
		log.Println("Swagger is enabled, initializing Swagger routes...")
		initSwaggerRoutes(router)
	}

	api.POST("/register", h.AuthHandler.Register)

	initWhatsappRoutes(api, h)
	initWebhookRoutes(api, h)

	router.NoRoute(func(c *gin.Context) {
		err := errDomain.NewError(errDomain.ErrNotFound, errors.New("the requested resource could not found"))
		c.JSON(http.StatusNotFound, httperror.FromError(err))
	})

	router.NoMethod(func(c *gin.Context) {
		err := errDomain.NewError(errDomain.ErrMethodNotAllowed, errors.New("the requested method is not allowed"))
		c.JSON(http.StatusMethodNotAllowed, httperror.FromError(err))
	})

	return router
}

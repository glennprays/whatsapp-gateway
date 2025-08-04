package router

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
)

var (
	cfg      *config.Config
	basePath string
)

func SetupRouter(
	conf *config.Config,
	handler *handler.Handler,
) *gin.Engine {
	cfg = conf
	basePath = cfg.BasePath

	router := gin.Default()
	api := router.Group(basePath)

	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Format(time.RFC3339)})
	})

	if cfg.EnableSwagger {
		log.Println("Swagger is enabled, initializing Swagger routes...")
		swaggerGroup := router.Group(fmt.Sprintf(`/%s`, cfg.SwaggerBasePath))
		initSwaggerRoutes(swaggerGroup)
	}

	api.POST("/register", handler.AuthHandler.Register)

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

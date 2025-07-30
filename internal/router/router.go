package router

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/config"
)

var (
	cfg      *config.Config
	basePath string
)

func SetupRouter(
	conf *config.Config,
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
		initSwaggerRoutes(api)
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested resource could not be found.",
		})
	})

	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error":   "Method Not Allowed",
			"message": "The requested method is not allowed for this resource.",
		})
	})

	return router
}

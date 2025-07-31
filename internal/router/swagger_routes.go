package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
)

func initSwaggerRoutes(r *gin.RouterGroup) {
	swaggerGroup := r.Group("/swagger")
	{
		swaggerGroup.Use(middleware.BasicAuthMiddleware(cfg.SwaggerUser, cfg.SwaggerPassword))

		swaggerGroup.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "./ui/index.html")
		})

		swaggerGroup.GET("/yaml", docs.ServeDynamicSwaggerGin)

		swaggerGroup.Static("/ui", "./docs/swagger-ui")
	}
}

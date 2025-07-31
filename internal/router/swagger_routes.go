package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/docs"
)

func initSwaggerRoutes(r *gin.RouterGroup) {
	swaggerGroup := r.Group("/swagger")
	{
		swaggerGroup.Use(docs.BasicAuthMiddleware())

		swaggerGroup.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "./ui/index.html")
		})

		swaggerGroup.GET("/yaml", docs.ServeDynamicSwaggerGin)

		swaggerGroup.Static("/ui", "./docs/swagger-ui")
	}
}

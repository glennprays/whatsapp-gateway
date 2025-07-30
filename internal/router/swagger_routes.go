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

		// Redirect root to Swagger UI
		swaggerGroup.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "./ui/index.html")
		})

		// Serve YAML explicitly
		swaggerGroup.GET("/yaml", docs.ServeDynamicSwagger)

		// Serve Swagger UI under /swagger/ui/*
		swaggerGroup.Static("/ui", "./docs/swagger-ui")
	}
}

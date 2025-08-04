package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
)

func initSwaggerRoutes(r *gin.RouterGroup) {
	{
		r.Use(middleware.BasicAuthMiddleware(cfg.SwaggerUser, cfg.SwaggerPassword))

		r.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "./ui/index.html")
		})

		r.GET("/yaml", docs.ServeDynamicSwaggerGin)

		r.Static("/ui", "./docs/swagger-ui")
	}
}

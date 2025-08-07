package router

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/docs"
)

func initSwaggerRoutes(r *gin.Engine) {
	swaggerGroup := r.Group(fmt.Sprintf(`/%s`, cfg.SwaggerBasePath))
	swaggerGroup.Use(authMiddleware.BasicAuthMiddleware(cfg.SwaggerUser, cfg.SwaggerPassword))

	swaggerGroup.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "./ui/index.html")
	})

	swaggerGroup.GET("/yaml", docs.ServeDynamicSwaggerGin)

	swaggerGroup.Static("/ui", "./docs/swagger-ui")
}

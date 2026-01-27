package router

import (
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/glennprays/whatsapp-gateway/docs"
)

func initSwaggerRoutes(r *fiber.App) {
	swaggerGroup := r.Group(fmt.Sprintf(`/%s`, cfg.SwaggerBasePath))
	swaggerGroup.Use(authMiddleware.BasicAuthMiddleware(cfg.SwaggerUser, cfg.SwaggerPassword))

	swaggerGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("./ui/index.html", http.StatusMovedPermanently)
	})

	swaggerGroup.Get("/yaml", docs.ServeDynamicSwaggerFiber)

	swaggerGroup.Static("/ui", "./docs/swagger-ui")
}

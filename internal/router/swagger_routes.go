package router

import (
	"fmt"
	"net/http"

	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initSwaggerRoutes(r *fiber.App) {
	sessionAuth := middleware.NewSwaggerSessionAuth(
		cfg.SwaggerUser,
		cfg.SwaggerPassword,
	)
	swaggerGroup := r.Group(cfg.SwaggerBasePath, sessionAuth.Handler(cfg.SwaggerBasePath))

	swaggerGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect(fmt.Sprintf("%s/ui/index.html", cfg.SwaggerBasePath), http.StatusMovedPermanently)
	})

	swaggerGroup.Get("/yaml", docs.ServeDynamicSwaggerFiber)

	swaggerGroup.Static("/ui", "docs/swagger-ui", fiber.Static{
		Index: "index.html",
	})
}

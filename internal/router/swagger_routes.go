package router

import (
	"net/http"

	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
)

func initSwaggerRoutes(r *fiber.App) {
	swaggerGroup := r.Group(
		cfg.SwaggerBasePath,
		basicauth.New(basicauth.Config{
			Users: map[string]string{
				cfg.SwaggerUser: cfg.SwaggerPassword,
			},
			Realm: "Swagger Docs",
			Unauthorized: func(c *fiber.Ctx) error {
				c.Set("WWW-Authenticate", `Basic realm="Swagger Docs", charset="UTF-8"`)
				return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
			},
		}),
	)

	// Redirect base path to Swagger UI
	swaggerGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("./ui/index.html", http.StatusMovedPermanently)
	})

	// Serve swagger YAML (or JSON) dynamically
	swaggerGroup.Get("/yaml", docs.ServeDynamicSwaggerFiber)

	// Serve Swagger UI static files
	swaggerGroup.Static("/ui", "docs/swagger-ui", fiber.Static{
		Index: "index.html",
	})
}

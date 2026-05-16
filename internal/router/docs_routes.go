package router

import (
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initDocumentationRoutes(r *fiber.App) {
	sessionAuth := middleware.NewDocsSessionAuth(
		cfg.DocumentationUser,
		cfg.DocumentationPassword,
	)
	docsGroup := r.Group(cfg.DocumentationBasePath, sessionAuth.Handler(cfg.DocumentationBasePath))

	docsGroup.Get("/yaml", docs.ServeDynamicDocumentationFiber)

	docsGroup.Static("/assets", "./docs/ui/assets")
	// remove / from the string
	basePath := cfg.DocumentationBasePath
	if len(basePath) > 0 && basePath[0] == '/' {
		basePath = basePath[1:]
	}
	docsGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"BasePath": basePath,
		})
	})
}

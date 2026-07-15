package router

import (
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initDocumentationRoutes(r *fiber.App, cfg *config.Config) {
	sessionAuth := middleware.NewDocsSessionAuth(
		cfg.DocumentationUser,
		cfg.DocumentationPassword,
	)
	docsGroup := r.Group(cfg.DocumentationBasePath, sessionAuth.Handler(cfg.DocumentationBasePath))

	docsGroup.Get("/yaml", docs.ServeDynamicDocumentationFiber)

	docsGroup.Static("/assets", "./docs/ui/assets")
	docsGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			// URL prefix the console is mounted at (e.g. "/docs" or "" for root).
			// Used verbatim as the asset prefix: {{ .BasePath }}/assets/...
			"BasePath": cfg.DocumentationBasePath,
			// The Go console gets the interactive API (RapiDoc) tab; the public
			// static build (cmd/docs-gen) renders this template with ShowAPI=false.
			"ShowAPI": true,
		})
	})
}

package router

import (
	"encoding/json"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/pkg/docsgen"
	"github.com/gofiber/fiber/v2"
)

func initDocumentationRoutes(r *fiber.App, cfg *config.Config) {
	sessionAuth := middleware.NewDocsSessionAuth(
		cfg.DocumentationUser,
		cfg.DocumentationPassword,
	)
	docsGroup := r.Group(cfg.DocumentationBasePath, sessionAuth.Handler(cfg.DocumentationBasePath))

	docsGroup.Get("/yaml", docs.ServeDynamicDocumentationFiber)

	// Serve the full-text search index, built in-memory at startup from the same
	// docs the static site indexes. Registered before the /assets static mount so
	// this virtual file wins (there is no search-index.json on disk). Degrades to
	// an empty index if the docs can't be read, so search just returns nothing.
	searchJSON := []byte("[]")
	if index, err := docsgen.BuildSearchIndex("./docs/ui/assets/docs"); err == nil {
		if b, err := json.Marshal(index); err == nil {
			searchJSON = b
		}
	}
	docsGroup.Get("/assets/search-index.json", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.Send(searchJSON)
	})

	docsGroup.Static("/assets", "./docs/ui/assets")
	docsGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			// URL prefix the console is mounted at (e.g. "/docs" or "" for root).
			// Used verbatim as the asset prefix: {{ .BasePath }}/assets/...
			"BasePath": cfg.DocumentationBasePath,
			// The Go console gets the interactive API (RapiDoc) tab; the public
			// static build (cmd/docs-gen) renders this template with ShowAPI=false.
			"ShowAPI": true,
			// Running server version, shown as a small badge next to the logo.
			"Version": cfg.AppVersion,
		})
	})
}

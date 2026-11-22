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

	// Marketing landing at the site root — the same docs/ui/landing.html the
	// public static site (cmd/docs-gen) renders at /index.html. Unauthenticated
	// by design (front door); its /docs/ deep links land on the session-authed
	// console below. BasePath="" keeps asset refs at /assets/..., matching the
	// static-site layout; Version drives the header badge.
	r.Get("/", func(c *fiber.Ctx) error {
		return c.Render("landing", fiber.Map{
			"BasePath": "",
			"Version":  cfg.AppVersion,
		})
	})

	// Landing asset refs (favicons, og-image) point at /assets/ — the same tree
	// the console mounts at /docs/assets, exposed once more at the root so the
	// landing page resolves them without a base path prefix.
	r.Static("/assets", "./docs/ui/assets")

	// The landing page links /llms.txt and /openapi.yaml at the root (static-
	// site layout). The runtime serves them under the API base path and the
	// docs group respectively, so alias both here for parity.
	r.Get("/llms.txt", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendFile("./llms.txt")
	})
	r.Get("/openapi.yaml", docs.ServeDynamicDocumentationFiber)

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

// Command docs-gen renders the public, docs-only static site artifacts from the
// same sources the Go console uses, so there is no hand-maintained second copy.
//
// It renders two pages from docs/ui/:
//   - landing.html -> <out>/index.html           (the marketing landing page)
//   - index.html   -> <out>/docs/index.html      (the docs-only console, ShowAPI=false)
//
// It also validates that assets/nav.json covers every markdown doc (a
// build-time drift guard) and writes the full-text search index under
// <out>/docs/assets/. scripts/build-docs-pages.sh invokes it in place of the
// old inline heredocs.
//
// Usage: docs-gen <outDir>   (default outDir: _site); run from the repo root.
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/glennprays/whatsapp-gateway/pkg/docsgen"
)

const (
	landingTemplate = "docs/ui/landing.html"
	indexTemplate   = "docs/ui/index.html"
	navPath         = "docs/ui/assets/nav.json"
	docsDir         = "docs/ui/assets/docs"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docs-gen:", err)
		os.Exit(1)
	}
}

func run() error {
	outDir := "_site"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	// Drift guard: fail the build loudly if the sidebar and docs disagree.
	nav, err := docsgen.LoadNav(navPath)
	if err != nil {
		return err
	}
	keys, err := docsgen.ListMarkdown(docsDir)
	if err != nil {
		return err
	}
	if err := docsgen.ValidateNav(nav, keys); err != nil {
		return err
	}

	tmpl, err := template.ParseFiles(landingTemplate, indexTemplate)
	if err != nil {
		return err
	}

	// Render the landing page at the site root. BasePath="" points asset refs
	// at /assets/ (favicon set + og-image copied there by the build script).
	// Version drives the header badge (empty ⇒ hidden).
	landingData := map[string]any{"BasePath": "", "Version": os.Getenv("DOCS_VERSION")}
	if err := renderTo(tmpl, filepath.Base(landingTemplate), filepath.Join(outDir, "index.html"), landingData); err != nil {
		return err
	}

	// Render the docs-only index.html from the shared console template under
	// /docs/. ShowAPI=false strips the RapiDoc tab; BasePath="/docs" serves
	// engine assets from /docs/assets/.
	docsData := map[string]any{"BasePath": "/docs", "ShowAPI": false, "Version": os.Getenv("DOCS_VERSION")}
	if err := renderTo(tmpl, filepath.Base(indexTemplate), filepath.Join(outDir, "docs", "index.html"), docsData); err != nil {
		return err
	}

	// Write the full-text search index for the static site. The docs engine at
	// /docs/ requests it via buildUrl, i.e. /docs/assets/search-index.json.
	index, err := docsgen.BuildSearchIndex(docsDir)
	if err != nil {
		return err
	}
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return err
	}
	docsAssetsDir := filepath.Join(outDir, "docs", "assets")
	if err := os.MkdirAll(docsAssetsDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsAssetsDir, "search-index.json"), indexJSON, 0o644); err != nil {
		return err
	}

	fmt.Printf("docs-gen: wrote %s/index.html (landing) + %s/index.html + search-index.json (%d docs, %d sections)\n",
		outDir, filepath.Join(outDir, "docs"), len(keys), len(index))
	return nil
}

func renderTo(tmpl *template.Template, name, path string, data any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.ExecuteTemplate(f, name, data)
}

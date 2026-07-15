// Command docs-gen renders the public, docs-only static site artifacts from the
// same sources the Go console uses, so there is no hand-maintained second copy.
//
// Today it renders index.html from docs/ui/index.html with ShowAPI=false and a
// root asset prefix, and validates that assets/nav.json covers every markdown
// doc (a build-time drift guard). scripts/build-docs-pages.sh invokes it in
// place of the old inline heredocs.
//
// Usage: docs-gen <outDir>   (default outDir: _site); run from the repo root.
package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/glennprays/whatsapp-gateway/pkg/docsgen"
)

const (
	indexTemplate = "docs/ui/index.html"
	navPath       = "docs/ui/assets/nav.json"
	docsDir       = "docs/ui/assets/docs"
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

	// Render the docs-only index.html from the shared console template.
	tmpl, err := template.ParseFiles(indexTemplate)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	// ShowAPI=false strips the RapiDoc tab; BasePath="" serves assets from root.
	data := map[string]any{"BasePath": "", "ShowAPI": false}
	if err := tmpl.ExecuteTemplate(f, filepath.Base(indexTemplate), data); err != nil {
		return err
	}

	fmt.Printf("docs-gen: wrote %s/index.html (%d docs in nav)\n", outDir, len(keys))
	return nil
}

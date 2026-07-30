#!/bin/bash
# Builds the public GitHub Pages docs site from docs/ui/.
#
# The docs engine (app.js, styles.css, index.html template, nav.json) is the
# SAME source the Go console serves; this script just copies it and renders the
# docs-only index.html via cmd/docs-gen (ShowAPI=false). No engine is duplicated
# here anymore, so the public site and the console can no longer drift.
#
# Usage: scripts/build-docs-pages.sh [SITE_DIR]   (run from the repo root)

set -e

SITE_DIR="${1:-_site}"

echo "Building documentation to $SITE_DIR..."

# Clean + create structure
mkdir -p "$SITE_DIR/assets/docs"

echo "Copying documentation markdown..."
cp -r docs/ui/assets/docs/* "$SITE_DIR/assets/docs/"

echo "Copying engine + assets..."
cp docs/ui/assets/app.js "$SITE_DIR/assets/"
cp docs/ui/assets/search.js "$SITE_DIR/assets/"
cp docs/ui/assets/nav.json "$SITE_DIR/assets/"
cp docs/ui/assets/styles.css "$SITE_DIR/assets/"
cp docs/ui/assets/marked.min.js "$SITE_DIR/assets/"
cp docs/ui/assets/mermaid.min.js "$SITE_DIR/assets/"
cp docs/ui/assets/highlight.min.js "$SITE_DIR/assets/"

echo "Copying favicons + og image..."
cp docs/ui/assets/favicon-32x32.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon-16x16.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon.ico "$SITE_DIR/assets/"
cp docs/ui/assets/og-image.png "$SITE_DIR/assets/"

echo "Copying llms.txt + openapi.yaml..."
cp llms.txt "$SITE_DIR/"
cp docs/openapi.yaml "$SITE_DIR/"

# Render the docs-only index.html from the shared template (and validate nav).
# DOCS_VERSION drives the header version badge (empty ⇒ hidden).
echo "Rendering index.html (cmd/docs-gen)..."
DOCS_VERSION="${DOCS_VERSION:-$(git describe --tags --always 2>/dev/null || echo "")}" \
  go run ./cmd/docs-gen "$SITE_DIR"

# Prevent Jekyll processing on GitHub Pages
touch "$SITE_DIR/.nojekyll"

echo ""
echo "Done! Site built in $SITE_DIR/"
echo "Documentation count: $(find "$SITE_DIR/assets/docs" -name '*.md' | wc -l | tr -d ' ')"

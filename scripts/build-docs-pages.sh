#!/bin/bash
# Builds the public GitHub Pages site from docs/ui/.
#
# Site layout:
#   _site/index.html            landing page (rendered from docs/ui/landing.html)
#   _site/assets/               favicon set + og-image.png (landing refs only)
#   _site/llms.txt              root URLs stay stable (advertised in README)
#   _site/openapi.yaml
#   _site/docs/index.html       docs engine (rendered from docs/ui/index.html)
#   _site/docs/assets/          app.js, search.js, nav.json, styles.css, marked/
#                               mermaid/highlight.min.js, favicons, og-image,
#                               search-index.json
#   _site/docs/assets/docs/     markdown tree
#
# The docs engine (app.js, styles.css, index.html template, nav.json) is the
# SAME source the Go console serves; this script just copies it and renders
# both pages via cmd/docs-gen. No engine is duplicated here, so the public
# site and the console can no longer drift.
#
# Usage: scripts/build-docs-pages.sh [SITE_DIR]   (run from the repo root)

set -e

SITE_DIR="${1:-_site}"

echo "Building site to $SITE_DIR..."

# Clean + create structure
mkdir -p "$SITE_DIR/assets" "$SITE_DIR/docs/assets/docs"

echo "Copying documentation markdown..."
cp -r docs/ui/assets/docs/* "$SITE_DIR/docs/assets/docs/"

echo "Copying engine + assets..."
cp docs/ui/assets/app.js "$SITE_DIR/docs/assets/"
cp docs/ui/assets/search.js "$SITE_DIR/docs/assets/"
cp docs/ui/assets/nav.json "$SITE_DIR/docs/assets/"
cp docs/ui/assets/styles.css "$SITE_DIR/docs/assets/"
cp docs/ui/assets/marked.min.js "$SITE_DIR/docs/assets/"
cp docs/ui/assets/mermaid.min.js "$SITE_DIR/docs/assets/"
cp docs/ui/assets/highlight.min.js "$SITE_DIR/docs/assets/"

echo "Copying favicons + og image..."
cp docs/ui/assets/favicon-32x32.png "$SITE_DIR/docs/assets/"
cp docs/ui/assets/favicon-16x16.png "$SITE_DIR/docs/assets/"
cp docs/ui/assets/favicon.ico "$SITE_DIR/docs/assets/"
cp docs/ui/assets/favicon.svg "$SITE_DIR/docs/assets/" 2>/dev/null || true
cp docs/ui/assets/og-image.png "$SITE_DIR/docs/assets/"

echo "Copying favicon set + og image to root (landing refs)..."
cp docs/ui/assets/favicon-32x32.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon-16x16.png "$SITE_DIR/assets/"
cp docs/ui/assets/favicon.ico "$SITE_DIR/assets/"
cp docs/ui/assets/favicon.svg "$SITE_DIR/assets/" 2>/dev/null || true
cp docs/ui/assets/og-image.png "$SITE_DIR/assets/"

echo "Copying llms.txt + openapi.yaml..."
cp llms.txt "$SITE_DIR/"
cp docs/openapi.yaml "$SITE_DIR/"

# Render the landing page and the docs engine (and validate nav).
# DOCS_VERSION drives the header version badge (empty ⇒ hidden). Resolve it to
# the highest release tag (clean, never an offset/SHA) the same way docker.yml
# does; needs tags present (the workflow checks out with fetch-depth: 0).
echo "Rendering index.html + docs/index.html (cmd/docs-gen)..."
DOCS_VERSION="${DOCS_VERSION:-$(git tag -l 'v*' --sort=-v:refname 2>/dev/null | head -n 1)}" \
  go run ./cmd/docs-gen "$SITE_DIR"

# Prevent Jekyll processing on GitHub Pages
touch "$SITE_DIR/.nojekyll"

echo ""
echo "Done! Site built in $SITE_DIR/"
echo "Documentation count: $(find "$SITE_DIR/docs/assets/docs" -name '*.md' | wc -l | tr -d ' ')"

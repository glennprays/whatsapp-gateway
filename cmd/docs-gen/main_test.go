package main

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
	"testing"

	"github.com/glennprays/whatsapp-gateway/pkg/docsgen"
)

// Tests run from the package dir, so the repo-root template paths are
// re-declared here relative to it (kept independent of main.go's constants).
const (
	testLanding = "../../docs/ui/landing.html"
	testIndex   = "../../docs/ui/index.html"
	testNav     = "../../docs/ui/assets/nav.json"
)

// render executes one of the docs/ui templates with the given data, the same
// way docs-gen does.
func render(t *testing.T, name string, data map[string]any) string {
	t.Helper()
	tmpl, err := template.ParseFiles(testLanding, testIndex)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return buf.String()
}

func renderLanding(t *testing.T, version string) string {
	return render(t, "landing.html", map[string]any{"BasePath": "", "Version": version})
}

func renderDocs(t *testing.T, showAPI bool) string {
	return render(t, "index.html", map[string]any{"BasePath": "/docs", "ShowAPI": showAPI, "Version": "v1.2.3"})
}

func navKeys(t *testing.T) map[string]bool {
	t.Helper()
	nav, err := docsgen.LoadNav(testNav)
	if err != nil {
		t.Fatalf("load nav: %v", err)
	}
	keys := map[string]bool{}
	for _, s := range nav.Sections {
		for _, l := range s.Links {
			keys[l.File] = true
		}
	}
	return keys
}

func TestLandingLinksToDocs(t *testing.T) {
	out := renderLanding(t, "")
	if !strings.Contains(out, `href="/docs/"`) {
		t.Error(`landing page must link to href="/docs/"`)
	}
}

func TestLandingDeepLinksExistInNav(t *testing.T) {
	out := renderLanding(t, "")
	keys := navKeys(t)
	re := regexp.MustCompile(`href="/docs/#docs:([^"]+)"`)
	matches := re.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatal("expected at least one /docs/#docs:<key> deep link on the landing page")
	}
	for _, m := range matches {
		// app.js routes #docs:<key>[:<heading-id>]; only the key must exist.
		key := strings.SplitN(m[1], ":", 2)[0]
		if !keys[key] {
			t.Errorf("landing deep link %q is not a nav.json key", key)
		}
	}
}

func TestLandingFaviconsUnderRootAssets(t *testing.T) {
	out := renderLanding(t, "")
	for _, href := range []string{
		`href="/assets/favicon-32x32.png"`,
		`href="/assets/favicon-16x16.png"`,
		`href="/assets/favicon.ico"`,
	} {
		if !strings.Contains(out, href) {
			t.Errorf("landing page missing %s", href)
		}
	}
}

func TestLandingVersionBadgeOnlyWhenSet(t *testing.T) {
	if out := renderLanding(t, ""); strings.Contains(out, `class="version-badge"`) {
		t.Error("version badge should be hidden when Version is empty")
	}
	out := renderLanding(t, "v9.8.7")
	if !strings.Contains(out, `class="version-badge"`) || !strings.Contains(out, "v9.8.7") {
		t.Error("version badge should render the version when set")
	}
}

func TestDocsConsoleAssetsUnderBasePath(t *testing.T) {
	out := renderDocs(t, false)
	for _, src := range []string{
		"/docs/assets/styles.css",
		"/docs/assets/app.js",
		"/docs/assets/search.js",
	} {
		if !strings.Contains(out, src) {
			t.Errorf("docs console missing %s", src)
		}
	}
}

func TestDocsConsoleAPITabToggle(t *testing.T) {
	if out := renderDocs(t, false); strings.Contains(out, "<rapi-doc") {
		t.Error("ShowAPI=false must not render <rapi-doc>")
	}
	if out := renderDocs(t, true); !strings.Contains(out, "<rapi-doc") {
		t.Error("ShowAPI=true must render <rapi-doc>")
	}
}

// Design guard: the landing page is dark-only (Signal design system).
func TestLandingIsDark(t *testing.T) {
	out := renderLanding(t, "")
	if strings.Contains(out, "--paper: #ffffff") {
		t.Error("landing page still uses the light --paper token")
	}
	re := regexp.MustCompile(`--bg:\s*#([0-9a-fA-F]{6})`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatal("landing page must define a --bg token")
	}
	if !isDark(m[1]) {
		t.Errorf("--bg #%s is not a dark color", m[1])
	}
}

// isDark reports whether a 6-digit hex color has low perceived luminance.
func isDark(hex string) bool {
	var rgb [3]int
	for i := 0; i < 3; i++ {
		var v int
		for _, c := range strings.ToLower(hex[i*2 : i*2+2]) {
			v *= 16
			switch {
			case c >= '0' && c <= '9':
				v += int(c - '0')
			default:
				v += int(c-'a') + 10
			}
		}
		rgb[i] = v
	}
	return (299*rgb[0]+587*rgb[1]+114*rgb[2])/1000 < 40
}

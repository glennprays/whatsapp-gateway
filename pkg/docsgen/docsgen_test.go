package docsgen

import (
	"strings"
	"testing"
)

func TestSlugifyMatchesJS(t *testing.T) {
	cases := map[string]string{
		"Hello, World!":        "hello-world",
		"HMAC_MASTER_KEY":      "hmac_master_key", // underscores preserved (\w)
		"Getting  Started":     "getting-started",
		"[IMPORTANT] Security": "important-security",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildSearchIndexCoversDocs(t *testing.T) {
	idx, err := BuildSearchIndex(docsDir)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(idx) == 0 {
		t.Fatal("empty search index")
	}
	var sawWebhook bool
	for _, r := range idx {
		if r.File == "" {
			t.Errorf("record with empty File: %+v", r)
		}
		if r.Heading != "" && r.HeadingID == "" {
			t.Errorf("heading %q has no slug", r.Heading)
		}
		if strings.Contains(strings.ToLower(r.Text), "webhook") {
			sawWebhook = true
		}
	}
	if !sawWebhook {
		t.Error("expected some indexed section to mention 'webhook'")
	}
}

func TestParseDocDropsMermaidKeepsCode(t *testing.T) {
	md := "# Title\n\nIntro prose.\n\n## Flow\n\n" +
		"```mermaid\ngraph TD; A-->B; SECRETDIAGRAM\n```\n\n" +
		"```bash\necho KEEPTHISCODE\n```\n"
	recs := parseDoc("x/y", strings.NewReader(md))
	all := ""
	for _, r := range recs {
		all += r.Text + "|"
	}
	if strings.Contains(all, "SECRETDIAGRAM") {
		t.Error("mermaid block source should be stripped from the index")
	}
	if !strings.Contains(all, "KEEPTHISCODE") {
		t.Error("non-mermaid code should remain searchable")
	}
	if recs[0].DocTitle != "Title" {
		t.Errorf("docTitle = %q, want Title", recs[0].DocTitle)
	}
}

const (
	navPath = "../../docs/ui/assets/nav.json"
	docsDir = "../../docs/ui/assets/docs"
)

// TestNavCoversAllDocs is the drift guard: it fails if any markdown doc on disk
// is not reachable from nav.json, or if nav.json references a doc that does not
// exist. This is exactly the check that would have caught group-management.md
// falling off the old hand-synced sidebar.
func TestNavCoversAllDocs(t *testing.T) {
	nav, err := LoadNav(navPath)
	if err != nil {
		t.Fatalf("load nav: %v", err)
	}
	keys, err := ListMarkdown(docsDir)
	if err != nil {
		t.Fatalf("list docs: %v", err)
	}
	if len(keys) == 0 {
		t.Fatalf("found no markdown docs under %s", docsDir)
	}
	if err := ValidateNav(nav, keys); err != nil {
		t.Error(err)
	}
}

func TestValidateNavReportsBothDirections(t *testing.T) {
	nav := &Nav{Sections: []Section{{Title: "S", Links: []Link{{File: "a"}, {File: "ghost"}}}}}
	err := ValidateNav(nav, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected an error for mismatched nav")
	}
	msg := err.Error()
	// "b" is on disk but not in nav; "ghost" is in nav but not on disk.
	if !strings.Contains(msg, "b") || !strings.Contains(msg, "ghost") {
		t.Errorf("error should mention both b and ghost, got: %s", msg)
	}
}

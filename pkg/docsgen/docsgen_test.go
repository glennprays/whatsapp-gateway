package docsgen

import (
	"strings"
	"testing"
)

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

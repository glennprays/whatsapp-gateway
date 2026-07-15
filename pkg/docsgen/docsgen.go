// Package docsgen holds the shared logic for the documentation site: the nav
// manifest model plus a drift guard that keeps the sidebar (assets/nav.json) in
// sync with the markdown docs on disk. Both the Go console and the static
// GitHub Pages generator (cmd/docs-gen) build on it, so the two surfaces can no
// longer diverge the way the old bash-heredoc copy did.
package docsgen

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Link is one sidebar entry. File is the doc key: the path under the docs dir
// without the ".md" suffix, using forward slashes (e.g. "mcp/quick-start").
type Link struct {
	Title string `json:"title"`
	File  string `json:"file"`
}

// Section is a titled group of links in the sidebar.
type Section struct {
	Title string `json:"title"`
	Links []Link `json:"links"`
}

// Nav is the whole sidebar manifest (assets/nav.json), the single source of
// truth consumed by app.js at runtime.
type Nav struct {
	DefaultDoc string    `json:"defaultDoc"`
	Sections   []Section `json:"sections"`
}

// LoadNav reads and parses a nav.json manifest.
func LoadNav(path string) (*Nav, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var nav Nav
	if err := json.Unmarshal(b, &nav); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &nav, nil
}

// docKeys returns every link's File across all sections.
func (n *Nav) docKeys() []string {
	var keys []string
	for _, s := range n.Sections {
		for _, l := range s.Links {
			keys = append(keys, l.File)
		}
	}
	return keys
}

// ListMarkdown walks docsDir and returns the doc key of every ".md" file: the
// path relative to docsDir without the extension, slash-separated and sorted.
func ListMarkdown(docsDir string) ([]string, error) {
	var keys []string
	err := filepath.WalkDir(docsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(docsDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(strings.TrimSuffix(rel, ".md"))
		keys = append(keys, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

// ValidateNav is the drift guard: it fails if any markdown doc on disk is
// missing from the nav (unreachable in the UI), or if any nav link points at a
// doc that does not exist. Returns a single error listing every mismatch.
func ValidateNav(nav *Nav, docKeysOnDisk []string) error {
	inNav := map[string]bool{}
	for _, k := range nav.docKeys() {
		inNav[k] = true
	}
	onDisk := map[string]bool{}
	for _, k := range docKeysOnDisk {
		onDisk[k] = true
	}

	var missingFromNav, missingFromDisk []string
	for _, k := range docKeysOnDisk {
		if !inNav[k] {
			missingFromNav = append(missingFromNav, k)
		}
	}
	for _, k := range nav.docKeys() {
		if !onDisk[k] {
			missingFromDisk = append(missingFromDisk, k)
		}
	}
	sort.Strings(missingFromNav)
	sort.Strings(missingFromDisk)

	if len(missingFromNav) == 0 && len(missingFromDisk) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("nav.json is out of sync with the docs on disk:")
	if len(missingFromNav) > 0 {
		fmt.Fprintf(&b, "\n  docs on disk missing from nav (unreachable): %s", strings.Join(missingFromNav, ", "))
	}
	if len(missingFromDisk) > 0 {
		fmt.Fprintf(&b, "\n  nav links with no doc on disk: %s", strings.Join(missingFromDisk, ", "))
	}
	return fmt.Errorf("%s", b.String())
}

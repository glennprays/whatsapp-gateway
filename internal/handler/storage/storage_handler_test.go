package storage

import (
	"strings"
	"testing"
)

func TestFormatContentDisposition_Normal(t *testing.T) {
	got := formatContentDisposition("photo.jpg")
	if !strings.Contains(got, "inline") {
		t.Errorf("expected 'inline' disposition, got %q", got)
	}
	if !strings.Contains(got, "photo.jpg") {
		t.Errorf("expected filename photo.jpg in header, got %q", got)
	}
}

func TestFormatContentDisposition_Quotes(t *testing.T) {
	got := formatContentDisposition(`file "name".jpg`)
	// The filename should be properly encoded/escaped, not raw quotes
	if strings.Contains(got, `"name"`) {
		t.Errorf("raw quotes should be escaped in Content-Disposition, got %q", got)
	}
	if !strings.Contains(got, "inline") {
		t.Errorf("expected 'inline' disposition, got %q", got)
	}
}

func TestFormatContentDisposition_PathTraversal(t *testing.T) {
	got := formatContentDisposition("../../../etc/passwd")
	// filepath.Base should strip the directory traversal
	if strings.Contains(got, "..") {
		t.Errorf("path traversal not stripped, got %q", got)
	}
	if !strings.Contains(got, "passwd") {
		t.Errorf("expected base filename 'passwd' in header, got %q", got)
	}
}

func TestFormatContentDisposition_NestedPath(t *testing.T) {
	got := formatContentDisposition("uploads/2024/image.png")
	// Should use only the base filename, not the full path
	if strings.Contains(got, "uploads") {
		t.Errorf("directory path should be stripped, got %q", got)
	}
	if !strings.Contains(got, "image.png") {
		t.Errorf("expected base filename 'image.png', got %q", got)
	}
}

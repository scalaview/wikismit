package composer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateDocsReportsOnlyMissingInternalTargets(t *testing.T) {
	docsDir := t.TempDir()
	modulesDir := filepath.Join(docsDir, "modules")
	sharedDir := filepath.Join(docsDir, "shared")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(modulesDir) error = %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sharedDir) error = %v", err)
	}

	content := "# Auth\nSee [Logger](../shared/logger.md) and [Missing](../shared/missing.md).\n"
	if err := os.WriteFile(filepath.Join(modulesDir, "auth.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "logger.md"), []byte("# Logger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(logger.md) error = %v", err)
	}

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if report.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", report.TotalFiles)
	}
	if report.TotalLinks != 2 {
		t.Fatalf("TotalLinks = %d, want 2", report.TotalLinks)
	}
	if len(report.BrokenLinks) != 1 {
		t.Fatalf("len(BrokenLinks) = %d, want 1", len(report.BrokenLinks))
	}
	if report.BrokenLinks[0].LinkTarget != "../shared/missing.md" {
		t.Fatalf("LinkTarget = %q, want %q", report.BrokenLinks[0].LinkTarget, "../shared/missing.md")
	}
}

func TestValidateDocsSkipsExternalAndAnchorLinks(t *testing.T) {
	docsDir := t.TempDir()
	content := "# Auth\n[Docs](https://example.com)\n[Section](#usage)\n"
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(index.md) error = %v", err)
	}

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if report.TotalLinks != 0 {
		t.Fatalf("TotalLinks = %d, want 0", report.TotalLinks)
	}
	if len(report.BrokenLinks) != 0 {
		t.Fatalf("len(BrokenLinks) = %d, want 0", len(report.BrokenLinks))
	}
}

func TestValidateDocsAllowsInternalLinksWithFragments(t *testing.T) {
	docsDir := t.TempDir()
	modulesDir := filepath.Join(docsDir, "modules")
	sharedDir := filepath.Join(docsDir, "shared")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(modulesDir) error = %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sharedDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulesDir, "auth.md"), []byte("See [Logger](../shared/logger.md#overview)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "logger.md"), []byte("# Logger\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(logger.md) error = %v", err)
	}

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if len(report.BrokenLinks) != 0 {
		t.Fatalf("len(BrokenLinks) = %d, want 0 for existing file with fragment", len(report.BrokenLinks))
	}
}

func TestValidateDocsHandlesEmptyDocsDirectory(t *testing.T) {
	docsDir := t.TempDir()

	report, err := ValidateDocs(docsDir)
	if err != nil {
		t.Fatalf("ValidateDocs() error = %v", err)
	}

	if report.TotalFiles != 0 {
		t.Fatalf("TotalFiles = %d, want 0", report.TotalFiles)
	}
	if report.TotalLinks != 0 {
		t.Fatalf("TotalLinks = %d, want 0", report.TotalLinks)
	}
	if len(report.BrokenLinks) != 0 {
		t.Fatalf("len(BrokenLinks) = %d, want 0", len(report.BrokenLinks))
	}
	if report.GeneratedAt.IsZero() {
		t.Fatalf("GeneratedAt is zero, want timestamp to be set")
	}
	if time.Since(report.GeneratedAt) > time.Minute {
		t.Fatalf("GeneratedAt = %v, want recent timestamp", report.GeneratedAt)
	}
}

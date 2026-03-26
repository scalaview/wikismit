package composer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestGenerateTOCInsertsContentsAfterFirstH1(t *testing.T) {
	content := "# Auth Module\n\nIntro text.\n\n## Overview\nBody.\n\n### Usage\nMore body.\n"

	result := GenerateTOC(content)

	h1Index := strings.Index(result, "# Auth Module\n")
	contentsIndex := strings.Index(result, "## Contents\n")
	overviewIndex := strings.Index(result, "## Overview\n")
	if h1Index == -1 || contentsIndex == -1 || overviewIndex == -1 {
		t.Fatalf("result missing expected sections:\n%s", result)
	}
	if !(h1Index < contentsIndex && contentsIndex < overviewIndex) {
		t.Fatalf("contents block not inserted after first H1:\n%s", result)
	}
	if !strings.Contains(result, "- [Overview](#overview)") {
		t.Fatalf("result missing H2 TOC entry:\n%s", result)
	}
	if !strings.Contains(result, "  - [Usage](#usage)") {
		t.Fatalf("result missing H3 TOC entry:\n%s", result)
	}
}

func TestGenerateTOCSkipsFilesWithoutH2OrH3Headings(t *testing.T) {
	content := "# Auth Module\n\nOnly body text.\n"

	result := GenerateTOC(content)

	if result != content {
		t.Fatalf("GenerateTOC() changed content without H2/H3 headings\nwant: %q\n got: %q", content, result)
	}
}

func TestGenerateTOCBuildsGitHubCompatibleAnchors(t *testing.T) {
	content := "# Auth Module\n\n## Key Functions\n\n### HTTP Handler\n"

	result := GenerateTOC(content)

	if !strings.Contains(result, "- [Key Functions](#key-functions)") {
		t.Fatalf("result missing normalized H2 anchor:\n%s", result)
	}
	if !strings.Contains(result, "  - [HTTP Handler](#http-handler)") {
		t.Fatalf("result missing normalized H3 anchor:\n%s", result)
	}
}

func TestCopyModuleDocsWritesModulesAndSharedDocsToSeparateDirectories(t *testing.T) {
	artifactsDir := t.TempDir()
	docsDir := t.TempDir()
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDocsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "auth.md"), []byte("# Auth\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "logger.md"), []byte("# Logger\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(logger.md) error = %v", err)
	}

	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Shared: false},
		{ID: "logger", Shared: true},
	}}

	if err := CopyModuleDocs(artifactsDir, docsDir, plan, map[string]string{}); err != nil {
		t.Fatalf("CopyModuleDocs() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(docsDir, "modules", "auth.md")); err != nil {
		t.Fatalf("modules/auth.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(docsDir, "shared", "logger.md")); err != nil {
		t.Fatalf("shared/logger.md missing: %v", err)
	}
}

func TestCopyModuleDocsAppliesCitationsAndTOCBeforeWriting(t *testing.T) {
	artifactsDir := t.TempDir()
	docsDir := t.TempDir()
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDocsDir) error = %v", err)
	}
	content := "# Auth\n\nUses `GenerateToken`.\n\n## Overview\n\n### Usage\n"
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "auth.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	plan := &store.NavPlan{Modules: []store.Module{{ID: "auth", Shared: false}}}
	symbolMap := map[string]string{"GenerateToken": "internal/auth/jwt.go#L24"}

	if err := CopyModuleDocs(artifactsDir, docsDir, plan, symbolMap); err != nil {
		t.Fatalf("CopyModuleDocs() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(docsDir, "modules", "auth.md"))
	if err != nil {
		t.Fatalf("ReadFile(rendered auth.md) error = %v", err)
	}
	rendered := string(data)
	if !strings.Contains(rendered, "[GenerateToken](internal/auth/jwt.go#L24)") {
		t.Fatalf("rendered doc missing citation injection:\n%s", rendered)
	}
	if !strings.Contains(rendered, "## Contents\n") {
		t.Fatalf("rendered doc missing TOC:\n%s", rendered)
	}
}

func TestGenerateIndexPageListsModulesByDependencyDepth(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "api", Shared: false},
		{ID: "auth", Shared: false},
		{ID: "db", Shared: false},
	}}
	graph := store.DepGraph{
		"api":  {"auth", "db"},
		"auth": {"db"},
		"db":   nil,
	}

	result := GenerateIndexPage(plan, graph)

	apiIndex := strings.Index(result, "| api |")
	authIndex := strings.Index(result, "| auth |")
	dbIndex := strings.Index(result, "| db |")
	if apiIndex == -1 || authIndex == -1 || dbIndex == -1 {
		t.Fatalf("result missing module rows:\n%s", result)
	}
	if !(dbIndex < authIndex && authIndex < apiIndex) {
		t.Fatalf("modules not ordered shallowest-first by dependency depth:\n%s", result)
	}
}

func TestGenerateIndexPageIncludesSharedUsedByColumn(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "logger", Shared: true, ReferencedBy: []string{"api", "auth"}},
	}}

	result := GenerateIndexPage(plan, store.DepGraph{})

	if !strings.Contains(result, "| logger | shared | api, auth |") {
		t.Fatalf("result missing shared used-by column:\n%s", result)
	}
}

func TestRunComposerWritesDocsIndexAndValidationReport(t *testing.T) {
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDocsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "auth.md"), []byte("# Auth\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	cfg := &configpkg.Config{ArtifactsDir: artifactsDir, OutputDir: outputDir}
	plan := &store.NavPlan{Modules: []store.Module{{ID: "auth", Shared: false}}}
	idx := store.FileIndex{}
	graph := store.DepGraph{"auth": nil}

	if err := RunComposer(cfg, plan, idx, graph); err != nil {
		t.Fatalf("RunComposer() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "index.md")); err != nil {
		t.Fatalf("index.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "validation_report.json")); err != nil {
		t.Fatalf("validation_report.json missing: %v", err)
	}
}

func TestRunComposerCreatesModuleAndSharedDirectories(t *testing.T) {
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDocsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "auth.md"), []byte("# Auth\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "logger.md"), []byte("# Logger\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(logger.md) error = %v", err)
	}

	cfg := &configpkg.Config{ArtifactsDir: artifactsDir, OutputDir: outputDir}
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Shared: false},
		{ID: "logger", Shared: true},
	}}

	if err := RunComposer(cfg, plan, store.FileIndex{}, store.DepGraph{}); err != nil {
		t.Fatalf("RunComposer() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "modules")); err != nil {
		t.Fatalf("modules directory missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "shared")); err != nil {
		t.Fatalf("shared directory missing: %v", err)
	}
}

func TestRunComposerWritesVitePressConfigAndOptionalLogo(t *testing.T) {
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(moduleDocsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDocsDir, "auth.md"), []byte("# Auth\n\n## Overview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}
	logoPath := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(logoPath, []byte("logo-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile(logoPath) error = %v", err)
	}

	cfg := &configpkg.Config{
		ArtifactsDir: artifactsDir,
		OutputDir:    outputDir,
		RepoPath:     "/tmp/wikismit",
		Site: configpkg.SiteConfig{
			Title: "WikiSmit Docs",
			Logo:  logoPath,
		},
	}
	plan := &store.NavPlan{Modules: []store.Module{{ID: "auth", Shared: false}}}

	if err := RunComposer(cfg, plan, store.FileIndex{}, store.DepGraph{}); err != nil {
		t.Fatalf("RunComposer() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, ".vitepress", "config.mts")); err != nil {
		t.Fatalf("vitepress config missing: %v", err)
	}
	packageJSON, err := os.ReadFile(filepath.Join(outputDir, "package.json"))
	if err != nil {
		t.Fatalf("ReadFile(package.json) error = %v", err)
	}
	for _, want := range []string{
		`"docs:build": "vitepress build"`,
		`"docs:preview": "vitepress preview"`,
		`"docs:dev": "vitepress dev"`,
	} {
		if !strings.Contains(string(packageJSON), want) {
			t.Fatalf("package.json missing %q:\n%s", want, string(packageJSON))
		}
	}
	logoData, err := os.ReadFile(filepath.Join(outputDir, "public", "logo.png"))
	if err != nil {
		t.Fatalf("ReadFile(logo.png) error = %v", err)
	}
	if string(logoData) != "logo-bytes" {
		t.Fatalf("logo content = %q, want %q", string(logoData), "logo-bytes")
	}
}

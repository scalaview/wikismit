package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"

	"github.com/scalaview/wikismit/pkg/store"
)

func TestReadModulePathReturnsGoModModule(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")

	got, err := readModulePath(repoPath)
	if err != nil {
		t.Fatalf("readModulePath() error = %v", err)
	}

	const want = "github.com/wikismit/sample"
	if got != want {
		t.Fatalf("readModulePath() = %q, want %q", got, want)
	}
}

func TestResolveInternalImportsMarksImportsAndResolvedPaths(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	mainImports := idx["cmd/main.go"].Imports
	if len(mainImports) != 1 {
		t.Fatalf("cmd/main.go imports len = %d, want 1", len(mainImports))
	}
	if !mainImports[0].Internal {
		t.Fatal("cmd/main.go import should be marked internal")
	}
	if mainImports[0].ResolvedPath != "internal/api/handler.go" {
		t.Fatalf("cmd/main.go resolved path = %q, want %q", mainImports[0].ResolvedPath, "internal/api/handler.go")
	}

	jwtImports := idx["internal/auth/jwt.go"].Imports
	if len(jwtImports) != 2 {
		t.Fatalf("internal/auth/jwt.go imports len = %d, want 2", len(jwtImports))
	}
	for _, imp := range jwtImports {
		if !imp.Internal {
			t.Fatalf("jwt import %q should be internal", imp.Path)
		}
		if imp.ResolvedPath == "" {
			t.Fatalf("jwt import %q should have a resolved path", imp.Path)
		}
	}

	apiImports := idx["internal/api/handler.go"].Imports
	if len(apiImports) != 2 {
		t.Fatalf("internal/api/handler.go imports len = %d, want 2", len(apiImports))
	}
	for _, imp := range apiImports {
		if !imp.Internal {
			t.Fatalf("api import %q should be internal", imp.Path)
		}
	}
	if apiImports[0].ResolvedPath == "" || apiImports[1].ResolvedPath == "" {
		t.Fatal("api imports should have resolved paths")
	}

	errorsImports := idx["pkg/errors/errors.go"].Imports
	if len(errorsImports) != 1 {
		t.Fatalf("pkg/errors/errors.go imports len = %d, want 1", len(errorsImports))
	}
	if errorsImports[0].Internal {
		t.Fatalf("stdlib import %q should remain external", errorsImports[0].Path)
	}
	if errorsImports[0].ResolvedPath != "" {
		t.Fatalf("stdlib import %q resolved path = %q, want empty", errorsImports[0].Path, errorsImports[0].ResolvedPath)
	}
}

func TestBuildDepGraphIncludesEdgesForInternalImports(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)
	jwtDeps := graph["internal/auth/jwt.go"]
	if len(jwtDeps) != 2 {
		t.Fatalf("len(jwt deps) = %d, want 2", len(jwtDeps))
	}
	if jwtDeps[0] != "pkg/errors/errors.go" || jwtDeps[1] != "pkg/logger/logger.go" {
		t.Fatalf("jwt deps = %#v, want pkg/errors/errors.go and pkg/logger/logger.go", jwtDeps)
	}
}

func TestBuildDepGraphIncludesFilesWithNoInternalImports(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)
	loggerDeps, ok := graph["pkg/logger/logger.go"]
	if !ok {
		t.Fatal("dep graph missing pkg/logger/logger.go")
	}
	if len(loggerDeps) != 0 {
		t.Fatalf("logger deps = %#v, want empty slice", loggerDeps)
	}
}

func TestBuildDepGraphOmitsThirdPartyEdges(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)
	errorsDeps := graph["pkg/errors/errors.go"]
	if len(errorsDeps) != 0 {
		t.Fatalf("errors deps = %#v, want empty slice because stdlib imports should not create edges", errorsDeps)
	}
	if len(graph) != len(idx) {
		t.Fatalf("dep graph keys = %d, want %d", len(graph), len(idx))
	}
}

func TestResolveInternalImportsHandlesModuleRootImport(t *testing.T) {
	repoPath := copySampleRepo(t)

	rootAlphaPath := filepath.Join(repoPath, "alpha.go")
	if err := os.WriteFile(rootAlphaPath, []byte("package sample\n\nfunc Alpha() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(alpha.go) error = %v", err)
	}
	rootZetaPath := filepath.Join(repoPath, "zeta.go")
	if err := os.WriteFile(rootZetaPath, []byte("package sample\n\nfunc Zeta() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(zeta.go) error = %v", err)
	}

	consumerPath := filepath.Join(repoPath, "internal", "consumer", "consumer.go")
	if err := os.MkdirAll(filepath.Dir(consumerPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(consumer) error = %v", err)
	}
	if err := os.WriteFile(consumerPath, []byte("package consumer\n\nimport \"github.com/wikismit/sample\"\n\nfunc Use() { sample.Alpha() }\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(consumer.go) error = %v", err)
	}

	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	consumerImports := idx["internal/consumer/consumer.go"].Imports
	if len(consumerImports) != 1 {
		t.Fatalf("consumer imports len = %d, want 1", len(consumerImports))
	}
	if !consumerImports[0].Internal {
		t.Fatal("consumer import should be marked internal")
	}
	if consumerImports[0].ResolvedPath != "alpha.go" {
		t.Fatalf("consumer resolved path = %q, want %q", consumerImports[0].ResolvedPath, "alpha.go")
	}
	if resolved, err := resolveInternalImportPath(repoPath, "github.com/wikismit/sample", "github.com/wikismit/sample/internal/api"); err != nil {
		t.Fatalf("resolveInternalImportPath(subpackage) error = %v", err)
	} else if resolved != "internal/api/handler.go" {
		t.Fatalf("resolveInternalImportPath(subpackage) = %q, want %q", resolved, "internal/api/handler.go")
	}
}

func TestReadWorkspaceModulesParsesGoWork(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")

	modules, err := readWorkspaceModules(repoPath)
	if err != nil {
		t.Fatalf("readWorkspaceModules() error = %v", err)
	}

	if len(modules) != 2 {
		t.Fatalf("len(modules) = %d, want 2", len(modules))
	}

	if dir, ok := modules["github.com/org/service-a"]; !ok {
		t.Fatal("missing module github.com/org/service-a")
	} else if dir != "service-a" {
		t.Fatalf("service-a dir = %q, want %q", dir, "service-a")
	}

	if dir, ok := modules["github.com/org/shared"]; !ok {
		t.Fatal("missing module github.com/org/shared")
	} else if dir != "shared" {
		t.Fatalf("shared dir = %q, want %q", dir, "shared")
	}
}

func TestReadWorkspaceModulesReturnsErrorWithoutGoWork(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := readWorkspaceModules(tmpDir)
	if err == nil {
		t.Fatal("readWorkspaceModules() expected error for directory without go.work")
	}
}

func TestEnsureModulePathDetectsWorkspace(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	if len(analyzer.workspaceModules) != 2 {
		t.Fatalf("workspaceModules len = %d, want 2", len(analyzer.workspaceModules))
	}
}

func TestEnsureModulePathFallsBackToGoMod(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	if analyzer.modulePath != "github.com/wikismit/sample" {
		t.Fatalf("modulePath = %q, want %q", analyzer.modulePath, "github.com/wikismit/sample")
	}
	if len(analyzer.workspaceModules) != 0 {
		t.Fatalf("workspaceModules len = %d, want 0 for single-module project", len(analyzer.workspaceModules))
	}
}

func TestEnsureModulePathReturnsErrorWithoutGoModOrGoWork(t *testing.T) {
	tmpDir := t.TempDir()
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	err := analyzer.ensureModulePath(tmpDir)
	if err == nil {
		t.Fatal("ensureModulePath() expected error for directory without go.mod or go.work")
	}
}

func TestReadWorkspaceModulesRejectsEmptyUseDirectives(t *testing.T) {
	tmpDir := t.TempDir()
	goWorkContent := "go 1.25.0\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.work"), []byte(goWorkContent), 0o644); err != nil {
		t.Fatalf("WriteFile(go.work) error = %v", err)
	}

	_, err := readWorkspaceModules(tmpDir)
	if err == nil {
		t.Fatal("readWorkspaceModules() expected error for go.work with no use directives")
	}
}

func TestResolveImportsMarksCrossModuleImportAsInternal(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	entry := &store.FileEntry{
		Imports: []*store.Import{
			{Path: "github.com/org/shared/pkg/utils"},
		},
	}

	if err := analyzer.resolveImports(repoPath, entry); err != nil {
		t.Fatalf("resolveImports() error = %v", err)
	}

	if !entry.Imports[0].Internal {
		t.Fatalf("import %q should be marked internal (cross-module workspace)", entry.Imports[0].Path)
	}

	const wantResolved = "shared/pkg/utils/utils.go"
	if entry.Imports[0].ResolvedPath != wantResolved {
		t.Fatalf("ResolvedPath = %q, want %q", entry.Imports[0].ResolvedPath, wantResolved)
	}
}

func TestResolveImportsSkipsExternalImportsInWorkspace(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	if err := analyzer.ensureModulePath(repoPath); err != nil {
		t.Fatalf("ensureModulePath() error = %v", err)
	}

	entry := &store.FileEntry{
		Imports: []*store.Import{
			{Path: "fmt"},
			{Path: "github.com/external/something"},
		},
	}

	if err := analyzer.resolveImports(repoPath, entry); err != nil {
		t.Fatalf("resolveImports() error = %v", err)
	}

	for _, imp := range entry.Imports {
		if imp.Internal {
			t.Fatalf("import %q should remain external", imp.Path)
		}
		if imp.ResolvedPath != "" {
			t.Fatalf("import %q should have empty resolved path", imp.Path)
		}
	}
}

func TestResolveImportPathsHandlesWorkspaceFixture(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "workspace_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	resolved, err := ResolveImportPaths(repoPath, configpkg.AnalysisConfig{}, idx)
	if err != nil {
		t.Fatalf("ResolveImportPaths() error = %v", err)
	}

	handlerEntry, ok := resolved["service-a/internal/handler/handler.go"]
	if !ok {
		t.Fatal("resolved FileIndex missing service-a/internal/handler/handler.go")
	}

	for _, imp := range handlerEntry.Imports {
		if imp.Path == "github.com/org/shared/pkg/utils" {
			if !imp.Internal {
				t.Fatal("cross-module import should be marked internal")
			}
			if imp.ResolvedPath != "shared/pkg/utils/utils.go" {
				t.Fatalf("ResolvedPath = %q, want %q", imp.ResolvedPath, "shared/pkg/utils/utils.go")
			}
			return
		}
	}

	t.Fatal("handler missing cross-module import to shared/pkg/utils")
}

func TestFindModuleForImportRequiresPathBoundary(t *testing.T) {
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})
	analyzer.workspaceModules = map[string]string{
		"github.com/org/shared":       "shared",
		"github.com/org/shared-utils": "shared-utils",
	}

	modulePath, moduleDir := analyzer.findModuleForImport("github.com/org/shared-utils/pkg/helper")
	if modulePath != "github.com/org/shared-utils" {
		t.Fatalf("modulePath = %q, want %q", modulePath, "github.com/org/shared-utils")
	}
	if moduleDir != "shared-utils" {
		t.Fatalf("moduleDir = %q, want %q", moduleDir, "shared-utils")
	}

	modulePath, moduleDir = analyzer.findModuleForImport("github.com/org/sharedness/pkg/helper")
	if modulePath != "" || moduleDir != "" {
		t.Fatalf("unexpected match for non-boundary prefix: modulePath=%q moduleDir=%q", modulePath, moduleDir)
	}

	analyzer.workspaceModules = nil
	analyzer.modulePath = "github.com/org/shared"
	modulePath, moduleDir = analyzer.findModuleForImport("github.com/org/sharedness/pkg/helper")
	if modulePath != "" || moduleDir != "" {
		t.Fatalf("unexpected single-module match for non-boundary prefix: modulePath=%q moduleDir=%q", modulePath, moduleDir)
	}
}

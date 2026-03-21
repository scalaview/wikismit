package analyzer

import (
	"path/filepath"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
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

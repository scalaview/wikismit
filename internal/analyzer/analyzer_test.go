package analyzer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
)

func TestNewAnalyzerStoresExcludePatternsAndRegistry(t *testing.T) {
	cfg := configpkg.AnalysisConfig{
		ExcludePatterns: []string{"*_test.go", "vendor/**"},
	}

	analyzer := NewAnalyzer(cfg)

	if analyzer == nil {
		t.Fatal("NewAnalyzer() = nil, want analyzer")
	}
	if analyzer.registry == nil {
		t.Fatal("registry = nil, want parser registry")
	}
	if len(analyzer.excludePatterns) != len(cfg.ExcludePatterns) {
		t.Fatalf("excludePatterns len = %d, want %d", len(analyzer.excludePatterns), len(cfg.ExcludePatterns))
	}
	for idx, pattern := range cfg.ExcludePatterns {
		if analyzer.excludePatterns[idx] != pattern {
			t.Fatalf("excludePatterns[%d] = %q, want %q", idx, analyzer.excludePatterns[idx], pattern)
		}
	}
}

func TestAnalyzeIndexesAllGoFilesInSampleRepo(t *testing.T) {
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	wantFiles := []string{
		"cmd/main.go",
		"internal/api/handler.go",
		"internal/auth/jwt.go",
		"internal/auth/middleware.go",
		"internal/db/client.go",
		"pkg/errors/errors.go",
		"pkg/logger/logger.go",
	}

	if len(idx) != len(wantFiles) {
		t.Fatalf("len(FileIndex) = %d, want %d", len(idx), len(wantFiles))
	}
	for _, path := range wantFiles {
		if _, ok := idx[path]; !ok {
			t.Fatalf("FileIndex missing %q", path)
		}
	}
}

func TestAnalyzeSkipsFilesMatchingExcludePatterns(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	testFilePath := filepath.Join(repoPath, "internal", "auth", "jwt_test.go")
	vendorFilePath := filepath.Join(repoPath, "vendor", "example", "vendor.go")

	if err := os.MkdirAll(filepath.Dir(vendorFilePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(vendor) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(testFilePath)
		_ = os.RemoveAll(filepath.Join(repoPath, "vendor"))
	})

	if err := os.WriteFile(testFilePath, []byte("package auth\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(jwt_test.go) error = %v", err)
	}
	if err := os.WriteFile(vendorFilePath, []byte("package example\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(vendor.go) error = %v", err)
	}

	analyzer := NewAnalyzer(configpkg.AnalysisConfig{
		ExcludePatterns: []string{"*_test.go", "vendor/**"},
	})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if _, ok := idx["internal/auth/jwt_test.go"]; ok {
		t.Fatal("FileIndex unexpectedly contains excluded test file")
	}
	if _, ok := idx["vendor/example/vendor.go"]; ok {
		t.Fatal("FileIndex unexpectedly contains excluded vendor file")
	}
}

func TestAnalyzeSkipsUnknownExtensionsSilently(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	pythonFilePath := filepath.Join(repoPath, "scripts", "helper.py")

	if err := os.MkdirAll(filepath.Dir(pythonFilePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(scripts) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(repoPath, "scripts"))
	})

	if err := os.WriteFile(pythonFilePath, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(helper.py) error = %v", err)
	}

	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if _, ok := idx["scripts/helper.py"]; ok {
		t.Fatal("FileIndex unexpectedly contains unsupported Python file")
	}
}

func TestAnalyzeWarnsAndContinuesOnParseError(t *testing.T) {
	repoPath := t.TempDir()
	goModPath := filepath.Join(repoPath, "go.mod")
	validFilePath := filepath.Join(repoPath, "valid.go")
	invalidFilePath := filepath.Join(repoPath, "broken.go")

	if err := os.WriteFile(goModPath, []byte("module example.com/temp\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
	if err := os.WriteFile(validFilePath, []byte("package sample\n\nfunc Valid() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(valid.go) error = %v", err)
	}
	if err := os.WriteFile(invalidFilePath, []byte("package sample\n\nfunc Broken( {\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(broken.go) error = %v", err)
	}

	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if _, ok := idx["valid.go"]; !ok {
		t.Fatal("FileIndex missing valid.go after parse failure in another file")
	}
	if _, ok := idx["broken.go"]; ok {
		t.Fatal("FileIndex unexpectedly contains broken.go")
	}
	if analyzer.skippedFiles != 1 {
		t.Fatalf("skippedFiles = %d, want 1", analyzer.skippedFiles)
	}
}

func TestRunPhase1WritesFileIndexAndDepGraph(t *testing.T) {
	artifactsDir := t.TempDir()
	cfg := &configpkg.Config{
		RepoPath:     filepath.Join("..", "..", "testdata", "sample_repo"),
		ArtifactsDir: artifactsDir,
		Analysis:     configpkg.AnalysisConfig{},
	}

	if err := RunPhase1(cfg); err != nil {
		t.Fatalf("RunPhase1() error = %v", err)
	}

	fileIndexPath := filepath.Join(artifactsDir, "file_index.json")
	depGraphPath := filepath.Join(artifactsDir, "dep_graph.json")

	fileIndexData, err := os.ReadFile(fileIndexPath)
	if err != nil {
		t.Fatalf("ReadFile(file_index.json) error = %v", err)
	}
	depGraphData, err := os.ReadFile(depGraphPath)
	if err != nil {
		t.Fatalf("ReadFile(dep_graph.json) error = %v", err)
	}
	if len(fileIndexData) == 0 {
		t.Fatal("file_index.json is empty")
	}
	if len(depGraphData) == 0 {
		t.Fatal("dep_graph.json is empty")
	}
}

func TestRunPhase1IsIdempotentForUnchangedRepo(t *testing.T) {
	artifactsDir := t.TempDir()
	cfg := &configpkg.Config{
		RepoPath:     filepath.Join("..", "..", "testdata", "sample_repo"),
		ArtifactsDir: artifactsDir,
		Analysis:     configpkg.AnalysisConfig{},
	}

	if err := RunPhase1(cfg); err != nil {
		t.Fatalf("first RunPhase1() error = %v", err)
	}
	firstFileIndex, err := os.ReadFile(filepath.Join(artifactsDir, "file_index.json"))
	if err != nil {
		t.Fatalf("ReadFile(first file_index.json) error = %v", err)
	}
	firstDepGraph, err := os.ReadFile(filepath.Join(artifactsDir, "dep_graph.json"))
	if err != nil {
		t.Fatalf("ReadFile(first dep_graph.json) error = %v", err)
	}

	if err := RunPhase1(cfg); err != nil {
		t.Fatalf("second RunPhase1() error = %v", err)
	}
	secondFileIndex, err := os.ReadFile(filepath.Join(artifactsDir, "file_index.json"))
	if err != nil {
		t.Fatalf("ReadFile(second file_index.json) error = %v", err)
	}
	secondDepGraph, err := os.ReadFile(filepath.Join(artifactsDir, "dep_graph.json"))
	if err != nil {
		t.Fatalf("ReadFile(second dep_graph.json) error = %v", err)
	}

	if !bytes.Equal(firstFileIndex, secondFileIndex) {
		t.Fatal("file_index.json changed between identical Phase 1 runs")
	}
	if !bytes.Equal(firstDepGraph, secondDepGraph) {
		t.Fatal("dep_graph.json changed between identical Phase 1 runs")
	}
}

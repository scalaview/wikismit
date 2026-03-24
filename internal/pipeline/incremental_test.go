package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scalaview/wikismit/internal/agent"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	logpkg "github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/internal/preprocessor"
	"github.com/scalaview/wikismit/pkg/gitdiff"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestRunFullGenerateVerboseLoggingIncludesFallbackPhaseBoundariesInOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stderr pipe capture is unix-focused")
	}

	repoPath := t.TempDir()
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	writeRepoFile(t, repoPath, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeRepoFile(t, repoPath, "internal/auth/jwt.go", "package auth\n\nfunc GenerateToken() string { return \"token\" }\n")
	if err := os.MkdirAll(filepath.Join(artifactsDir, "module_docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module_docs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "module_docs", "auth.md"), []byte("# Auth\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	cfg := &configpkg.Config{
		RepoPath:     repoPath,
		ArtifactsDir: artifactsDir,
		OutputDir:    outputDir,
		Verbose:      true,
		Analysis:     configpkg.AnalysisConfig{},
		Agent: configpkg.AgentConfig{
			Concurrency:       1,
			SkeletonMaxTokens: 3000,
		},
		LLM: configpkg.LLMConfig{
			PlannerModel:      "planner-test-model",
			PreprocessorModel: "preprocessor-test-model",
			AgentModel:        "agent-test-model",
			MaxTokens:         1024,
			Temperature:       0.2,
		},
	}
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		"# Auth\n",
	)

	out := captureStderrOutput(t, func() {
		if err := runFullGenerate(context.Background(), cfg, client); err != nil {
			t.Fatalf("runFullGenerate() error = %v", err)
		}
	})

	orderedMessages := []string{
		`msg="starting fallback full-generate phase" phase=phase1`,
		`msg="finished fallback full-generate phase" phase=phase1`,
		`msg="starting fallback full-generate phase" phase=planner`,
		`msg="finished fallback full-generate phase" phase=planner`,
		`msg="starting fallback full-generate phase" phase=preprocessor`,
		`msg="finished fallback full-generate phase" phase=preprocessor`,
		`msg="starting fallback full-generate phase" phase=agent`,
		`msg="finished fallback full-generate phase" phase=agent`,
		`msg="starting fallback full-generate phase" phase=composer`,
		`msg="finished fallback full-generate phase" phase=composer`,
	}
	assertOrderedSubstrings(t, out, orderedMessages)

	for _, phase := range []string{"phase1", "planner", "preprocessor", "agent", "composer"} {
		if got := strings.Count(out, "phase="+phase); got != 2 {
			t.Fatalf("log count for %s = %d, want 2; output=%q", phase, got, out)
		}
	}
	if got := strings.Count(out, `msg="finished fallback full-generate phase"`); got != 5 {
		t.Fatalf("finished phase log count = %d, want 5; output=%q", got, out)
	}
	if !regexp.MustCompile(`msg="finished fallback full-generate phase" phase=phase1 .*duration_ms=\d+`).MatchString(out) {
		t.Fatalf("phase1 finish log missing duration_ms: %q", out)
	}
}

func TestRunFullGenerateWithoutVerboseOmitsFallbackPhaseBoundaryDebugLogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stderr pipe capture is unix-focused")
	}

	repoPath := t.TempDir()
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	writeRepoFile(t, repoPath, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeRepoFile(t, repoPath, "internal/auth/jwt.go", "package auth\n\nfunc GenerateToken() string { return \"token\" }\n")
	if err := os.MkdirAll(filepath.Join(artifactsDir, "module_docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module_docs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "module_docs", "auth.md"), []byte("# Auth\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	cfg := &configpkg.Config{
		RepoPath:     repoPath,
		ArtifactsDir: artifactsDir,
		OutputDir:    outputDir,
		Verbose:      false,
		Analysis:     configpkg.AnalysisConfig{},
		Agent: configpkg.AgentConfig{
			Concurrency:       1,
			SkeletonMaxTokens: 3000,
		},
		LLM: configpkg.LLMConfig{
			PlannerModel:      "planner-test-model",
			PreprocessorModel: "preprocessor-test-model",
			AgentModel:        "agent-test-model",
			MaxTokens:         1024,
			Temperature:       0.2,
		},
	}
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		"# Auth\n",
	)

	out := captureStderrOutput(t, func() {
		if err := runFullGenerate(context.Background(), cfg, client); err != nil {
			t.Fatalf("runFullGenerate() error = %v", err)
		}
	})

	for _, unwanted := range []string{
		"starting fallback full-generate phase",
		"finished fallback full-generate phase",
		"phase=phase1",
		"phase=planner",
		"phase=preprocessor",
		"phase=agent",
		"phase=composer",
	} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("non-verbose output unexpectedly contained %q in %q", unwanted, out)
		}
	}
}

func TestRunIncrementalFallsBackToFullGenerateWhenArtifactsMissing(t *testing.T) {
	cfg := &configpkg.Config{ArtifactsDir: t.TempDir()}
	client := llm.NewMockClient()

	called := false
	originalFallback := runGenerateFallback
	runGenerateFallback = func(ctx context.Context, cfg *configpkg.Config, client llm.Client) error {
		called = true
		return nil
	}
	t.Cleanup(func() { runGenerateFallback = originalFallback })

	if err := RunIncremental(context.Background(), cfg, client, IncrementalOptions{}); err != nil {
		t.Fatalf("RunIncremental() error = %v", err)
	}
	if !called {
		t.Fatal("RunIncremental() did not fall back to full generate when artifacts were missing")
	}
}

func TestRunIncrementalUsesChangedFilesOverrideWithoutOpeningGit(t *testing.T) {
	artifactsDir := t.TempDir()
	if err := writeMinimalArtifacts(artifactsDir); err != nil {
		t.Fatalf("writeMinimalArtifacts() error = %v", err)
	}
	if err := store.WriteNavPlan(artifactsDir, store.NavPlan{Modules: []store.Module{{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent"}}}); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	if err := store.WriteDepGraph(artifactsDir, store.DepGraph{"internal/auth/jwt.go": {}}); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}

	cfg := &configpkg.Config{ArtifactsDir: artifactsDir}
	client := llm.NewMockClient()

	originalGetter := getChangedFiles
	originalCompute := computeAffected
	originalPre := runPreprocessorFor
	originalAgent := runAgentFor
	originalComposer := runComposer
	originalReanalyze := reanalyzeChangedFunc
	getChangedFiles = func(repoPath string, baseRef string, headRef string) ([]gitdiff.FileChange, error) {
		t.Fatal("getChangedFiles should not be called when changed-files override is provided")
		return nil, nil
	}
	computeAffected = func(changed []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
		return []store.Module{{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent"}}
	}
	runPreprocessorFor = func(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
		return store.SharedContext{}, nil
	}
	runAgentFor = func(ctx context.Context, modules []store.Module, input agent.AgentInput, client llm.Client, artifactsDir string, concurrency int) error {
		return nil
	}
	runComposer = func(cfg *configpkg.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error {
		return nil
	}
	reanalyzeChangedFunc = func(changes []gitdiff.FileChange, idx store.FileIndex, cfg *configpkg.Config) (store.FileIndex, error) {
		return idx, nil
	}
	t.Cleanup(func() {
		getChangedFiles = originalGetter
		computeAffected = originalCompute
		runPreprocessorFor = originalPre
		runAgentFor = originalAgent
		runComposer = originalComposer
		reanalyzeChangedFunc = originalReanalyze
	})

	if err := RunIncremental(context.Background(), cfg, client, IncrementalOptions{
		ChangedFiles: "internal/auth/jwt.go,pkg/logger/logger.go",
	}); err != nil {
		t.Fatalf("RunIncremental() error = %v", err)
	}
}

func TestReanalyzeChangedUpdatesModifiedAndAddedFiles(t *testing.T) {
	repoPath := t.TempDir()
	writeRepoFile(t, repoPath, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeRepoFile(t, repoPath, "existing.go", "package sample\n\nfunc Existing() {}\n")
	writeRepoFile(t, repoPath, "new.go", "package sample\n\nfunc New() {}\n")

	cfg := &configpkg.Config{
		RepoPath:     repoPath,
		ArtifactsDir: t.TempDir(),
		Analysis:     configpkg.AnalysisConfig{},
	}
	idx := store.FileIndex{
		"existing.go": {ContentHash: "old-hash"},
	}

	got, err := reanalyzeChanged([]gitdiff.FileChange{
		{Path: "existing.go", Type: gitdiff.ChangeModified},
		{Path: "new.go", Type: gitdiff.ChangeAdded},
	}, idx, cfg)
	if err != nil {
		t.Fatalf("reanalyzeChanged() error = %v", err)
	}

	if _, ok := got["existing.go"]; !ok {
		t.Fatal("reanalyzeChanged() missing modified file entry")
	}
	if got["existing.go"].ContentHash == "old-hash" {
		t.Fatal("reanalyzeChanged() did not refresh modified file content hash")
	}
	if _, ok := got["new.go"]; !ok {
		t.Fatal("reanalyzeChanged() missing added file entry")
	}
	if _, err := store.ReadFileIndex(cfg.ArtifactsDir); err != nil {
		t.Fatalf("ReadFileIndex() error = %v", err)
	}
	if _, err := store.ReadDepGraph(cfg.ArtifactsDir); err != nil {
		t.Fatalf("ReadDepGraph() error = %v", err)
	}
}

func TestReanalyzeChangedRemovesDeletedFiles(t *testing.T) {
	cfg := &configpkg.Config{ArtifactsDir: t.TempDir()}
	idx := store.FileIndex{
		"existing.go": {ContentHash: "keep"},
		"deleted.go":  {ContentHash: "drop"},
	}

	got, err := reanalyzeChanged([]gitdiff.FileChange{{Path: "deleted.go", Type: gitdiff.ChangeDeleted}}, idx, cfg)
	if err != nil {
		t.Fatalf("reanalyzeChanged() error = %v", err)
	}
	if _, ok := got["deleted.go"]; ok {
		t.Fatal("reanalyzeChanged() kept deleted file entry")
	}
	if _, ok := got["existing.go"]; !ok {
		t.Fatal("reanalyzeChanged() unexpectedly removed untouched file")
	}
}

func TestReanalyzeChangedHandlesRenamesByDroppingOldPathAndParsingNewPath(t *testing.T) {
	repoPath := t.TempDir()
	writeRepoFile(t, repoPath, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeRepoFile(t, repoPath, "renamed.go", "package sample\n\nfunc Renamed() {}\n")

	cfg := &configpkg.Config{
		RepoPath:     repoPath,
		ArtifactsDir: t.TempDir(),
		Analysis:     configpkg.AnalysisConfig{},
	}
	idx := store.FileIndex{
		"old.go": {ContentHash: "old-hash"},
	}

	got, err := reanalyzeChanged([]gitdiff.FileChange{{Path: "renamed.go", OldPath: "old.go", Type: gitdiff.ChangeRenamed}}, idx, cfg)
	if err != nil {
		t.Fatalf("reanalyzeChanged() error = %v", err)
	}
	if _, ok := got["old.go"]; ok {
		t.Fatal("reanalyzeChanged() kept old renamed path")
	}
	if _, ok := got["renamed.go"]; !ok {
		t.Fatal("reanalyzeChanged() missing renamed file entry")
	}
}

func TestRunPreprocessorForRerunsOnlyAffectedSharedModules(t *testing.T) {
	artifactsDir := t.TempDir()
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "errors", Files: []string{"pkg/errors/errors.go"}, Shared: true, Owner: "shared_preprocessor"},
		{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
	}}
	idx := store.FileIndex{
		"pkg/errors/errors.go": {Functions: []store.FunctionDecl{{Name: "Wrap", Signature: "func Wrap(err error) error", LineStart: 11, Exported: true}}},
		"pkg/logger/logger.go": {Functions: []store.FunctionDecl{{Name: "New", Signature: "func New() Logger", LineStart: 18, Exported: true}}},
	}
	graph := store.DepGraph{
		"pkg/errors/errors.go": {},
		"pkg/logger/logger.go": {"pkg/errors/errors.go"},
	}
	if err := store.WriteSharedContext(artifactsDir, store.SharedContext{
		"errors": {Summary: "existing errors summary"},
	}); err != nil {
		t.Fatalf("WriteSharedContext() error = %v", err)
	}

	cfg := &configpkg.Config{
		ArtifactsDir: artifactsDir,
		Agent:        configpkg.AgentConfig{SkeletonMaxTokens: 3000},
		LLM:          configpkg.LLMConfig{PlannerModel: "planner", MaxTokens: 1024},
	}
	client := llm.NewMockClient(`{"summary":"updated logger summary","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() Logger","ref":"wrong.go#L1"}]}`)

	got, err := preprocessor.RunPreprocessorFor(context.Background(), []store.Module{{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"}}, plan, idx, graph, cfg, client)
	if err != nil {
		t.Fatalf("RunPreprocessorFor() error = %v", err)
	}
	if client.CallCount() != 1 {
		t.Fatalf("MockClient.CallCount() = %d, want 1", client.CallCount())
	}
	if got["errors"].Summary != "existing errors summary" {
		t.Fatalf("errors summary = %q, want preserved existing summary", got["errors"].Summary)
	}
	if !strings.Contains(got["logger"].Summary, "updated logger summary") {
		t.Fatalf("logger summary = %q, want updated summary", got["logger"].Summary)
	}
}

func TestRunForProcessesOnlyAffectedAgentModules(t *testing.T) {
	artifactsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(artifactsDir, "module_docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module_docs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "module_docs", "db.md"), []byte("# Existing DB"), 0o644); err != nil {
		t.Fatalf("WriteFile(db.md) error = %v", err)
	}

	client := llm.NewMockClient("# Auth")
	err := agent.RunFor(context.Background(), []store.Module{{ID: "auth", Owner: "agent"}}, agent.AgentInput{
		Config: &configpkg.Config{LLM: configpkg.LLMConfig{AgentModel: "agent", MaxTokens: 1024}},
	}, client, artifactsDir, 1)
	if err != nil {
		t.Fatalf("RunFor() error = %v", err)
	}
	if client.CallCount() != 1 {
		t.Fatalf("MockClient.CallCount() = %d, want 1", client.CallCount())
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", "auth.md")); err != nil {
		t.Fatalf("auth.md missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(artifactsDir, "module_docs", "db.md"))
	if err != nil {
		t.Fatalf("ReadFile(db.md) error = %v", err)
	}
	if string(data) != "# Existing DB" {
		t.Fatalf("db.md content = %q, want untouched existing doc", string(data))
	}
}

func TestRunIncrementalRerunsSharedDependenciesBeforeAffectedAgentModules(t *testing.T) {
	artifactsDir := t.TempDir()
	if err := store.WriteFileIndex(artifactsDir, store.FileIndex{"pkg/logger/logger.go": {}}); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	if err := store.WriteNavPlan(artifactsDir, store.NavPlan{Modules: []store.Module{{ID: "logger", Shared: true, Owner: "shared_preprocessor"}, {ID: "auth", Owner: "agent"}}}); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	if err := store.WriteDepGraph(artifactsDir, store.DepGraph{"pkg/logger/logger.go": {}}); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}

	cfg := &configpkg.Config{ArtifactsDir: artifactsDir, Agent: configpkg.AgentConfig{Concurrency: 2}}
	client := llm.NewMockClient()
	changes := []gitdiff.FileChange{{Path: "pkg/logger/logger.go", Type: gitdiff.ChangeModified}}
	order := []string{}

	originalGetter := getChangedFiles
	originalCompute := computeAffected
	originalPre := runPreprocessorFor
	originalAgent := runAgentFor
	originalComposer := runComposer
	originalReanalyze := reanalyzeChangedFunc
	getChangedFiles = func(repoPath string, baseRef string, headRef string) ([]gitdiff.FileChange, error) {
		return changes, nil
	}
	computeAffected = func(changed []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
		return []store.Module{{ID: "logger", Shared: true, Owner: "shared_preprocessor"}, {ID: "auth", Owner: "agent"}}
	}
	runPreprocessorFor = func(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
		order = append(order, "preprocessor")
		return store.SharedContext{"logger": {Summary: "logger"}}, nil
	}
	runAgentFor = func(ctx context.Context, modules []store.Module, input agent.AgentInput, client llm.Client, artifactsDir string, concurrency int) error {
		order = append(order, "agent")
		return nil
	}
	runComposer = func(cfg *configpkg.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error {
		order = append(order, "composer")
		return nil
	}
	reanalyzeChangedFunc = func(changes []gitdiff.FileChange, idx store.FileIndex, cfg *configpkg.Config) (store.FileIndex, error) {
		return idx, nil
	}
	t.Cleanup(func() {
		getChangedFiles = originalGetter
		computeAffected = originalCompute
		runPreprocessorFor = originalPre
		runAgentFor = originalAgent
		runComposer = originalComposer
		reanalyzeChangedFunc = originalReanalyze
	})

	if err := RunIncremental(context.Background(), cfg, client, IncrementalOptions{}); err != nil {
		t.Fatalf("RunIncremental() error = %v", err)
	}
	want := []string{"preprocessor", "agent", "composer"}
	if diff := cmp.Diff(want, order); diff != "" {
		t.Fatalf("RunIncremental() order mismatch (-want +got):\n%s", diff)
	}
}

func TestRunIncrementalRunsComposerInFullAfterPartialReruns(t *testing.T) {
	artifactsDir := t.TempDir()
	idx := store.FileIndex{"internal/auth/jwt.go": {ContentHash: "hash"}}
	graph := store.DepGraph{"internal/auth/jwt.go": {}}
	plan := store.NavPlan{Modules: []store.Module{{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent"}}}
	if err := store.WriteFileIndex(artifactsDir, idx); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	if err := store.WriteNavPlan(artifactsDir, plan); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	if err := store.WriteDepGraph(artifactsDir, graph); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}

	cfg := &configpkg.Config{ArtifactsDir: artifactsDir, Agent: configpkg.AgentConfig{Concurrency: 1}}
	client := llm.NewMockClient()
	changes := []gitdiff.FileChange{{Path: "internal/auth/jwt.go", Type: gitdiff.ChangeModified}}

	originalGetter := getChangedFiles
	originalCompute := computeAffected
	originalPre := runPreprocessorFor
	originalAgent := runAgentFor
	originalComposer := runComposer
	originalReanalyze := reanalyzeChangedFunc
	getChangedFiles = func(repoPath string, baseRef string, headRef string) ([]gitdiff.FileChange, error) {
		return changes, nil
	}
	computeAffected = func(changed []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
		return []store.Module{{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent"}}
	}
	runPreprocessorFor = func(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *configpkg.Config, client llm.Client) (store.SharedContext, error) {
		return store.SharedContext{}, nil
	}
	runAgentFor = func(ctx context.Context, modules []store.Module, input agent.AgentInput, client llm.Client, artifactsDir string, concurrency int) error {
		return nil
	}
	composerCalled := false
	runComposer = func(gotCfg *configpkg.Config, gotPlan *store.NavPlan, gotIdx store.FileIndex, gotGraph store.DepGraph) error {
		composerCalled = true
		if diff := cmp.Diff(plan.Modules, gotPlan.Modules); diff != "" {
			t.Fatalf("composer plan mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(idx, gotIdx); diff != "" {
			t.Fatalf("composer idx mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(graph, gotGraph); diff != "" {
			t.Fatalf("composer graph mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
	reanalyzeChangedFunc = func(changes []gitdiff.FileChange, current store.FileIndex, cfg *configpkg.Config) (store.FileIndex, error) {
		return current, nil
	}
	t.Cleanup(func() {
		getChangedFiles = originalGetter
		computeAffected = originalCompute
		runPreprocessorFor = originalPre
		runAgentFor = originalAgent
		runComposer = originalComposer
		reanalyzeChangedFunc = originalReanalyze
	})

	if err := RunIncremental(context.Background(), cfg, client, IncrementalOptions{}); err != nil {
		t.Fatalf("RunIncremental() error = %v", err)
	}
	if !composerCalled {
		t.Fatal("RunIncremental() did not invoke composer")
	}
}

func writeMinimalArtifacts(artifactsDir string) error {
	return store.WriteFileIndex(artifactsDir, store.FileIndex{
		"internal/auth/jwt.go": {},
	})
}

func captureStderrOutput(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w

	outputCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outputCh <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("stderr close error = %v", err)
	}
	os.Stderr = originalStderr
	t.Cleanup(func() {
		os.Stderr = originalStderr
	})

	return <-outputCh
}

func assertOrderedSubstrings(t *testing.T, content string, ordered []string) {
	t.Helper()

	position := 0
	for _, want := range ordered {
		idx := strings.Index(content[position:], want)
		if idx == -1 {
			t.Fatalf("output missing ordered substring %q after byte %d in %q", want, position, content)
		}
		position += idx + len(want)
	}
}

var _ logpkg.Logger

func writeRepoFile(t *testing.T, repoPath string, relPath string, content string) {
	t.Helper()
	path := filepath.Join(repoPath, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

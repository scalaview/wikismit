package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/gitdiff"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestOwningModulesReturnsDirectOwnersForChangedFiles(t *testing.T) {
	changed := []gitdiff.FileChange{
		{Path: "internal/auth/jwt.go", Type: gitdiff.ChangeModified},
		{Path: "pkg/logger/logger.go", Type: gitdiff.ChangeModified},
	}
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}},
		{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
	}}

	got := owningModules(changed, plan)
	want := []string{"auth", "logger"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("owningModules() mismatch (-want +got):\n%s", diff)
	}
}

func TestOwningModulesIgnoresUnknownFiles(t *testing.T) {
	changed := []gitdiff.FileChange{
		{Path: "internal/auth/jwt.go", Type: gitdiff.ChangeModified},
		{Path: "missing/file.go", Type: gitdiff.ChangeModified},
	}
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}},
	}}

	got := owningModules(changed, plan)
	want := []string{"auth"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("owningModules() mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildReverseGraphReversesFileEdges(t *testing.T) {
	graph := store.DepGraph{
		"internal/auth/jwt.go":    {"pkg/errors/errors.go", "pkg/logger/logger.go"},
		"internal/api/handler.go": {"pkg/logger/logger.go"},
		"pkg/logger/logger.go":    {},
		"pkg/errors/errors.go":    {},
	}

	got := buildReverseGraph(graph)
	want := store.DepGraph{
		"internal/auth/jwt.go":    {},
		"internal/api/handler.go": {},
		"pkg/errors/errors.go":    {"internal/auth/jwt.go"},
		"pkg/logger/logger.go":    {"internal/api/handler.go", "internal/auth/jwt.go"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("buildReverseGraph() mismatch (-want +got):\n%s", diff)
	}
}

func TestComputeAffectedReturnsLeafOwnerOnlyForIsolatedChange(t *testing.T) {
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}},
		{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
	}}
	graph := store.DepGraph{
		"internal/auth/jwt.go":        {"pkg/logger/logger.go"},
		"internal/auth/middleware.go": {"pkg/logger/logger.go"},
		"pkg/logger/logger.go":        {},
	}
	changed := []gitdiff.FileChange{{Path: "internal/auth/jwt.go", Type: gitdiff.ChangeModified}}

	got := ComputeAffected(changed, plan, graph)
	want := []store.Module{{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ComputeAffected() mismatch (-want +got):\n%s", diff)
	}
}

func TestComputeAffectedPropagatesSharedModuleChangesToDependents(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})
	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}, Owner: "agent", DependsOnShared: []string{"errors", "logger"}},
		{ID: "api", Files: []string{"internal/api/handler.go"}, Owner: "agent", DependsOnShared: []string{"logger"}},
		{ID: "db", Files: []string{"internal/db/client.go"}, Owner: "agent", DependsOnShared: []string{"errors"}},
		{ID: "errors", Files: []string{"pkg/errors/errors.go"}, Shared: true, Owner: "shared_preprocessor"},
		{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
	}}
	changed := []gitdiff.FileChange{{Path: "pkg/logger/logger.go", Type: gitdiff.ChangeModified}}

	got := moduleIDs(ComputeAffected(changed, plan, graph))
	want := []string{"api", "auth", "db", "logger"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ComputeAffected() ids mismatch (-want +got):\n%s", diff)
	}
}

func TestComputeAffectedHandlesErrorsModuleDependenciesFromSampleRepo(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	analyzer := NewAnalyzer(configpkg.AnalysisConfig{})
	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := BuildDepGraph(idx)
	plan := &store.NavPlan{Modules: []store.Module{
		{ID: "auth", Files: []string{"internal/auth/jwt.go", "internal/auth/middleware.go"}, Owner: "agent", DependsOnShared: []string{"errors", "logger"}},
		{ID: "api", Files: []string{"internal/api/handler.go"}, Owner: "agent", DependsOnShared: []string{"logger"}},
		{ID: "db", Files: []string{"internal/db/client.go"}, Owner: "agent", DependsOnShared: []string{"errors"}},
		{ID: "errors", Files: []string{"pkg/errors/errors.go"}, Shared: true, Owner: "shared_preprocessor"},
		{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
	}}
	changed := []gitdiff.FileChange{{Path: "pkg/errors/errors.go", Type: gitdiff.ChangeModified}}

	got := moduleIDs(ComputeAffected(changed, plan, graph))
	want := []string{"api", "auth", "db", "errors"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ComputeAffected() ids mismatch (-want +got):\n%s", diff)
	}
}

func moduleIDs(modules []store.Module) []string {
	ids := make([]string, 0, len(modules))
	for _, module := range modules {
		ids = append(ids, module.ID)
	}
	return ids
}

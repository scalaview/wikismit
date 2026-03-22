package preprocessor

import (
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func sampleSharedNavPlan() *store.NavPlan {
	return &store.NavPlan{
		Modules: []store.Module{
			{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Shared: false, Owner: "agent"},
			{ID: "errors", Files: []string{"pkg/errors/errors.go"}, Shared: true, Owner: "shared_preprocessor"},
			{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Shared: true, Owner: "shared_preprocessor"},
		},
	}
}

func sampleSharedDepGraph() store.DepGraph {
	return store.DepGraph{
		"internal/auth/jwt.go":   {"pkg/errors/errors.go", "pkg/logger/logger.go"},
		"pkg/errors/errors.go":   {},
		"pkg/logger/logger.go":   {"pkg/errors/errors.go"},
		"pkg/metrics/metrics.go": {"pkg/errors/errors.go"},
	}
}

func TestSharedSubgraphIncludesOnlySharedModuleDependencies(t *testing.T) {
	got := sharedSubgraph(sampleSharedNavPlan(), sampleSharedDepGraph())

	edges := got["logger"]
	if len(edges) != 1 || edges[0] != "errors" {
		t.Fatalf("sharedSubgraph()[logger] = %v, want [errors]", edges)
	}
	if len(got["errors"]) != 0 {
		t.Fatalf("sharedSubgraph()[errors] = %v, want empty", got["errors"])
	}
	if _, ok := got["auth"]; ok {
		t.Fatalf("sharedSubgraph() contains non-shared module auth: %v", got)
	}
}

func TestTopoSortOrdersDependenciesBeforeDependents(t *testing.T) {
	got, err := topoSort(map[string][]string{
		"logger":     {"errors"},
		"middleware": {"logger"},
		"errors":     {},
	})
	if err != nil {
		t.Fatalf("topoSort() error = %v", err)
	}
	want := []string{"errors", "logger", "middleware"}
	for i, wantModule := range want {
		if got[i] != wantModule {
			t.Fatalf("topoSort()[%d] = %q, want %q (full=%v)", i, got[i], wantModule, got)
		}
	}
}

func TestTopoSortReturnsErrorOnCycle(t *testing.T) {
	_, err := topoSort(map[string][]string{
		"A": {"B"},
		"B": {"A"},
	})
	if err == nil {
		t.Fatal("topoSort() error = nil, want error")
	}
}

func TestTopoSortReturnsEmptyOrderForEmptyGraph(t *testing.T) {
	got, err := topoSort(map[string][]string{})
	if err != nil {
		t.Fatalf("topoSort() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("topoSort() = %v, want empty", got)
	}
}

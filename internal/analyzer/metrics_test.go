package analyzer

import (
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

// buildTestFileIndex creates a minimal FileIndex for metrics testing.
// Graph: Entry → A → B, Entry → C → B
func buildTestFileIndex() store.FileIndex {
	return store.FileIndex{
		"pkg/main.go": {
			Path: "pkg/main.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "Entry", Path: "pkg/main.go",
					Exported: true, LineStart: 1, LineEnd: 20,
					Src: "func Entry() {\n\ta()\n\tc()\n}",
					Calls: []*store.CallRef{
						{Name: "a", Path: "pkg/main.go", Ownership: store.OwnershipInternal},
						{Name: "c", Path: "pkg/main.go", Ownership: store.OwnershipInternal},
					},
				},
				{
					Name: "a", Path: "pkg/main.go",
					Exported: false, LineStart: 22, LineEnd: 31,
					Src: "func a() {\n\tb()\n}",
					Calls: []*store.CallRef{
						{Name: "b", Path: "pkg/main.go", Ownership: store.OwnershipInternal},
					},
				},
				{
					Name: "b", Path: "pkg/main.go",
					Exported: false, LineStart: 33, LineEnd: 37,
					Src: "func b() {}\n",
				},
				{
					Name: "c", Path: "pkg/main.go",
					Exported: true, LineStart: 39, LineEnd: 46,
					Src: "func c() error {\n\tb()\n\treturn nil\n}",
					Calls: []*store.CallRef{
						{Name: "b", Path: "pkg/main.go", Ownership: store.OwnershipInternal},
					},
				},
			},
		},
	}
}

func buildTestCallGraph() store.CallGraph {
	return store.CallGraph{
		"pkg/main.go#Entry": {"pkg/main.go#a", "pkg/main.go#c"},
		"pkg/main.go#a":     {"pkg/main.go#b"},
		"pkg/main.go#c":     {"pkg/main.go#b"},
	}
}

func TestComputeMetricsEntryIsEntryPoint(t *testing.T) {
	idx := buildTestFileIndex()
	graph := buildTestCallGraph()

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	entry := metrics["pkg/main.go#Entry"]
	if entry == nil {
		t.Fatal("missing metrics for Entry")
	}
	if !entry.IsEntryPoint {
		t.Fatal("Entry should be an entry point (exported, no internal callers)")
	}
	if entry.DepthFromEntryPoint != 0 {
		t.Fatalf("Entry DepthFromEntryPoint = %d, want 0", entry.DepthFromEntryPoint)
	}
}

func TestComputeMetricsDepthFromEntryPoint(t *testing.T) {
	idx := buildTestFileIndex()
	graph := buildTestCallGraph()

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	tests := []struct {
		funcID string
		want   int
	}{
		{"pkg/main.go#Entry", 0},
		{"pkg/main.go#a", 1},
		{"pkg/main.go#c", 1},
		{"pkg/main.go#b", 2},
	}
	for _, tt := range tests {
		m := metrics[tt.funcID]
		if m == nil {
			t.Fatalf("missing metrics for %s", tt.funcID)
		}
		if m.DepthFromEntryPoint != tt.want {
			t.Errorf("%s DepthFromEntryPoint = %d, want %d", tt.funcID, m.DepthFromEntryPoint, tt.want)
		}
	}
}

func TestComputeMetricsReachableFromEntry(t *testing.T) {
	idx := buildTestFileIndex()
	graph := buildTestCallGraph()

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	b := metrics["pkg/main.go#b"]
	if b == nil {
		t.Fatal("missing metrics for b")
	}
	// b is reachable from Entry→a→b AND Entry→c→b = 2 paths
	if b.ReachableFromEntry != 2 {
		t.Fatalf("b ReachableFromEntry = %d, want 2", b.ReachableFromEntry)
	}
}

func TestComputeMetricsOutDegree(t *testing.T) {
	idx := buildTestFileIndex()
	graph := buildTestCallGraph()

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	entry := metrics["pkg/main.go#Entry"]
	if entry.OutDegree != 2 {
		t.Fatalf("Entry OutDegree = %d, want 2", entry.OutDegree)
	}

	b := metrics["pkg/main.go#b"]
	if b.OutDegree != 0 {
		t.Fatalf("b OutDegree = %d, want 0", b.OutDegree)
	}
}

func TestComputeMetricsEmptyCallGraph(t *testing.T) {
	idx := store.FileIndex{
		"pkg/util.go": {
			Path: "pkg/util.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "Helper", Path: "pkg/util.go",
					Exported: false, Src: "func Helper() {}\n",
				},
			},
		},
	}
	graph := store.CallGraph{}

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	m := metrics["pkg/util.go#Helper"]
	if m == nil {
		t.Fatal("missing metrics for Helper")
	}
	if m.DepthFromEntryPoint != -1 {
		t.Fatalf("unreachable function DepthFromEntryPoint = %d, want -1", m.DepthFromEntryPoint)
	}
	if m.IsEntryPoint {
		t.Fatal("unexported function with no callers should not be entry point")
	}
	if m.ReachableFromEntry != 0 {
		t.Fatalf("ReachableFromEntry = %d, want 0", m.ReachableFromEntry)
	}
}

func TestComputeMetricsNilFunction(t *testing.T) {
	idx := store.FileIndex{
		"pkg/main.go": {
			Path: "pkg/main.go",
			Functions: []*store.FunctionDecl{
				nil,
				{Name: "Foo", Path: "pkg/main.go", Exported: true, Src: "func Foo(){}\n"},
			},
		},
	}
	graph := store.CallGraph{}

	mc := NewMetricsComputer()
	metrics := mc.Compute(idx, graph)

	if len(metrics) != 1 {
		t.Fatalf("MetricsMap length = %d, want 1 (nil function skipped)", len(metrics))
	}
}

func TestWeightedScoringUnreachableGetsLowScore(t *testing.T) {
	s := DefaultWeightedScoring()
	ctx := ScoringContext{TotalFunctions: 10, MaxReachable: 5, MaxDepth: 3}

	m := &store.FunctionMetrics{
		DepthFromEntryPoint: -1,
		ReachableFromEntry:  0,
		IsExported:          false,
		IsEntryPoint:        false,
		LinesOfCode:         3,
	}

	score := s.Score(m, ctx)
	if score > 0.2 {
		t.Fatalf("unreachable trivial function score = %.3f, want <= 0.2", score)
	}
}

func TestWeightedScoringEntryPointGetsHighScore(t *testing.T) {
	s := DefaultWeightedScoring()
	ctx := ScoringContext{TotalFunctions: 10, MaxReachable: 5, MaxDepth: 3}

	m := &store.FunctionMetrics{
		DepthFromEntryPoint: 0,
		ReachableFromEntry:  5,
		IsExported:          true,
		IsEntryPoint:        true,
		LinesOfCode:         50,
	}

	score := s.Score(m, ctx)
	if score < 0.8 {
		t.Fatalf("entry point score = %.3f, want >= 0.8", score)
	}
}

package pipeline

import (
	"testing"

	"github.com/scalaview/wikismit/internal/analyzer"
	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

func buildIntegrationFileIndex() store.FileIndex {
	return store.FileIndex{
		"svc/handler.go": {
			Path:     "svc/handler.go",
			Language: "go",
			Functions: []*store.FunctionDecl{
				{
					Name:      "HandleRequest",
					Path:      "svc/handler.go",
					Signature: "func HandleRequest(w http.ResponseWriter, r *http.Request)",
					Exported:  true,
					LineStart: 1,
					LineEnd:   30,
					Src: "func HandleRequest(w http.ResponseWriter, r *http.Request) {\n\tprocess(r)\n\tlogRequest(r)\n}",
					Calls: []*store.CallRef{
						{Name: "process", Path: "svc/handler.go", Ownership: store.OwnershipInternal},
						{Name: "logRequest", Path: "svc/handler.go", Ownership: store.OwnershipInternal},
					},
				},
				{
					Name:      "process",
					Path:      "svc/handler.go",
					Signature: "func process(r *http.Request)",
					Exported:  false,
					LineStart: 32,
					LineEnd:   50,
					Src:       "func process(r *http.Request) {\n\tdoWork(r)\n}",
					Calls: []*store.CallRef{
						{Name: "doWork", Path: "svc/handler.go", Ownership: store.OwnershipInternal},
					},
				},
				{
					Name:      "logRequest",
					Path:      "svc/handler.go",
					Signature: "func logRequest(r *http.Request)",
					Exported:  false,
					LineStart: 52,
					LineEnd:   54,
					Src:       "func logRequest(r *http.Request) {\n\t// logging\n}",
				},
				{
					Name:      "doWork",
					Path:      "svc/handler.go",
					Signature: "func doWork(r *http.Request) error",
					Exported:  false,
					LineStart: 56,
					LineEnd:   80,
					Src:       "func doWork(r *http.Request) error {\n\t// important logic\n\treturn nil\n}",
				},
			},
		},
	}
}

func buildIntegrationCallGraph() store.CallGraph {
	return store.CallGraph{
		"svc/handler.go#HandleRequest": {"svc/handler.go#process", "svc/handler.go#logRequest"},
		"svc/handler.go#process":       {"svc/handler.go#doWork"},
	}
}

func TestMetricsDriveFunctionSummaryFiltering(t *testing.T) {
	idx := buildIntegrationFileIndex()
	graph := buildIntegrationCallGraph()

	// Phase 1: Compute metrics
	mc := analyzer.NewMetricsComputer()
	computed := mc.Compute(idx, graph)

	// Verify entry point detection
	entryFn := computed["svc/handler.go#HandleRequest"]
	if entryFn == nil {
		t.Fatal("HandleRequest should have metrics")
	}
	if !entryFn.IsEntryPoint {
		t.Fatal("HandleRequest should be an entry point (exported, no internal callers)")
	}

	// Verify depth computation
	processFn := computed["svc/handler.go#process"]
	if processFn == nil {
		t.Fatal("process should have metrics")
	}
	if processFn.DepthFromEntryPoint != 1 {
		t.Fatalf("process depth = %d, want 1", processFn.DepthFromEntryPoint)
	}

	// Phase 2: Filter with threshold
	filter := metrics.NewImportanceFilter(computed, 0.2)

	// HandleRequest should pass (high importance)
	if !filter.ShouldSummarize("svc/handler.go#HandleRequest") {
		t.Fatal("HandleRequest should pass threshold 0.2")
	}

	// Count passing functions
	total := 0
	passing := 0
	for _, entry := range idx {
		for _, fn := range entry.Functions {
			id := store.FuncID(fn)
			total++
			if filter.ShouldSummarize(id) {
				passing++
			}
		}
	}
	t.Logf("Importance filter: %d/%d functions pass threshold 0.2", passing, total)
	if passing == 0 {
		t.Fatal("at least some functions should pass the threshold")
	}

	// Phase 3: Verify importance ordering
	sorted := filter.SortByImportance([]string{
		"svc/handler.go#logRequest",
		"svc/handler.go#HandleRequest",
		"svc/handler.go#doWork",
		"svc/handler.go#process",
	})
	if sorted[0] != "svc/handler.go#HandleRequest" {
		t.Fatalf("highest importance should be first, got %q", sorted[0])
	}
}

func TestImportanceFilterWithEmptyMetrics(t *testing.T) {
	filter := metrics.NewImportanceFilter(nil, 0.5)
	// Unknown functions should always be included
	if !filter.ShouldSummarize("unknown#Func") {
		t.Fatal("unknown function should be included when no metrics available")
	}
}

func TestMetricsScoresAreBounded(t *testing.T) {
	idx := buildIntegrationFileIndex()
	graph := buildIntegrationCallGraph()

	mc := analyzer.NewMetricsComputer()
	computed := mc.Compute(idx, graph)

	for id, m := range computed {
		if m.ImportanceScore < 0 || m.ImportanceScore > 1 {
			t.Fatalf("ImportanceScore for %s = %.4f, want [0, 1]", id, m.ImportanceScore)
		}
	}
}

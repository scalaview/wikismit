package planner

import (
	"strings"
	"testing"

	"github.com/scalaview/wikismit/internal/metrics"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestShouldIncludeFunction_EntryPoint(t *testing.T) {
	fn := &store.FunctionDecl{Name: "main", Path: "main.go", LineStart: 1, LineEnd: 20, Exported: true}
	m := &store.FunctionMetrics{FuncID: "main.go#main", IsEntryPoint: true, ImportanceScore: 0.5, LinesOfCode: 20}
	if !shouldIncludeFunction(fn, m, 0, DefaultSkeletonFilterConfig()) {
		t.Error("entry point should always be included")
	}
}

func TestShouldIncludeFunction_ExportedWithCallers(t *testing.T) {
	fn := &store.FunctionDecl{Name: "Handler", Path: "handler.go", LineStart: 10, LineEnd: 30, Exported: true}
	m := &store.FunctionMetrics{FuncID: "handler.go#Handler", ImportanceScore: 0.3, LinesOfCode: 20}
	if !shouldIncludeFunction(fn, m, 3, DefaultSkeletonFilterConfig()) {
		t.Error("exported function with callers should be included")
	}
}

func TestShouldIncludeFunction_TooShort(t *testing.T) {
	fn := &store.FunctionDecl{Name: "getter", Path: "model.go", LineStart: 5, LineEnd: 7}
	m := &store.FunctionMetrics{FuncID: "model.go#getter", ImportanceScore: 0.2, LinesOfCode: 2}
	if shouldIncludeFunction(fn, m, 0, DefaultSkeletonFilterConfig()) {
		t.Error("short function with no callers should be excluded")
	}
}

func TestShouldIncludeFunction_LowImportance(t *testing.T) {
	fn := &store.FunctionDecl{Name: "helper", Path: "util.go", LineStart: 1, LineEnd: 15}
	m := &store.FunctionMetrics{FuncID: "util.go#helper", ImportanceScore: 0.01, LinesOfCode: 15}
	if shouldIncludeFunction(fn, m, 0, DefaultSkeletonFilterConfig()) {
		t.Error("low importance function should be excluded")
	}
}

func TestShouldIncludeFunction_HighlyCalled(t *testing.T) {
	fn := &store.FunctionDecl{Name: "parse", Path: "parser.go", LineStart: 1, LineEnd: 50}
	m := &store.FunctionMetrics{FuncID: "parser.go#parse", ImportanceScore: 0.3, LinesOfCode: 50}
	if !shouldIncludeFunction(fn, m, 5, DefaultSkeletonFilterConfig()) {
		t.Error("highly-called function should be included")
	}
}

func TestShouldIncludeFunction_NilMetrics(t *testing.T) {
	fn := &store.FunctionDecl{Name: "unknown", Path: "misc.go", LineStart: 1, LineEnd: 30}
	if !shouldIncludeFunction(fn, nil, 2, DefaultSkeletonFilterConfig()) {
		t.Error("function with enough callers should be included without metrics")
	}
}

func TestShouldIncludeFunction_ExactThreshold(t *testing.T) {
	fn := &store.FunctionDecl{Name: "border", Path: "edge.go", LineStart: 1, LineEnd: 5}
	m := &store.FunctionMetrics{FuncID: "edge.go#border", ImportanceScore: 0.05, LinesOfCode: 5}
	if shouldIncludeFunction(fn, m, 0, DefaultSkeletonFilterConfig()) {
		t.Error("at threshold but no callers should be excluded")
	}
}

func TestBuildExploreSkeleton_Format(t *testing.T) {
	idx := store.FileIndex{
		"handler.go": &store.FileEntry{
			Path: "handler.go",
			Functions: []*store.FunctionDecl{
				{Name: "Handle", Signature: "func Handle(w ResponseWriter, r *Request)", LineStart: 10, LineEnd: 40, Exported: true, Path: "handler.go"},
				{Name: "logReq", Signature: "func logReq(r *Request)", LineStart: 42, LineEnd: 44, Path: "handler.go"},
			},
			Imports: []*store.Import{
				{Path: "internal/svc", Internal: true, ResolvedPath: "internal/svc"},
			},
		},
	}
	metricsMap := store.MetricsMap{
		"handler.go#Handle": {FuncID: "handler.go#Handle", IsEntryPoint: true, LinesOfCode: 30, ImportanceScore: 0.8},
		"handler.go#logReq": {FuncID: "handler.go#logReq", LinesOfCode: 2, ImportanceScore: 0.01},
	}
	filter := metrics.NewImportanceFilter(metricsMap, 0)
	result := BuildExploreSkeleton(idx, 5000, filter, DefaultSkeletonFilterConfig())
	if !strings.Contains(result, "[entry]") {
		t.Error("expected [entry] tag")
	}
	if !strings.Contains(result, "Handle") {
		t.Error("expected Handle function")
	}
	if strings.Contains(result, "logReq") {
		t.Error("logReq should be filtered out")
	}
	if !strings.Contains(result, "calls=") || !strings.Contains(result, "called_by=") {
		t.Error("expected calls= and called_by= in header")
	}
}

func TestBuildExploreSkeleton_EmptyIndex(t *testing.T) {
	filter := metrics.NewImportanceFilter(store.MetricsMap{}, 0)
	result := BuildExploreSkeleton(store.FileIndex{}, 5000, filter, DefaultSkeletonFilterConfig())
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildExploreSkeleton_NilFilter(t *testing.T) {
	idx := store.FileIndex{
		"main.go": &store.FileEntry{
			Path:      "main.go",
			Functions: []*store.FunctionDecl{{Name: "main", Signature: "func main()", LineStart: 1, LineEnd: 10, Exported: true, Path: "main.go"}},
		},
	}
	result := BuildExploreSkeleton(idx, 5000, nil, DefaultSkeletonFilterConfig())
	if result == "" {
		t.Error("expected non-empty skeleton")
	}
	if strings.Contains(result, "calls=") {
		t.Error("nil filter should use BuildPlannerSkeleton format")
	}
}

func TestBuildExploreSkeletonIncludesConfirmedEventLandmarksOnlyByDefault(t *testing.T) {
	idx := store.FileIndex{
		"handler.go": &store.FileEntry{
			Path: "handler.go",
			Functions: []*store.FunctionDecl{{
				Name:      "Handle",
				Signature: "func Handle(evt Event)",
				LineStart: 10,
				LineEnd:   40,
				Exported:  true,
				Path:      "handler.go",
				CalledBy:  []*store.CallRef{{Name: "main"}},
				EventFacts: &store.EventFacts{
					Publishes: []*store.EventFact{{EventName: "user.created", Line: 18, Evidence: "bus.Publish(user.created)"}},
					Handles:   []*store.EventFact{{EventName: "user.updated", Line: 22, Evidence: "switch evt.Name"}},
				},
				EventHints: &store.EventHints{
					LikelyPublishes: []*store.EventFact{{EventName: "hint.only", Line: 25, Evidence: "comment hint"}},
				},
			}},
		},
	}
	metricsMap := store.MetricsMap{
		"handler.go#Handle": {FuncID: "handler.go#Handle", LinesOfCode: 30, ImportanceScore: 0.8},
	}
	filter := metrics.NewImportanceFilter(metricsMap, 0)

	result := BuildExploreSkeleton(idx, 5000, filter, DefaultSkeletonFilterConfig())
	if !strings.Contains(result, "user.created") || !strings.Contains(result, "user.updated") {
		t.Fatalf("expected confirmed event landmarks in skeleton:\n%s", result)
	}
	if strings.Contains(result, "hint.only") {
		t.Fatalf("expected hints to be excluded by default:\n%s", result)
	}
}

func TestBuildExploreSkeletonIncludesEventHintsWhenEnabled(t *testing.T) {
	idx := store.FileIndex{
		"handler.go": &store.FileEntry{
			Path: "handler.go",
			Functions: []*store.FunctionDecl{{
				Name:      "Handle",
				Signature: "func Handle(evt Event)",
				LineStart: 10,
				LineEnd:   40,
				Exported:  true,
				Path:      "handler.go",
				CalledBy:  []*store.CallRef{{Name: "main"}},
				EventHints: &store.EventHints{
					LikelyPublishes: []*store.EventFact{{EventName: "hint.only", Line: 25, Evidence: "comment hint"}},
				},
			}},
		},
	}
	metricsMap := store.MetricsMap{
		"handler.go#Handle": {FuncID: "handler.go#Handle", LinesOfCode: 30, ImportanceScore: 0.8},
	}
	filter := metrics.NewImportanceFilter(metricsMap, 0)
	cfg := DefaultSkeletonFilterConfig()
	cfg.IncludeEventHints = true

	result := BuildExploreSkeleton(idx, 5000, filter, cfg)
	if !strings.Contains(result, "hint.only") {
		t.Fatalf("expected hints when enabled:\n%s", result)
	}
}

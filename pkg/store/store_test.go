package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleFileIndex() FileIndex {
	return FileIndex{
		"internal/auth/jwt.go": {
			Language:    "go",
			ContentHash: "sha256:abc",
			Functions: []*FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				LineEnd:   20,
				Exported:  true,
				Path:      "internal/auth/jwt.go",
				EventFacts: &EventFacts{
					Publishes: []*EventFact{{
						EventName: "token.generated",
						FuncID:    "internal/auth/jwt.go#GenerateToken",
						Line:      14,
						Evidence:  "emit(token.generated)",
					}},
					Handles: []*EventFact{{
						EventName:  "token.generated",
						HandlerRef: "internal/auth/jwt.go#GenerateToken",
						FuncID:     "internal/auth/jwt.go#GenerateToken",
						Line:       15,
						Evidence:   "case token.generated:",
					}},
					Registers: []*EventFact{{
						EventName:  "token.generated",
						HandlerRef: "internal/auth/jwt.go#GenerateToken",
						FuncID:     "internal/auth/jwt.go#GenerateToken",
						Line:       15,
						Evidence:   "bus.Register(token.generated, GenerateToken)",
					}},
				},
				EventHints: &EventHints{
					LikelyPublishes: []*EventFact{{
						EventName: "token.refreshed",
						FuncID:    "internal/auth/jwt.go#GenerateToken",
						Line:      17,
						Evidence:  "refresh path emits similar event",
					}},
					LikelyHandles: []*EventFact{{
						EventName:  "audit.token",
						HandlerRef: "internal/auth/jwt.go#GenerateToken",
						FuncID:     "internal/auth/jwt.go#GenerateToken",
						Line:       18,
						Evidence:   "comment mentions audit sink",
					}},
					LikelyRegisters: []*EventFact{{
						EventName:  "audit.token",
						HandlerRef: "internal/auth/jwt.go#GenerateToken",
						FuncID:     "internal/auth/jwt.go#GenerateToken",
						Line:       19,
						Evidence:   "TODO register audit handler",
					}},
				},
			}},
			Types: []*TypeDecl{{
				Name:      "Claims",
				Kind:      "struct",
				LineStart: 1,
				Exported:  true,
			}},
			Imports: []*Import{{Path: "internal/models", Internal: true}},
		},
	}
}

func sampleEventFactIndex() EventFactIndex {
	return EventFactIndex{
		Version:     "epic14/v1",
		GeneratedAt: time.Unix(1710004321, 0).UTC(),
		Events: []*EventEntry{{
			EventName: "token.generated",
			Publishers: []*EventFact{{
				EventName: "token.generated",
				FuncID:    "internal/auth/jwt.go#GenerateToken",
				Line:      14,
				Evidence:  "emit(token.generated)",
			}},
			Handlers: []*EventFact{{
				EventName:  "token.generated",
				HandlerRef: "internal/auth/jwt.go#GenerateToken",
				FuncID:     "internal/auth/jwt.go#GenerateToken",
				Line:       15,
				Evidence:   "case token.generated:",
			}},
			Registrations: []*EventFact{{
				EventName:  "token.generated",
				HandlerRef: "internal/auth/jwt.go#GenerateToken",
				FuncID:     "internal/auth/jwt.go#GenerateToken",
				Line:       15,
				Evidence:   "bus.Register(token.generated, GenerateToken)",
			}},
		}},
	}
}

func sampleDepGraph() DepGraph {
	return DepGraph{
		"internal/auth/jwt.go": {"internal/models/user.go"},
	}
}

func sampleNavPlan() *NavPlan {
	return &NavPlan{
		GeneratedAt: time.Unix(1710000000, 0).UTC(),
		Version:     "planner/v2",
		Navigation: &Navigation{
			Sections: []*NavigationSection{{
				Type:        "generated",
				Title:       "Generated Overview",
				Description: "Top-level generated summary",
				Items: []*NavigationItem{{
					Title:      "Auth entrypoint",
					Path:       "docs/modules/auth.md",
					EntryPoint: "internal/auth/jwt.go#GenerateToken",
					Events:     []string{"token.generated"},
					Highlights: []string{"Issues JWTs"},
				}},
			}},
		},
		Modules: []*Module{{
			ID:              "auth",
			Files:           []string{"internal/auth/jwt.go"},
			Shared:          false,
			Owner:           "agent",
			DependsOnShared: []string{"logger"},
			NavigationRefs:  []string{"generated"},
		}},
	}
}

func sampleSharedContext() SharedContext {
	return SharedContext{
		"logger": {
			Summary:    "Structured logger wrapper",
			KeyTypes:   []string{"Logger"},
			SourceRefs: []string{"pkg/logger/logger.go#L1"},
			KeyFunctions: []*KeyFunction{{
				Name:      "New",
				Signature: "func New() Logger",
				Ref:       "pkg/logger/logger.go#L18",
			}},
		},
	}
}

func sampleValidationReport() *ValidationReport {
	return &ValidationReport{
		GeneratedAt: time.Unix(1710001234, 0).UTC(),
		BrokenLinks: []*BrokenLink{{
			SourceFile: "docs/modules/auth.md",
			LinkText:   "GenerateToken",
			LinkTarget: "internal/auth/jwt.md#generate-token",
			Line:       14,
		}},
		TotalLinks: 9,
		TotalFiles: 3,
	}
}

func TestWriteAndReadFileIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleFileIndex()

	if err := WriteFileIndex(dir, want); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	got, err := ReadFileIndex(dir)
	if err != nil {
		t.Fatalf("ReadFileIndex() error = %v", err)
	}
	if got["internal/auth/jwt.go"].ContentHash != want["internal/auth/jwt.go"].ContentHash {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
	if len(got["internal/auth/jwt.go"].Functions) != 1 {
		t.Fatalf("len(Functions) = %d, want 1", len(got["internal/auth/jwt.go"].Functions))
	}
	if got["internal/auth/jwt.go"].Functions[0].EventFacts == nil {
		t.Fatal("EventFacts = nil, want nested facts")
	}
	if len(got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes) != 1 {
		t.Fatalf("len(EventFacts.Publishes) = %d, want 1", len(got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes))
	}
	if got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].EventName != want["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].EventName {
		t.Fatalf("EventFacts.Publishes[0].EventName = %q, want %q", got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].EventName, want["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].EventName)
	}
	if got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].FuncID != want["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].FuncID {
		t.Fatalf("EventFacts.Publishes[0].FuncID = %q, want %q", got["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].FuncID, want["internal/auth/jwt.go"].Functions[0].EventFacts.Publishes[0].FuncID)
	}
	if got["internal/auth/jwt.go"].Functions[0].EventHints == nil {
		t.Fatal("EventHints = nil, want nested hints")
	}
	if len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyPublishes) != 1 {
		t.Fatalf("len(EventHints.LikelyPublishes) = %d, want 1", len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyPublishes))
	}
	if len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles) != 1 {
		t.Fatalf("len(EventHints.LikelyHandles) = %d, want 1", len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles))
	}
	if got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles[0].HandlerRef != want["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles[0].HandlerRef {
		t.Fatalf("EventHints.LikelyHandles[0].HandlerRef = %q, want %q", got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles[0].HandlerRef, want["internal/auth/jwt.go"].Functions[0].EventHints.LikelyHandles[0].HandlerRef)
	}
	if len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyRegisters) != 1 {
		t.Fatalf("len(EventHints.LikelyRegisters) = %d, want 1", len(got["internal/auth/jwt.go"].Functions[0].EventHints.LikelyRegisters))
	}
}

func TestWriteAndReadEventFactIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleEventFactIndex()

	if err := WriteEventFactIndex(dir, want); err != nil {
		t.Fatalf("WriteEventFactIndex() error = %v", err)
	}
	got, err := ReadEventFactIndex(dir)
	if err != nil {
		t.Fatalf("ReadEventFactIndex() error = %v", err)
	}
	if len(got.Events) != len(want.Events) {
		t.Fatalf("len(Events) = %d, want %d", len(got.Events), len(want.Events))
	}
	if got.Version != want.Version {
		t.Fatalf("Version = %q, want %q", got.Version, want.Version)
	}
	if !got.GeneratedAt.Equal(want.GeneratedAt) {
		t.Fatalf("GeneratedAt = %v, want %v", got.GeneratedAt, want.GeneratedAt)
	}
	if got.Events[0].EventName != want.Events[0].EventName {
		t.Fatalf("Events[0].EventName = %q, want %q", got.Events[0].EventName, want.Events[0].EventName)
	}
	if got.Events[0].Publishers[0].FuncID != want.Events[0].Publishers[0].FuncID {
		t.Fatalf("Events[0].Publishers[0].FuncID = %q, want %q", got.Events[0].Publishers[0].FuncID, want.Events[0].Publishers[0].FuncID)
	}
	if got.Events[0].Handlers[0].FuncID != want.Events[0].Handlers[0].FuncID {
		t.Fatalf("Events[0].Handlers[0].FuncID = %q, want %q", got.Events[0].Handlers[0].FuncID, want.Events[0].Handlers[0].FuncID)
	}
	if got.Events[0].Registrations[0].FuncID != want.Events[0].Registrations[0].FuncID {
		t.Fatalf("Events[0].Registrations[0].FuncID = %q, want %q", got.Events[0].Registrations[0].FuncID, want.Events[0].Registrations[0].FuncID)
	}
}

func TestWriteEventFactIndexUsesSensibleJSONShape(t *testing.T) {
	dir := t.TempDir()
	want := sampleEventFactIndex()

	if err := WriteEventFactIndex(dir, want); err != nil {
		t.Fatalf("WriteEventFactIndex() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "event_fact_index.json"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	events, ok := got["events"].([]any)
	if !ok {
		t.Fatalf("events type = %T, want []any", got["events"])
	}
	if got["version"] != "epic14/v1" {
		t.Fatalf("version = %#v, want %q", got["version"], "epic14/v1")
	}
	if generatedAt, ok := got["generated_at"].(string); !ok || !strings.HasPrefix(generatedAt, "2024-03-09T17:12:01Z") {
		t.Fatalf("generated_at = %#v, want RFC3339 timestamp", got["generated_at"])
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	entry0, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("events[0] type = %T, want map[string]any", events[0])
	}
	if entry0["event_name"] != "token.generated" {
		t.Fatalf("event_name = %#v, want %q", entry0["event_name"], "token.generated")
	}
	if _, exists := entry0["likely_handles"]; exists {
		t.Fatal("likely_handles present in aggregate JSON, want absent")
	}
	if _, exists := entry0["likely_publishes"]; exists {
		t.Fatal("likely_publishes present in aggregate JSON, want absent")
	}
	if _, exists := entry0["likely_registers"]; exists {
		t.Fatal("likely_registers present in aggregate JSON, want absent")
	}
	publishers, ok := entry0["publishers"].([]any)
	if !ok {
		t.Fatalf("publishers type = %T, want []any", entry0["publishers"])
	}
	publisher0, ok := publishers[0].(map[string]any)
	if !ok {
		t.Fatalf("publishers[0] type = %T, want map[string]any", publishers[0])
	}
	if publisher0["func_id"] != "internal/auth/jwt.go#GenerateToken" {
		t.Fatalf("publisher func_id = %#v, want %q", publisher0["func_id"], "internal/auth/jwt.go#GenerateToken")
	}
	handlers, ok := entry0["handlers"].([]any)
	if !ok {
		t.Fatalf("handlers type = %T, want []any", entry0["handlers"])
	}
	handler0, ok := handlers[0].(map[string]any)
	if !ok {
		t.Fatalf("handlers[0] type = %T, want map[string]any", handlers[0])
	}
	if handler0["func_id"] != "internal/auth/jwt.go#GenerateToken" {
		t.Fatalf("handler func_id = %#v, want %q", handler0["func_id"], "internal/auth/jwt.go#GenerateToken")
	}
	registrations, ok := entry0["registrations"].([]any)
	if !ok {
		t.Fatalf("registrations type = %T, want []any", entry0["registrations"])
	}
	registration0, ok := registrations[0].(map[string]any)
	if !ok {
		t.Fatalf("registrations[0] type = %T, want map[string]any", registrations[0])
	}
	if registration0["func_id"] != "internal/auth/jwt.go#GenerateToken" {
		t.Fatalf("registration func_id = %#v, want %q", registration0["func_id"], "internal/auth/jwt.go#GenerateToken")
	}
	if registration0["handler_ref"] != "internal/auth/jwt.go#GenerateToken" {
		t.Fatalf("handler_ref = %#v, want %q", registration0["handler_ref"], "internal/auth/jwt.go#GenerateToken")
	}
}

func TestWriteAndReadDepGraphRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleDepGraph()

	if err := WriteDepGraph(dir, want); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}
	got, err := ReadDepGraph(dir)
	if err != nil {
		t.Fatalf("ReadDepGraph() error = %v", err)
	}
	if len(got["internal/auth/jwt.go"]) != 1 || got["internal/auth/jwt.go"][0] != "internal/models/user.go" {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteAndReadNavPlanRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleNavPlan()

	if err := WriteNavPlan(dir, want); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	got, err := ReadNavPlan(dir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}
	if got.Modules[0].ID != want.Modules[0].ID {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteAndReadNavPlanRoundTripPreservesNavigationSections(t *testing.T) {
	dir := t.TempDir()
	want := sampleNavPlan()

	if err := WriteNavPlan(dir, want); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}

	got, err := ReadNavPlan(dir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}

	if got.Version != want.Version {
		t.Fatalf("Version = %q, want %q", got.Version, want.Version)
	}
	if got.Navigation == nil {
		t.Fatal("Navigation = nil, want populated navigation")
	}
	if len(got.Navigation.Sections) != 1 {
		t.Fatalf("len(Navigation.Sections) = %d, want 1", len(got.Navigation.Sections))
	}
	if got.Navigation.Sections[0].Type != want.Navigation.Sections[0].Type {
		t.Fatalf("Navigation.Sections[0].Type = %q, want %q", got.Navigation.Sections[0].Type, want.Navigation.Sections[0].Type)
	}
	if got.Navigation.Sections[0].Items[0].EntryPoint != want.Navigation.Sections[0].Items[0].EntryPoint {
		t.Fatalf("Navigation.Sections[0].Items[0].EntryPoint = %q, want %q", got.Navigation.Sections[0].Items[0].EntryPoint, want.Navigation.Sections[0].Items[0].EntryPoint)
	}
	if len(got.Modules[0].NavigationRefs) != 1 || got.Modules[0].NavigationRefs[0] != "generated" {
		t.Fatalf("Modules[0].NavigationRefs = %#v, want [generated]", got.Modules[0].NavigationRefs)
	}
}

func TestReadLegacyNavPlanWithoutNavigationStillSucceeds(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"generated_at":"2024-03-09T16:00:00Z","modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]}]}`)

	if err := os.WriteFile(filepath.Join(dir, "nav_plan.json"), legacy, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	got, err := ReadNavPlan(dir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}

	if got.Navigation != nil {
		t.Fatalf("Navigation = %#v, want nil for legacy nav plan", got.Navigation)
	}
	if got.Version != "" {
		t.Fatalf("Version = %q, want empty for legacy nav plan", got.Version)
	}
	if len(got.Modules) != 1 || got.Modules[0].ID != "auth" {
		t.Fatalf("Modules = %#v, want one auth module", got.Modules)
	}
}

func TestWriteNavPlanUsesSensibleNavigationJSONShape(t *testing.T) {
	dir := t.TempDir()
	want := sampleNavPlan()

	if err := WriteNavPlan(dir, want); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "nav_plan.json"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got["version"] != "planner/v2" {
		t.Fatalf("version = %#v, want %q", got["version"], "planner/v2")
	}
	navigation, ok := got["navigation"].(map[string]any)
	if !ok {
		t.Fatalf("navigation type = %T, want map[string]any", got["navigation"])
	}
	sections, ok := navigation["sections"].([]any)
	if !ok || len(sections) != 1 {
		t.Fatalf("navigation.sections = %#v, want one section", navigation["sections"])
	}
	section0, ok := sections[0].(map[string]any)
	if !ok {
		t.Fatalf("sections[0] type = %T, want map[string]any", sections[0])
	}
	if section0["type"] != "generated" {
		t.Fatalf("sections[0].type = %#v, want %q", section0["type"], "generated")
	}
	items, ok := section0["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("sections[0].items = %#v, want one item", section0["items"])
	}
	modules, ok := got["modules"].([]any)
	if !ok || len(modules) != 1 {
		t.Fatalf("modules = %#v, want one module", got["modules"])
	}
	module0, ok := modules[0].(map[string]any)
	if !ok {
		t.Fatalf("modules[0] type = %T, want map[string]any", modules[0])
	}
	navigationRefs, ok := module0["navigation_refs"].([]any)
	if !ok || len(navigationRefs) != 1 || navigationRefs[0] != "generated" {
		t.Fatalf("modules[0].navigation_refs = %#v, want [generated]", module0["navigation_refs"])
	}
}

func TestWriteAndReadSharedContextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleSharedContext()

	if err := WriteSharedContext(dir, want); err != nil {
		t.Fatalf("WriteSharedContext() error = %v", err)
	}
	got, err := ReadSharedContext(dir)
	if err != nil {
		t.Fatalf("ReadSharedContext() error = %v", err)
	}
	if got["logger"].Summary != want["logger"].Summary {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestWriteValidationReportRoundTripsJSON(t *testing.T) {
	dir := t.TempDir()
	want := sampleValidationReport()

	if err := WriteValidationReport(dir, want); err != nil {
		t.Fatalf("WriteValidationReport() error = %v", err)
	}

	path := filepath.Join(dir, "validation_report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var got ValidationReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.TotalLinks != want.TotalLinks {
		t.Fatalf("TotalLinks = %d, want %d", got.TotalLinks, want.TotalLinks)
	}
	if got.TotalFiles != want.TotalFiles {
		t.Fatalf("TotalFiles = %d, want %d", got.TotalFiles, want.TotalFiles)
	}
	if len(got.BrokenLinks) != 1 {
		t.Fatalf("len(BrokenLinks) = %d, want 1", len(got.BrokenLinks))
	}
	if got.BrokenLinks[0].LinkTarget != want.BrokenLinks[0].LinkTarget {
		t.Fatalf("LinkTarget = %q, want %q", got.BrokenLinks[0].LinkTarget, want.BrokenLinks[0].LinkTarget)
	}
}

func TestReadReturnsErrArtifactNotFound(t *testing.T) {
	_, err := ReadFileIndex(t.TempDir())
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("error = %v, want ErrArtifactNotFound", err)
	}
}

func TestFuncID(t *testing.T) {
	tests := []struct {
		name string
		fn   *FunctionDecl
		want string
	}{
		{
			name: "regular function",
			fn:   &FunctionDecl{Path: "pkg/foo.go", Name: "Bar"},
			want: "pkg/foo.go#Bar",
		},
		{
			name: "method",
			fn:   &FunctionDecl{Path: "pkg/foo.go", Name: "Bar", Receiver: "MyType"},
			want: "pkg/foo.go#MyType#Bar",
		},
		{
			name: "nil",
			fn:   nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FuncID(tt.fn); got != tt.want {
				t.Fatalf("FuncID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteAndReadMetricsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := MetricsMap{
		"pkg/foo.go#Bar": {
			FuncID:              "pkg/foo.go#Bar",
			OutDegree:           3,
			DepthFromEntryPoint: 1,
			ReachableFromEntry:  5,
			IsExported:          true,
			IsEntryPoint:        false,
			LinesOfCode:         42,
			ImportanceScore:     0.75,
		},
	}

	if err := WriteMetrics(dir, want); err != nil {
		t.Fatalf("WriteMetrics() error = %v", err)
	}
	got, err := ReadMetrics(dir)
	if err != nil {
		t.Fatalf("ReadMetrics() error = %v", err)
	}
	if got["pkg/foo.go#Bar"].FuncID != want["pkg/foo.go#Bar"].FuncID {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
	if got["pkg/foo.go#Bar"].ImportanceScore != want["pkg/foo.go#Bar"].ImportanceScore {
		t.Fatalf("ImportanceScore = %f, want %f", got["pkg/foo.go#Bar"].ImportanceScore, want["pkg/foo.go#Bar"].ImportanceScore)
	}
}

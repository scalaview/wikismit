package planner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestPlannerRoundRequestJSONShape(t *testing.T) {
	t.Helper()

	params := json.RawMessage(`{"function_ref":"svc/service.go#HandleRequest"}`)
	navigation := json.RawMessage(`{"modules":[{"id":"auth"}]}`)
	request := &PlannerRoundRequest{
		Round:         2,
		Understanding: "Need call graph data before final modules",
		Requests: []*PlannerRequest{{
			Type:   "call_chain",
			Params: params,
		}},
		Navigation: &navigation,
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{"round", "understanding", "requests", "navigation"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("marshaled PlannerRoundRequest missing key %q: %s", key, raw)
		}
	}
	if _, ok := got["previousResponses"]; ok {
		t.Fatalf("marshaled PlannerRoundRequest used unexpected camelCase key: %s", raw)
	}
	requests, ok := got["requests"].([]any)
	if !ok || len(requests) != 1 {
		t.Fatalf("marshaled PlannerRoundRequest requests = %#v, want single request", got["requests"])
	}
	first, ok := requests[0].(map[string]any)
	if !ok {
		t.Fatalf("marshaled PlannerRoundRequest request entry = %#v, want object", requests[0])
	}
	if first["type"] != "call_chain" {
		t.Fatalf("marshaled PlannerRoundRequest request type = %#v, want call_chain", first["type"])
	}
	if _, ok := first["params"]; !ok {
		t.Fatalf("marshaled PlannerRoundRequest request missing params: %s", raw)
	}
	if _, ok := got["navigation"].(map[string]any); !ok {
		t.Fatalf("marshaled PlannerRoundRequest navigation = %#v, want object", got["navigation"])
	}
}

func TestRunInteractivePlannerRoutesReadRequestsAndQueryRequestsAcrossRounds(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(
		`{"round":1,"understanding":"Need entry point details first","requests":[{"type":"read_file","params":{"target":"svc/handler.go"}},{"type":"read_function","params":{"target":"svc/service.go#Service#Process"}}]}`,
		`{"round":2,"understanding":"Need graph traversals","requests":[{"type":"call_chain","params":{"function_ref":"svc/handler.go#HandleRequest","direction":"downstream","depth":2}},{"type":"event_flow","params":{"event_name":"user.created","expand_handlers":true,"handler_depth":1}}]}`,
		`{"round":3,"navigation":{"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent"}]}}`,
	)

	got, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err != nil {
		t.Fatalf("RunInteractivePlanner() error = %v", err)
	}
	if got == nil || len(got.Modules) != 1 {
		t.Fatalf("RunInteractivePlanner() modules = %#v, want one module", got)
	}
	if got.Modules[0].ID != "svc" {
		t.Fatalf("RunInteractivePlanner() module ID = %q, want svc", got.Modules[0].ID)
	}
	if client.CallCount() != 3 {
		t.Fatalf("MockClient.CallCount() = %d, want 3", client.CallCount())
	}
	if got.GeneratedAt.IsZero() {
		t.Fatal("RunInteractivePlanner() GeneratedAt is zero, want non-zero time")
	}
}

func TestRunInteractivePlannerRejectsUnknownRequestType(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"requests":[{"type":"rag_search","params":{"query":"user.created"}}]}`)

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want unknown request type error")
	}
	if !strings.Contains(err.Error(), "unknown request type") {
		t.Fatalf("RunInteractivePlanner() error = %v, want unknown request type context", err)
	}
}

func TestRunInteractivePlannerLimitsRequestsPerRound(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	cfg.Analysis.InteractivePlanner.MaxRequestsPerRound = 2
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(
		`{"round":1,"requests":[{"type":"read_file","params":{"target":"svc/handler.go"}},{"type":"read_file","params":{"target":"svc/service.go"}},{"type":"read_function","params":{"target":"svc/service.go#Service#Process"}}]}`,
		`{"round":2,"navigation":{"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent"}]}}`,
	)

	got, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err != nil {
		t.Fatalf("RunInteractivePlanner() error = %v", err)
	}
	if got == nil || len(got.Modules) != 1 {
		t.Fatalf("RunInteractivePlanner() modules = %#v, want one module", got)
	}
	if client.CallCount() != 2 {
		t.Fatalf("MockClient.CallCount() = %d, want 2", client.CallCount())
	}
}

func TestBuildInteractivePlannerPromptIncludesToolSchemaAndRoundState(t *testing.T) {
	t.Helper()

	contextState := &PlannerRoundContext{
		Round:              2,
		Skeleton:           "// svc/handler.go\nfunc HandleRequest() error",
		ExplorationContext: "Need call graph evidence before assigning modules",
		PreviousResponses: []*PlannerResponseEnvelope{{
			Type:   "read_file",
			Result: &plannerReadFileResult{Target: "svc/handler.go", Content: "func HandleRequest() error { return nil }"},
		}},
	}

	got := buildInteractivePlannerPrompt(contextState, 3)

	for _, want := range []string{
		"Round: 2",
		"Need call graph evidence before assigning modules",
		"read_file",
		"read_function",
		"call_chain",
		"event_flow",
		"modules",
		"owner",
		"Previous responses:",
		"svc/handler.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInteractivePlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestBuildInteractivePlannerPromptUsesReceiverAwareMethodExamples(t *testing.T) {
	contextState := &PlannerRoundContext{Round: 1, Skeleton: "// svc/service.go"}

	got := buildInteractivePlannerPrompt(contextState, 3)

	for _, want := range []string{
		`read_function {"target":"path/to/file.go#Receiver#Method"}`,
		`call_chain {"function_ref":"path/to/file.go#Receiver#Method","direction":"downstream|upstream","depth":N,"include_events":bool}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInteractivePlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestRunInteractivePlannerFailsFastWhenCallGraphArtifactMissing(t *testing.T) {
	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, _, eventIdx := buildInteractivePlannerFixtures(t)
	if err := store.WriteEventFactIndex(cfg.ArtifactsDir, eventIdx); err != nil {
		t.Fatalf("store.WriteEventFactIndex() error = %v", err)
	}

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, llm.NewMockClient())
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want missing call graph error")
	}
	if !strings.Contains(err.Error(), "read call graph") {
		t.Fatalf("RunInteractivePlanner() error = %v, want read call graph context", err)
	}
}

func TestRunInteractivePlannerFailsFastWhenEventFactIndexArtifactMissing(t *testing.T) {
	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, _ := buildInteractivePlannerFixtures(t)
	if err := store.WriteCallGraph(cfg.ArtifactsDir, callGraph); err != nil {
		t.Fatalf("store.WriteCallGraph() error = %v", err)
	}

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, llm.NewMockClient())
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want missing event fact index error")
	}
	if !strings.Contains(err.Error(), "read event fact index") {
		t.Fatalf("RunInteractivePlanner() error = %v, want read event fact index context", err)
	}
}

func TestRunInteractivePlannerStopsAfterMaxRounds(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	cfg.Analysis.InteractivePlanner.MaxRounds = 2
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(
		`{"round":1,"requests":[{"type":"read_file","params":{"target":"svc/handler.go"}}]}`,
		`{"round":2,"requests":[{"type":"read_function","params":{"target":"svc/service.go#Service#Process"}}]}`,
	)

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want max rounds error")
	}
	if !strings.Contains(err.Error(), "max rounds") {
		t.Fatalf("RunInteractivePlanner() error = %v, want max rounds context", err)
	}
	if client.CallCount() != 2 {
		t.Fatalf("MockClient.CallCount() = %d, want 2", client.CallCount())
	}
}

func TestRunInteractivePlannerStopsWhenNavigationReturned(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"navigation":{"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent"}]}}`)

	got, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err != nil {
		t.Fatalf("RunInteractivePlanner() error = %v", err)
	}
	if got == nil || len(got.Modules) != 1 {
		t.Fatalf("RunInteractivePlanner() modules = %#v, want one module", got)
	}
	if client.CallCount() != 1 {
		t.Fatalf("MockClient.CallCount() = %d, want 1", client.CallCount())
	}
}

func sampleInteractivePlannerConfig(t *testing.T) *configpkg.Config {
	t.Helper()

	cfg := samplePlannerConfig(t)
	cfg.Analysis.InteractivePlanner = &configpkg.InteractivePlannerConfig{
		Enabled:             true,
		MaxRounds:           4,
		MaxRequestsPerRound: 5,
	}
	return cfg
}

func buildInteractivePlannerFixtures(t *testing.T) (store.FileIndex, store.DepGraph, store.CallGraph, store.EventFactIndex) {
	t.Helper()

	idx := store.FileIndex{
		"svc/handler.go": {
			Path:     "svc/handler.go",
			Language: "go",
			Functions: []*store.FunctionDecl{{
				Name:      "HandleRequest",
				Signature: "func HandleRequest() error",
				LineStart: 10,
				LineEnd:   24,
				Exported:  true,
				Path:      "svc/handler.go",
				Src:       "func HandleRequest() error { return Process() }",
			}},
		},
		"svc/service.go": {
			Path:     "svc/service.go",
			Language: "go",
			Functions: []*store.FunctionDecl{{
				Name:         "Process",
				Receiver:     "Service",
				FunctionType: store.FunctionTypeMethod,
				Signature:    "func (s Service) Process() error",
				LineStart:    12,
				LineEnd:      30,
				Exported:     true,
				Path:         "svc/service.go",
				Src:          "func (s Service) Process() error { publishUserCreated(); return nil }",
			}},
		},
		"svc/events.go": {
			Path:     "svc/events.go",
			Language: "go",
			Functions: []*store.FunctionDecl{{
				Name:      "publishUserCreated",
				Signature: "func publishUserCreated()",
				LineStart: 8,
				LineEnd:   16,
				Path:      "svc/events.go",
				Src:       "func publishUserCreated() { bus.Publish(\"user.created\") }",
			}},
		},
	}

	depGraph := store.DepGraph{
		"svc/handler.go": {"svc/service.go"},
		"svc/service.go": {"svc/events.go"},
		"svc/events.go":  {},
	}

	callGraph := store.CallGraph{
		"svc/handler.go#HandleRequest": {"svc/service.go#Process"},
		"svc/service.go#Process":       {"svc/events.go#publishUserCreated"},
	}

	eventIdx := store.EventFactIndex{
		Version: "epic14/v1",
		Events: []*store.EventEntry{{
			EventName: "user.created",
			Publishers: []*store.EventFact{{
				EventName: "user.created",
				FuncID:    "svc/events.go#publishUserCreated",
				Line:      10,
				Evidence:  "bus.Publish(\"user.created\")",
			}},
			Handlers: []*store.EventFact{{
				EventName:  "user.created",
				HandlerRef: "svc/service.go#Service#Process",
				FuncID:     "svc/service.go#Service#Process",
				Line:       12,
				Evidence:   "case user.created:",
			}},
		}},
	}

	return idx, depGraph, callGraph, eventIdx
}

func writeInteractivePlannerArtifacts(t *testing.T, cfg *configpkg.Config, callGraph store.CallGraph, eventIdx store.EventFactIndex) {
	t.Helper()

	if err := store.WriteCallGraph(cfg.ArtifactsDir, callGraph); err != nil {
		t.Fatalf("store.WriteCallGraph() error = %v", err)
	}
	if err := store.WriteEventFactIndex(cfg.ArtifactsDir, eventIdx); err != nil {
		t.Fatalf("store.WriteEventFactIndex() error = %v", err)
	}
}

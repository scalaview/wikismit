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

func TestPlannerRoundRequestJSONShapeUsesTopLevelModulesAndNavigation(t *testing.T) {
	t.Helper()

	params := json.RawMessage(`{"function_ref":"svc/service.go#HandleRequest"}`)
	modules := json.RawMessage(`[{"id":"auth","files":["svc/service.go"],"shared":false,"owner":"agent"}]`)
	navigation := json.RawMessage(`{"sections":[{"type":"generated","title":"Auth"}]}`)
	request := &PlannerRoundRequest{
		Round:         2,
		Understanding: "Need call graph data before final modules",
		Requests: []*PlannerRequest{{
			Type:   "call_chain",
			Params: params,
		}},
		Modules:    modules,
		Navigation: navigation,
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{"round", "understanding", "requests", "modules", "navigation"} {
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
	if _, ok := got["modules"].([]any); !ok {
		t.Fatalf("marshaled PlannerRoundRequest modules = %#v, want array", got["modules"])
	}
	if _, ok := got["navigation"].(map[string]any); !ok {
		t.Fatalf("marshaled PlannerRoundRequest navigation = %#v, want object", got["navigation"])
	}
}

func TestPlannerRoundRequestAllowsModulesOnlyTerminalPayload(t *testing.T) {
	t.Helper()

	modules := json.RawMessage(`[{"id":"svc","files":["svc/handler.go"],"shared":false,"owner":"agent"}]`)
	request := &PlannerRoundRequest{
		Round:   3,
		Modules: modules,
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := got["modules"].([]any); !ok {
		t.Fatalf("marshaled PlannerRoundRequest modules = %#v, want array", got["modules"])
	}
	if _, ok := got["navigation"]; ok {
		t.Fatalf("marshaled PlannerRoundRequest unexpectedly included navigation: %s", raw)
	}
	if _, ok := got["requests"]; ok {
		t.Fatalf("marshaled PlannerRoundRequest unexpectedly included requests: %s", raw)
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
		`{"round":3,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent","navigation_refs":["generated"]}],"navigation":{"sections":[{"type":"generated","title":"Service Overview","items":[{"title":"Handle Request","path":"docs/modules/svc.md","entry_point":"svc/handler.go#HandleRequest"}]}]}}`,
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
	if got.Navigation == nil || len(got.Navigation.Sections) != 1 {
		t.Fatalf("RunInteractivePlanner() navigation = %#v, want one section", got.Navigation)
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
		`{"round":2,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent"}]}`,
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
		Skeleton:           "FILE: svc/handler.go\nQUERYABLE_FUNCTIONS\n  svc/handler.go#HandleRequest",
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
		"QUERYABLE_FUNCTIONS",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInteractivePlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestBuildInteractivePlannerPromptStatesOnlyQueryableFunctionsAreCallable(t *testing.T) {
	contextState := &PlannerRoundContext{Round: 1, Skeleton: "FILE: svc/service.go\nQUERYABLE_FUNCTIONS\n  svc/service.go#HandleRequest\nTYPES\n  Service"}

	got := buildInteractivePlannerPrompt(contextState, 3)

	for _, want := range []string{
		"Use read_file only with file paths listed in FILE headers.",
		"Use read_function and call_chain only with exact refs listed under QUERYABLE_FUNCTIONS.",
		"Names under TYPES, INTERNAL_IMPORTS, and EVENT_LANDMARKS are informational only and are not valid function refs.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInteractivePlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestBuildInteractivePlannerPromptUsesReceiverAwareMethodExamples(t *testing.T) {
	contextState := &PlannerRoundContext{Round: 1, Skeleton: "FILE: svc/service.go"}

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

func TestBuildInteractivePlannerPromptUsesTopLevelModulesAndNavigationSchema(t *testing.T) {
	contextState := &PlannerRoundContext{Round: 2, Skeleton: "FILE: svc/handler.go"}

	got := buildInteractivePlannerPrompt(contextState, 3)

	for _, want := range []string{
		`{"round":N,"understanding":"...","requests":[...]}`,
		`{"round":N,"modules":[...],"navigation":{"sections":[...]}}`,
		`"modules":[{"id":"module","files":["path/to/file.go"]`,
		`"navigation":{"sections":[{"type":"generated"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInteractivePlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestBuildInteractivePlannerPromptDoesNotNestModulesUnderNavigation(t *testing.T) {
	contextState := &PlannerRoundContext{Round: 2, Skeleton: "FILE: svc/handler.go"}

	got := buildInteractivePlannerPrompt(contextState, 3)

	for _, unwanted := range []string{
		`{"round":N,"navigation":{"modules":[...]}}`,
		`Final navigation should include version`,
		`Final navigation must stay modules-compatible for now.`,
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildInteractivePlannerPrompt() unexpectedly contained %q:\n%s", unwanted, got)
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

func TestRunInteractivePlannerStopsWhenModulesAndNavigationReturned(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent","navigation_refs":["generated"]}],"navigation":{"sections":[{"type":"generated","title":"Service Overview","items":[{"title":"Handle Request","path":"docs/modules/svc.md","entry_point":"svc/handler.go#HandleRequest"}]}]}}`)

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
	if got.Navigation == nil || len(got.Navigation.Sections) != 1 {
		t.Fatalf("RunInteractivePlanner() navigation = %#v, want one section", got.Navigation)
	}
}

func TestRunInteractivePlannerAcceptsModulesOnlyTerminalPayload(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent"}]}`)

	got, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err != nil {
		t.Fatalf("RunInteractivePlanner() error = %v", err)
	}
	if got == nil || len(got.Modules) != 1 {
		t.Fatalf("RunInteractivePlanner() modules = %#v, want one module", got)
	}
	if got.Navigation != nil {
		t.Fatalf("RunInteractivePlanner() navigation = %#v, want nil", got.Navigation)
	}
}

func TestRunInteractivePlannerRejectsNavigationOnlyTerminalPayload(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"navigation":{"sections":[{"type":"generated","title":"Service Overview"}]}}`)

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want missing modules error")
	}
	if !strings.Contains(err.Error(), "interactive terminal payload missing modules") {
		t.Fatalf("RunInteractivePlanner() error = %v, want missing modules context", err)
	}
}

func TestRunInteractivePlannerReportsModuleValidationAfterAssembly(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go"],"shared":false,"owner":"agent","navigation_refs":["generated"]}],"navigation":{"sections":[{"type":"generated","title":"Service Overview"}]}}`)

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "missing file assignment") {
		t.Fatalf("RunInteractivePlanner() error = %v, want missing file assignment context", err)
	}
}

func TestRunInteractivePlannerSetsVersionOnAssembledPlan(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent","navigation_refs":["generated"]}],"navigation":{"sections":[{"type":"generated","title":"Service Overview"}]}}`)

	got, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err != nil {
		t.Fatalf("RunInteractivePlanner() error = %v", err)
	}
	if got.Version != plannerNavPlanVersion {
		t.Fatalf("RunInteractivePlanner() Version = %q, want %q", got.Version, plannerNavPlanVersion)
	}
}

func TestRunInteractivePlannerPreservesNavigationRefsValidation(t *testing.T) {
	t.Helper()

	cfg := sampleInteractivePlannerConfig(t)
	idx, depGraph, callGraph, eventIdx := buildInteractivePlannerFixtures(t)
	writeInteractivePlannerArtifacts(t, cfg, callGraph, eventIdx)

	client := llm.NewMockClient(`{"round":1,"modules":[{"id":"svc","files":["svc/handler.go","svc/service.go","svc/events.go"],"shared":false,"owner":"agent","navigation_refs":["api"]}],"navigation":{"sections":[{"type":"generated","title":"Service Overview"}]}}`)

	_, err := RunInteractivePlanner(context.Background(), idx, depGraph, cfg, client)
	if err == nil {
		t.Fatal("RunInteractivePlanner() error = nil, want navigation ref validation error")
	}
	if !strings.Contains(err.Error(), "unknown navigation ref") {
		t.Fatalf("RunInteractivePlanner() error = %v, want unknown navigation ref context", err)
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

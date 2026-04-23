package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	promptpkg "github.com/scalaview/wikismit/internal/agent/prompt"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestFunctionSummaryAgentRunReturnsNilForEmptyIndex(t *testing.T) {
	agent := NewFunctionSummaryAgent(nil, nil)

	for _, tc := range []struct {
		name string
		idx  store.FileIndex
	}{
		{name: "nil", idx: nil},
		{name: "empty", idx: store.FileIndex{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := agent.Run(context.Background(), tc.idx, nil); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
		})
	}
}

func TestFunctionSummaryAgentRunReturnsNilWhenEverythingAlreadySummarized(t *testing.T) {
	agent := NewFunctionSummaryAgent(nil, nil)
	idx := store.FileIndex{
		"internal/auth/service.go": {
			Path: "internal/auth/service.go",
			Functions: []*store.FunctionDecl{
				{
					Name:    "HandleRequest",
					Path:    "internal/auth/service.go",
					Src:     "func HandleRequest() error {\n\treturn nil\n}",
					Summary: "internal/auth/service.go#HandleRequest\nSummary: Handles the incoming request and returns any error from the auth flow.",
				},
				{
					Name: "GeneratedStub",
					Path: "internal/auth/service.go",
					Src:  "",
				},
			},
		},
	}

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestFunctionSummaryAgentRunUsesFreshStatePerInvocation(t *testing.T) {
	agent := NewFunctionSummaryAgent(nil, nil)
	const path = "internal/auth/service.go"

	if err := agent.Run(context.Background(), store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{{
				Name:    "HandleRequest",
				Path:    path,
				Src:     "func HandleRequest() error {\n\treturn nil\n}",
				Summary: path + "#HandleRequest\nSummary: Handles the incoming request.",
			}},
		},
	}, nil); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}

	err := agent.Run(context.Background(), store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{{
				Name: "HandleRequest",
				Path: path,
				Src:  "func HandleRequest() error {\n\treturn nil\n}",
			}},
		},
	}, nil)
	if err == nil {
		t.Fatal("second Run() error = nil, want unusable runtime error")
	}
	if !strings.Contains(err.Error(), "function summary agent unusable") {
		t.Fatalf("second Run() error = %q, want unusable runtime error", err.Error())
	}
}

func TestFunctionSummaryDepGraph(t *testing.T) {
	const path = "internal/auth/service.go"
	const helperPath = "internal/auth/session.go"

	tests := []struct {
		name                 string
		idx                  store.FileIndex
		wantInitialReady     []FuncSign
		wantInitialRemaining []FuncSign
		resolves             []FuncSign
		wantFinalReady       []FuncSign
		wantFinalRemaining   []FuncSign
	}{
		{
			name: "chain",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn persistSession()\n}", "", functionSummaryTestInternalCall(path, "persistSession")),
						functionSummaryTestFunction(path, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
					},
				},
			},
			wantInitialReady:     []FuncSign{FuncSign(path + "#persistSession")},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#HandleRequest"), FuncSign(path + "#persistSession")},
			resolves:             []FuncSign{FuncSign(path + "#persistSession")},
			wantFinalReady:       []FuncSign{FuncSign(path + "#HandleRequest")},
			wantFinalRemaining:   []FuncSign{FuncSign(path + "#HandleRequest")},
		},
		{
			name: "shared helper",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleLogin", "func HandleLogin() error {\n\treturn persistSession()\n}", "", functionSummaryTestInternalCall(helperPath, "persistSession")),
						functionSummaryTestFunction(path, "HandleRefresh", "func HandleRefresh() error {\n\treturn persistSession()\n}", "", functionSummaryTestInternalCall(helperPath, "persistSession")),
					},
				},
				helperPath: {
					Path: helperPath,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
					},
				},
			},
			wantInitialReady:     []FuncSign{FuncSign(helperPath + "#persistSession")},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#HandleLogin"), FuncSign(path + "#HandleRefresh"), FuncSign(helperPath + "#persistSession")},
			resolves:             []FuncSign{FuncSign(helperPath + "#persistSession")},
			wantFinalReady:       []FuncSign{FuncSign(path + "#HandleLogin"), FuncSign(path + "#HandleRefresh")},
			wantFinalRemaining:   []FuncSign{FuncSign(path + "#HandleLogin"), FuncSign(path + "#HandleRefresh")},
		},
		{
			name: "package-qualified free function uses resolved target",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn sessionpkg.persistSession()\n}", "", func() *store.CallRef {
							call := functionSummaryTestInternalMethodCall(path, "sessionpkg", "persistSession")
							call.ResolvedTarget = helperPath + "#persistSession"
							return call
						}()),
					},
				},
				helperPath: {
					Path: helperPath,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
					},
				},
			},
			wantInitialReady:     []FuncSign{FuncSign(helperPath + "#persistSession")},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#HandleRequest"), FuncSign(helperPath + "#persistSession")},
			resolves:             []FuncSign{FuncSign(helperPath + "#persistSession")},
			wantFinalReady:       []FuncSign{FuncSign(path + "#HandleRequest")},
			wantFinalRemaining:   []FuncSign{FuncSign(path + "#HandleRequest")},
		},
		{
			name: "same-file function and method collision",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleFunction", "func HandleFunction() error {\n\treturn persistSession()\n}", "", functionSummaryTestInternalCall(path, "persistSession")),
						functionSummaryTestMethod(path, "sessionService", "HandleMethod", "func (s sessionService) HandleMethod() error {\n\treturn s.persistSession()\n}", "", functionSummaryTestInternalMethodCall(path, "sessionService", "persistSession")),
						functionSummaryTestFunction(path, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
						functionSummaryTestMethod(path, "sessionService", "persistSession", "func (s sessionService) persistSession() error {\n\treturn nil\n}", ""),
					},
				},
			},
			wantInitialReady: []FuncSign{
				FuncSign(path + "#persistSession"),
				FuncSign(path + "#sessionService#HandleMethod"),
				FuncSign(path + "#sessionService#persistSession"),
			},
			wantInitialRemaining: []FuncSign{
				FuncSign(path + "#HandleFunction"),
				FuncSign(path + "#persistSession"),
				FuncSign(path + "#sessionService#HandleMethod"),
				FuncSign(path + "#sessionService#persistSession"),
			},
			resolves: []FuncSign{FuncSign(path + "#persistSession")},
			wantFinalReady: []FuncSign{
				FuncSign(path + "#HandleFunction"),
				FuncSign(path + "#sessionService#HandleMethod"),
				FuncSign(path + "#sessionService#persistSession"),
			},
			wantFinalRemaining: []FuncSign{
				FuncSign(path + "#HandleFunction"),
				FuncSign(path + "#sessionService#HandleMethod"),
				FuncSign(path + "#sessionService#persistSession"),
			},
		},
		{
			name: "preexisting callee summary",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn persistSession()\n}", "", functionSummaryTestInternalCall(helperPath, "persistSession")),
					},
				},
				helperPath: {
					Path: helperPath,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", helperPath+"#persistSession\nSummary: Persists the session before returning any storage error."),
					},
				},
			},
			wantInitialReady:     []FuncSign{FuncSign(path + "#HandleRequest")},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#HandleRequest")},
			resolves:             []FuncSign{FuncSign(path + "#HandleRequest")},
			wantFinalReady:       []FuncSign{},
			wantFinalRemaining:   []FuncSign{},
		},
		{
			name: "empty source exclusion",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn generatedStub()\n}", "", functionSummaryTestInternalCall(path, "generatedStub")),
						functionSummaryTestFunction(path, "generatedStub", "", ""),
					},
				},
			},
			wantInitialReady:     []FuncSign{FuncSign(path + "#HandleRequest")},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#HandleRequest")},
			resolves:             []FuncSign{FuncSign(path + "#HandleRequest")},
			wantFinalReady:       []FuncSign{},
			wantFinalRemaining:   []FuncSign{},
		},
		{
			name: "simple cycle",
			idx: store.FileIndex{
				path: {
					Path: path,
					Functions: []*store.FunctionDecl{
						functionSummaryTestFunction(path, "stepA", "func stepA() error {\n\treturn stepB()\n}", "", functionSummaryTestInternalCall(path, "stepB")),
						functionSummaryTestFunction(path, "stepB", "func stepB() error {\n\treturn stepA()\n}", "", functionSummaryTestInternalCall(path, "stepA")),
					},
				},
			},
			wantInitialReady:     []FuncSign{},
			wantInitialRemaining: []FuncSign{FuncSign(path + "#stepA"), FuncSign(path + "#stepB")},
			resolves:             []FuncSign{FuncSign(path + "#stepA")},
			wantFinalReady:       []FuncSign{FuncSign(path + "#stepB")},
			wantFinalRemaining:   []FuncSign{FuncSign(path + "#stepB")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			graph := newDepGraph(tc.idx)

			if got := functionSummaryTestSigns(graph.ready()); !reflect.DeepEqual(got, tc.wantInitialReady) {
				t.Fatalf("ready() = %v, want %v", got, tc.wantInitialReady)
			}
			if got := functionSummaryTestSigns(graph.remaining()); !reflect.DeepEqual(got, tc.wantInitialRemaining) {
				t.Fatalf("remaining() = %v, want %v", got, tc.wantInitialRemaining)
			}

			for _, sign := range tc.resolves {
				graph.resolve(sign)
			}

			if got := functionSummaryTestSigns(graph.ready()); !reflect.DeepEqual(got, tc.wantFinalReady) {
				t.Fatalf("ready() after resolve = %v, want %v", got, tc.wantFinalReady)
			}
			if got := functionSummaryTestSigns(graph.remaining()); !reflect.DeepEqual(got, tc.wantFinalRemaining) {
				t.Fatalf("remaining() after resolve = %v, want %v", got, tc.wantFinalRemaining)
			}
		})
	}
}

func TestFunctionSummaryResolvedInternalCallKeyFailsClosedOnMethodAmbiguity(t *testing.T) {
	const path = "internal/auth/service.go"

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
				functionSummaryTestMethod(path, "sessionService", "persistSession", "func (s sessionService) persistSession() error {\n\treturn nil\n}", ""),
				functionSummaryTestMethod(path, "tokenService", "persistSession", "func (t tokenService) persistSession() error {\n\treturn nil\n}", ""),
			},
		},
	}

	call := functionSummaryTestInternalMethodCall(path, "svc", "persistSession")
	call.ResolvedTarget = path + "#persistSession"

	if got := resolvedInternalCallKey(idx, call); got != nil {
		t.Fatalf("resolvedInternalCallKey() = %q, want nil", got.Sign())
	}
}

func TestFunctionSummaryResolvedInternalCallKeyFailsClosedOnPackageQualifiedMixedFunctionMethodMatch(t *testing.T) {
	const path = "internal/auth/service.go"
	const helperPath = "internal/auth/session.go"

	idx := store.FileIndex{
		helperPath: {
			Path: helperPath,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", ""),
				functionSummaryTestMethod(helperPath, "sessionService", "persistSession", "func (s sessionService) persistSession() error {\n\treturn nil\n}", ""),
			},
		},
	}

	call := functionSummaryTestInternalMethodCall(path, "sessionpkg", "persistSession")
	call.ResolvedTarget = helperPath + "#persistSession"

	if got := resolvedInternalCallKey(idx, call); got != nil {
		t.Fatalf("resolvedInternalCallKey() = %q, want nil", got.Sign())
	}
}

func TestFunctionSummaryBuildBatchesRespectsContextBudget(t *testing.T) {
	const path = "internal/auth/service.go"
	src := strings.Repeat("a", 20)
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "A", src, ""),
				functionSummaryTestFunction(path, "B", src, ""),
				functionSummaryTestFunction(path, "C", src, ""),
			},
		},
	}

	batches := buildBatches([]*fnKey{
		{path: path, name: "A"},
		{path: path, name: "B"},
		{path: path, name: "C"},
	}, idx, 110, map[FuncSign]string{})

	want := [][]FuncSign{
		{FuncSign(path + "#A"), FuncSign(path + "#B")},
		{FuncSign(path + "#C")},
	}
	if got := functionSummaryTestBatchSigns(batches); !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBatches() = %v, want %v", got, want)
	}
}

func TestFunctionSummaryBuildBatchesKeepsOversizedFunctionAsSingleBatch(t *testing.T) {
	const path = "internal/auth/service.go"
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "A", strings.Repeat("a", 401), ""),
				functionSummaryTestFunction(path, "B", strings.Repeat("b", 20), ""),
			},
		},
	}

	batches := buildBatches([]*fnKey{
		{path: path, name: "A"},
		{path: path, name: "B"},
	}, idx, 100, map[FuncSign]string{})

	want := [][]FuncSign{
		{FuncSign(path + "#A")},
		{FuncSign(path + "#B")},
	}
	if got := functionSummaryTestBatchSigns(batches); !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBatches() = %v, want %v", got, want)
	}
}

func TestFunctionSummaryBuildPromptIncludesOnlyKnownInternalCalleeSummaries(t *testing.T) {
	const path = "internal/auth/service.go"
	const helperPath = "internal/auth/session.go"

	caller := functionSummaryTestFunction(
		path,
		"HandleRequest",
		"func HandleRequest() error {\n\tpersistSession()\n\tmissingHelper()\n\tfmt.Println(\"x\")\n\treturn nil\n}",
		"",
		functionSummaryTestInternalCall(helperPath, "persistSession"),
		functionSummaryTestInternalCall(path, "missingHelper"),
		functionSummaryTestExternalCall("fmt", "Println"),
	)
	helperSummary := helperPath + "#persistSession\nSummary: Persists the current session state before returning any storage error."
	helper := functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", helperSummary)

	state := newRunContext(store.FileIndex{
		path: {
			Path:      path,
			Functions: []*store.FunctionDecl{caller},
		},
		helperPath: {
			Path:      helperPath,
			Functions: []*store.FunctionDecl{helper},
		},
	})
	state.summaries[FuncSign("fmt#Println")] = "fmt#Println\nSummary: Writes output to stdout."

	cfg := &FunctionSummaryConfig{
		Model:           "test-model",
		MaxTokens:       512,
		DependencyDepth: 2,
	}
	agent := NewFunctionSummaryAgent(llm.NewMockClient(), cfg)

	req, err := agent.buildPrompt(state, &batch{keys: []*fnKey{{path: path, name: "HandleRequest"}}})
	if err != nil {
		t.Fatalf("buildPrompt() error = %v, want nil", err)
	}

	var systemBuf bytes.Buffer
	if err := promptpkg.FunctionSystemPromptTmp.Execute(&systemBuf, &promptpkg.FunctionSystemPromptData{
		Level: cfg.DependencyDepth - 1,
		Depth: cfg.DependencyDepth,
	}); err != nil {
		t.Fatalf("execute function system prompt: %v", err)
	}

	if req.SystemMsg != systemBuf.String() {
		t.Fatalf("buildPrompt().SystemMsg = %q, want %q", req.SystemMsg, systemBuf.String())
	}

	for _, want := range []string{
		"Path: " + caller.Path,
		caller.Src,
		helperSummary,
		"<called_functions>",
		"<function_summary>",
	} {
		if !strings.Contains(req.UserMsg, want) {
			t.Fatalf("buildPrompt().UserMsg missing %q:\n%s", want, req.UserMsg)
		}
	}
	if strings.Contains(req.UserMsg, "Writes output to stdout.") {
		t.Fatalf("buildPrompt().UserMsg unexpectedly included external callee summary:\n%s", req.UserMsg)
	}
	if got := strings.Count(req.UserMsg, "<function_summary>"); got != 1 {
		t.Fatalf("buildPrompt().UserMsg contained %d function summaries, want 1:\n%s", got, req.UserMsg)
	}
}

func TestFunctionSummaryBuildPromptDeduplicatesRepeatedInternalCallees(t *testing.T) {
	const path = "internal/auth/service.go"
	const helperPath = "internal/auth/session.go"

	helperSummary := helperPath + "#persistSession\nSummary: Persists the current session state before returning any storage error."
	caller := functionSummaryTestFunction(
		path,
		"HandleRequest",
		"func HandleRequest() error {\n\tpersistSession()\n\tpersistSession()\n\treturn persistSession()\n}",
		"",
		functionSummaryTestInternalCall(helperPath, "persistSession"),
		functionSummaryTestInternalCall(helperPath, "persistSession"),
		functionSummaryTestInternalCall(helperPath, "persistSession"),
	)
	helper := functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", helperSummary)

	idx := store.FileIndex{
		path: {
			Path:      path,
			Functions: []*store.FunctionDecl{caller},
		},
		helperPath: {
			Path:      helperPath,
			Functions: []*store.FunctionDecl{helper},
		},
	}
	state := newRunContext(idx)

	wantCost := estimateTokens(len(caller.Src)) + 50 + estimateTokens(len(helperSummary))
	if got := estimateFunctionTokens(idx, caller, state.summaries); got != wantCost {
		t.Fatalf("estimateFunctionTokens() = %d, want %d", got, wantCost)
	}

	agent := NewFunctionSummaryAgent(llm.NewMockClient(), &FunctionSummaryConfig{
		Model:     "test-model",
		MaxTokens: 512,
	})

	req, err := agent.buildPrompt(state, &batch{keys: []*fnKey{{path: path, name: "HandleRequest"}}})
	if err != nil {
		t.Fatalf("buildPrompt() error = %v, want nil", err)
	}
	if got := strings.Count(req.UserMsg, helperSummary); got != 1 {
		t.Fatalf("buildPrompt().UserMsg contained %d copies of helper summary, want 1:\n%s", got, req.UserMsg)
	}
	if got := strings.Count(req.UserMsg, "<function_summary>"); got != 1 {
		t.Fatalf("buildPrompt().UserMsg contained %d function summaries, want 1:\n%s", got, req.UserMsg)
	}
}

func TestFunctionSummaryBuildPromptUsesEventFlowTemplatesWhenEnabled(t *testing.T) {
	const path = "internal/auth/service.go"
	const helperPath = "internal/auth/session.go"

	helperSummary := helperPath + "#persistSession\nSummary: Persists session state before returning any storage error."
	caller := functionSummaryTestMethod(
		path,
		"sessionService",
		"HandleRequest",
		"func (s sessionService) HandleRequest() error {\n\treturn persistSession()\n}",
		"",
		functionSummaryTestInternalCall(helperPath, "persistSession"),
	)
	helper := functionSummaryTestFunction(helperPath, "persistSession", "func persistSession() error {\n\treturn nil\n}", helperSummary)

	state := newRunContext(store.FileIndex{
		path: {
			Path:      path,
			Functions: []*store.FunctionDecl{caller},
		},
		helperPath: {
			Path:      helperPath,
			Functions: []*store.FunctionDecl{helper},
		},
	})
	agent := NewFunctionSummaryAgent(llm.NewMockClient(), &FunctionSummaryConfig{
		Model:           "test-model",
		MaxTokens:       512,
		DependencyDepth: 2,
		EnableEventFlow: true,
	})

	req, err := agent.buildPrompt(state, &batch{keys: []*fnKey{{path: path, receiver: "sessionService", name: "HandleRequest"}}})
	if err != nil {
		t.Fatalf("buildPrompt() error = %v, want nil", err)
	}

	for _, want := range []string{
		"ID: sessionService#HandleRequest",
		"Receiver: sessionService",
		"Name: HandleRequest",
		"Signature:",
		"event flow",
		`"event_facts"`,
		`"event_hints"`,
	} {
		if !strings.Contains(req.SystemMsg+"\n"+req.UserMsg, want) {
			t.Fatalf("event-flow prompt missing %q:\nSYSTEM:\n%s\nUSER:\n%s", want, req.SystemMsg, req.UserMsg)
		}
	}
	if !strings.Contains(req.UserMsg, helperSummary) {
		t.Fatalf("event-flow prompt missing callee summary:\n%s", req.UserMsg)
	}
}

func TestFunctionSummaryBuildPromptTemplateSmoke(t *testing.T) {
	data := &promptpkg.FunctionUserPromptData{
		Functions: []promptpkg.FunctionStruct{{
			Path: "internal/auth/service.go",
			Src:  "func HandleRequest() error {\n\treturn persistSession()\n}",
			CalledFunctions: []*promptpkg.CalledFunctionStruct{{
				Name:    "internal/auth/session.go#persistSession",
				Summary: "internal/auth/session.go#persistSession\nSummary: Persists the current session state before returning any storage error.",
			}},
		}},
	}

	var buf bytes.Buffer
	if err := promptpkg.FunctionUserPromptTmp.Execute(&buf, data); err != nil {
		t.Fatalf("FunctionUserPromptTmp.Execute() error = %v, want nil", err)
	}

	got := buf.String()
	for _, want := range []string{
		"Path: internal/auth/service.go",
		"func HandleRequest() error {",
		"<called_functions>",
		"<function_summary>",
		"internal/auth/session.go#persistSession",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FunctionUserPromptTmp output missing %q:\n%s", want, got)
		}
	}
}

func TestFunctionSummaryParseResponseAcceptsBareJSONAndFencedJSON(t *testing.T) {
	agent := NewFunctionSummaryAgent(nil, nil)
	want := []*functionSummaryResult{{
		ID:      "HandleRequest",
		Path:    "internal/auth/service.go",
		Summary: "internal/auth/service.go#HandleRequest\nSummary: Handles the request.",
	}}
	content := functionSummaryTestResponse(want...)

	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "bare", content: content},
		{name: "fenced", content: "```json\n" + content + "\n```"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := agent.parseResponse(tc.content)
			if err != nil {
				t.Fatalf("parseResponse() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("parseResponse() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestFunctionSummaryRunAppliesEventFactsAndHintsFromResponse(t *testing.T) {
	const path = "internal/auth/service.go"
	const summary = path + "#HandleRequest\nSummary: Handles a request and emits a confirmed event."

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn nil\n}", ""),
			},
		},
	}
	client := llm.NewMockClient(functionSummaryTestResponse(&functionSummaryResult{
		ID:      "HandleRequest",
		Path:    path,
		Summary: summary,
		EventFacts: &store.EventFacts{Publishes: []*store.EventFact{{
			EventName: "user.created",
			Line:      12,
			Evidence:  "bus.Publish(UserCreated)",
		}}},
		EventHints: &store.EventHints{LikelyHandles: []*store.EventFact{{
			EventName:  "user.created",
			HandlerRef: "AuditHandler",
			Line:       18,
			Evidence:   "comment hint",
		}}},
	}))
	agent := NewFunctionSummaryAgent(client, &FunctionSummaryConfig{
		Model:           "test-model",
		MaxTokens:       512,
		ContextBudget:   1024,
		EnableEventFlow: true,
	})

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	fn := idx[path].Functions[0]
	if fn.Summary != summary {
		t.Fatalf("Summary = %q, want %q", fn.Summary, summary)
	}
	if fn.EventFacts == nil || len(fn.EventFacts.Publishes) != 1 {
		t.Fatalf("EventFacts.Publishes = %#v, want one confirmed fact", fn.EventFacts)
	}
	if got := fn.EventFacts.Publishes[0].EventName; got != "user.created" {
		t.Fatalf("EventFacts.Publishes[0].EventName = %q, want user.created", got)
	}
	if fn.EventHints == nil || len(fn.EventHints.LikelyHandles) != 1 {
		t.Fatalf("EventHints.LikelyHandles = %#v, want one likely hint", fn.EventHints)
	}
	if got := fn.EventHints.LikelyHandles[0].HandlerRef; got != "AuditHandler" {
		t.Fatalf("EventHints.LikelyHandles[0].HandlerRef = %q, want AuditHandler", got)
	}
}

func TestFunctionSummaryRunRequestsLowImportanceFunctionWhenEventFlowEnabled(t *testing.T) {
	const path = "internal/auth/service.go"
	const summary = path + "#EmitUserCreated\nSummary: Emits the user created event."

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "EmitUserCreated", "func EmitUserCreated() error {\n\treturn nil\n}", ""),
			},
		},
	}
	client := llm.NewMockClient(functionSummaryTestResponse(&functionSummaryResult{ID: "EmitUserCreated", Path: path, Summary: summary}))
	agent := NewFunctionSummaryAgent(client, &FunctionSummaryConfig{
		Model:               "test-model",
		MaxTokens:           512,
		ContextBudget:       1024,
		ImportanceThreshold: 0.9,
		EnableEventFlow:     true,
	})
	metrics := store.MetricsMap{
		path + "#EmitUserCreated": {FuncID: path + "#EmitUserCreated", ImportanceScore: 0.01},
	}

	if err := agent.Run(context.Background(), idx, metrics); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}
	if got := idx[path].Functions[0].Summary; got != summary {
		t.Fatalf("Summary = %q, want %q", got, summary)
	}
}

func TestFunctionSummaryApplySummariesUpdatesIndexAndSummaryMap(t *testing.T) {
	const path = "internal/auth/service.go"

	freeFn := functionSummaryTestFunction(path, "persistSession", "func persistSession() error {\n\treturn nil\n}", "")
	methodFn := functionSummaryTestMethod(path, "sessionService", "persistSession", "func (s sessionService) persistSession() error {\n\treturn nil\n}", "")
	rc := newRunContext(store.FileIndex{
		path: {
			Path:      path,
			Functions: []*store.FunctionDecl{freeFn, methodFn},
		},
	})
	agent := NewFunctionSummaryAgent(nil, nil)

	sign := FuncSign(path + "#sessionService#persistSession")
	summary := string(sign) + "\nSummary: Persists session state for the receiver instance."
	applied, failures := agent.applySummaries(rc, map[FuncSign]*functionSummaryMatch{
		sign: {
			key: &fnKey{path: path, receiver: "sessionService", name: "persistSession"},
			result: &functionSummaryResult{
				ID:      "persistSession",
				Path:    path,
				Summary: summary,
			},
		},
	})

	if len(failures) != 0 {
		t.Fatalf("applySummaries() failures = %v, want none", failures)
	}
	if !reflect.DeepEqual(applied, []FuncSign{sign}) {
		t.Fatalf("applySummaries() applied = %v, want [%s]", applied, sign)
	}
	if got := rc.summaries[sign]; got != summary {
		t.Fatalf("runContext.summaries[%q] = %q, want %q", sign, got, summary)
	}
	if got := methodFn.Summary; got != summary {
		t.Fatalf("method summary = %q, want %q", got, summary)
	}
	if freeFn.Summary != "" {
		t.Fatalf("free function summary = %q, want empty", freeFn.Summary)
	}
}

func TestFunctionSummaryRunReturnsAggregateErrorAfterLayerFailure(t *testing.T) {
	const path = "internal/auth/service.go"

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "FailLeaf", "func FailLeaf() error {\n\treturn nil\n}", ""),
				functionSummaryTestFunction(path, "PassLeaf", "func PassLeaf() error {\n\treturn nil\n}", ""),
			},
		},
	}
	passSummary := path + "#PassLeaf\nSummary: Completes the independent leaf work."
	client := llm.NewMockClient("not json", functionSummaryTestResponse(&functionSummaryResult{ID: "PassLeaf", Path: path, Summary: passSummary}))
	agent := functionSummaryTestAgent(client, 100, 0)

	err := agent.Run(context.Background(), idx, nil)
	runErr := functionSummaryTestRunError(t, err)

	if got := functionSummaryTestFailureSigns(runErr); !reflect.DeepEqual(got, []FuncSign{FuncSign(path + "#FailLeaf")}) {
		t.Fatalf("run failure signs = %v, want [FailLeaf]", got)
	}
	if len(runErr.Blocked) != 0 {
		t.Fatalf("run blocked = %v, want none", runErr.Blocked)
	}
	if got := idx[path].Functions[0].Summary; got != "" {
		t.Fatalf("FailLeaf summary = %q, want empty", got)
	}
	if got := idx[path].Functions[1].Summary; got != passSummary {
		t.Fatalf("PassLeaf summary = %q, want %q", got, passSummary)
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("CallCount() = %d, want 2", got)
	}
}

func TestFunctionSummaryRunDoesNotUnlockCallerAfterCalleeBatchFailure(t *testing.T) {
	const path = "internal/auth/service.go"

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "Caller", "func Caller() error {\n\treturn FailLeaf()\n}", "", functionSummaryTestInternalCall(path, "FailLeaf")),
				functionSummaryTestFunction(path, "FailLeaf", "func FailLeaf() error {\n\treturn nil\n}", ""),
				functionSummaryTestFunction(path, "OtherLeaf", "func OtherLeaf() error {\n\treturn nil\n}", ""),
			},
		},
	}
	otherLeafSummary := path + "#OtherLeaf\nSummary: Performs unrelated leaf work."
	client := llm.NewMockClient("not json", functionSummaryTestResponse(&functionSummaryResult{ID: "OtherLeaf", Path: path, Summary: otherLeafSummary}))
	agent := functionSummaryTestAgent(client, 100, 0)

	err := agent.Run(context.Background(), idx, nil)
	runErr := functionSummaryTestRunError(t, err)

	if got := functionSummaryTestFailureSigns(runErr); !reflect.DeepEqual(got, []FuncSign{FuncSign(path + "#FailLeaf")}) {
		t.Fatalf("run failure signs = %v, want [FailLeaf]", got)
	}
	if !reflect.DeepEqual(runErr.Blocked, []FuncSign{FuncSign(path + "#Caller")}) {
		t.Fatalf("run blocked = %v, want [Caller]", runErr.Blocked)
	}
	if got := idx[path].Functions[0].Summary; got != "" {
		t.Fatalf("Caller summary = %q, want empty", got)
	}
	if got := idx[path].Functions[2].Summary; got != otherLeafSummary {
		t.Fatalf("OtherLeaf summary = %q, want %q", got, otherLeafSummary)
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("CallCount() = %d, want 2", got)
	}
}

func TestFunctionSummaryRunTreatsMissingBatchResultsAsFailure(t *testing.T) {
	const path = "internal/auth/service.go"

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "Alpha", "func Alpha() error {\n\treturn nil\n}", ""),
				functionSummaryTestFunction(path, "Beta", "func Beta() error {\n\treturn nil\n}", ""),
			},
		},
	}
	alphaSummary := path + "#Alpha\nSummary: Handles the alpha leaf path."
	client := llm.NewMockClient(functionSummaryTestResponse(&functionSummaryResult{ID: "Alpha", Path: path, Summary: alphaSummary}))
	agent := functionSummaryTestAgent(client, 1000, 0)

	err := agent.Run(context.Background(), idx, nil)
	runErr := functionSummaryTestRunError(t, err)

	if got := functionSummaryTestFailureSigns(runErr); !reflect.DeepEqual(got, []FuncSign{FuncSign(path + "#Beta")}) {
		t.Fatalf("run failure signs = %v, want [Beta]", got)
	}
	if got := idx[path].Functions[0].Summary; got != alphaSummary {
		t.Fatalf("Alpha summary = %q, want %q", got, alphaSummary)
	}
	if got := idx[path].Functions[1].Summary; got != "" {
		t.Fatalf("Beta summary = %q, want empty", got)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}
}

func TestFunctionSummaryRunProcessesLeafBeforeCaller(t *testing.T) {
	const path = "internal/auth/service.go"

	helperSummary := path + "#helper\nSummary: Validates request state before returning."
	callerSummary := path + "#caller\nSummary: Handles the request after delegating to helper."
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "caller", "func caller() error {\n\treturn helper()\n}", "", functionSummaryTestInternalCall(path, "helper")),
				functionSummaryTestFunction(path, "helper", "func helper() error {\n\treturn nil\n}", ""),
			},
		},
	}
	client := llm.NewMockClient(
		functionSummaryTestResponse(&functionSummaryResult{ID: "helper", Path: path, Summary: helperSummary}),
		functionSummaryTestResponse(&functionSummaryResult{ID: "caller", Path: path, Summary: callerSummary}),
	)
	agent := functionSummaryTestAgent(client, 1000, 0)

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("CallCount() = %d, want 2", got)
	}

	calls := client.Calls()
	if !strings.Contains(calls[0].UserMsg, "func helper() error") {
		t.Fatalf("first prompt missing helper source:\n%s", calls[0].UserMsg)
	}
	if strings.Contains(calls[0].UserMsg, "func caller() error") {
		t.Fatalf("first prompt unexpectedly contained caller source:\n%s", calls[0].UserMsg)
	}
	if !strings.Contains(calls[1].UserMsg, "func caller() error") {
		t.Fatalf("second prompt missing caller source:\n%s", calls[1].UserMsg)
	}
	if !strings.Contains(calls[1].UserMsg, helperSummary) {
		t.Fatalf("second prompt missing helper summary:\n%s", calls[1].UserMsg)
	}
	if got := idx[path].Functions[0].Summary; got != callerSummary {
		t.Fatalf("caller summary = %q, want %q", got, callerSummary)
	}
	if got := idx[path].Functions[1].Summary; got != helperSummary {
		t.Fatalf("helper summary = %q, want %q", got, helperSummary)
	}
}

func TestFunctionSummaryRunReusesExistingSummaryWithoutReRequesting(t *testing.T) {
	const path = "internal/auth/service.go"

	helperSummary := path + "#helper\nSummary: Reuses the existing helper summary."
	callerSummary := path + "#caller\nSummary: Handles the request using the helper summary."
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "caller", "func caller() error {\n\treturn helper()\n}", "", functionSummaryTestInternalCall(path, "helper")),
				functionSummaryTestFunction(path, "helper", "func helper() error {\n\treturn nil\n}", helperSummary),
			},
		},
	}
	client := llm.NewMockClient(functionSummaryTestResponse(&functionSummaryResult{ID: "caller", Path: path, Summary: callerSummary}))
	agent := functionSummaryTestAgent(client, 1000, 0)

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}

	call := client.Calls()[0]
	if !strings.Contains(call.UserMsg, helperSummary) {
		t.Fatalf("prompt missing reused helper summary:\n%s", call.UserMsg)
	}
	if strings.Contains(call.UserMsg, "func helper() error") {
		t.Fatalf("prompt unexpectedly requested helper source again:\n%s", call.UserMsg)
	}
	if got := idx[path].Functions[0].Summary; got != callerSummary {
		t.Fatalf("caller summary = %q, want %q", got, callerSummary)
	}
	if got := idx[path].Functions[1].Summary; got != helperSummary {
		t.Fatalf("helper summary = %q, want %q", got, helperSummary)
	}
}

func TestFunctionSummaryRunProcessesCycleInFinalPass(t *testing.T) {
	const path = "internal/auth/service.go"

	stepASummary := path + "#stepA\nSummary: Performs the first cycle step."
	stepBSummary := path + "#stepB\nSummary: Performs the second cycle step."
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "stepA", "func stepA() error {\n\treturn stepB()\n}", "", functionSummaryTestInternalCall(path, "stepB")),
				functionSummaryTestFunction(path, "stepB", "func stepB() error {\n\treturn stepA()\n}", "", functionSummaryTestInternalCall(path, "stepA")),
			},
		},
	}
	client := llm.NewMockClient(functionSummaryTestResponse(
		&functionSummaryResult{ID: "stepA", Path: path, Summary: stepASummary},
		&functionSummaryResult{ID: "stepB", Path: path, Summary: stepBSummary},
	))
	agent := functionSummaryTestAgent(client, 1000, 0)

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}
	call := client.Calls()[0]
	for _, want := range []string{"func stepA() error", "func stepB() error"} {
		if !strings.Contains(call.UserMsg, want) {
			t.Fatalf("cycle prompt missing %q:\n%s", want, call.UserMsg)
		}
	}
	if got := idx[path].Functions[0].Summary; got != stepASummary {
		t.Fatalf("stepA summary = %q, want %q", got, stepASummary)
	}
	if got := idx[path].Functions[1].Summary; got != stepBSummary {
		t.Fatalf("stepB summary = %q, want %q", got, stepBSummary)
	}
}

func TestFunctionSummaryRunReturnsErrorWhenCycleBatchIsPartial(t *testing.T) {
	const path = "internal/auth/service.go"

	stepASummary := path + "#stepA\nSummary: Performs the first cycle step."
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "stepA", "func stepA() error {\n\treturn stepB()\n}", "", functionSummaryTestInternalCall(path, "stepB")),
				functionSummaryTestFunction(path, "stepB", "func stepB() error {\n\treturn stepA()\n}", "", functionSummaryTestInternalCall(path, "stepA")),
			},
		},
	}
	client := llm.NewMockClient(functionSummaryTestResponse(&functionSummaryResult{ID: "stepA", Path: path, Summary: stepASummary}))
	agent := functionSummaryTestAgent(client, 1000, 0)

	err := agent.Run(context.Background(), idx, nil)
	runErr := functionSummaryTestRunError(t, err)

	if got := functionSummaryTestFailureSigns(runErr); !reflect.DeepEqual(got, []FuncSign{FuncSign(path + "#stepB")}) {
		t.Fatalf("run failure signs = %v, want [stepB]", got)
	}
	if got := idx[path].Functions[0].Summary; got != stepASummary {
		t.Fatalf("stepA summary = %q, want %q", got, stepASummary)
	}
	if got := idx[path].Functions[1].Summary; got != "" {
		t.Fatalf("stepB summary = %q, want empty", got)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}
}

func TestFunctionSummaryAgentRetriesTransientLLMFailure(t *testing.T) {
	const path = "internal/auth/service.go"

	summary := path + "#HandleRequest\nSummary: Handles the request after a transient retry."
	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "HandleRequest", "func HandleRequest() error {\n\treturn nil\n}", ""),
			},
		},
	}
	client := llm.NewMockClient("ignored", functionSummaryTestResponse(&functionSummaryResult{ID: "HandleRequest", Path: path, Summary: summary})).WithErrors(
		&llm.LLMError{StatusCode: 500, Message: "retry me", Retryable: true},
		nil,
	)
	agent := functionSummaryTestAgent(client, 1000, 1)

	if err := agent.Run(context.Background(), idx, nil); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("CallCount() = %d, want 2", got)
	}
	if got := idx[path].Functions[0].Summary; got != summary {
		t.Fatalf("HandleRequest summary = %q, want %q", got, summary)
	}
}

func TestFunctionSummaryRunReturnsErrorWhenCycleBatchHasDuplicatePathIDPairs(t *testing.T) {
	const path = "internal/auth/service.go"

	idx := store.FileIndex{
		path: {
			Path: path,
			Functions: []*store.FunctionDecl{
				functionSummaryTestFunction(path, "step", "func step() error {\n\treturn step()\n}", "", functionSummaryTestInternalCall(path, "step")),
				functionSummaryTestMethod(path, "svc", "step", "func (s svc) step() error {\n\treturn step()\n}", "", functionSummaryTestInternalCall(path, "step")),
			},
		},
	}
	client := llm.NewMockClient()
	agent := functionSummaryTestAgent(client, 1000, 0)

	err := agent.Run(context.Background(), idx, nil)
	runErr := functionSummaryTestRunError(t, err)

	wantFailed := []FuncSign{FuncSign(path + "#step"), FuncSign(path + "#svc#step")}
	if got := functionSummaryTestFailureSigns(runErr); !reflect.DeepEqual(got, wantFailed) {
		t.Fatalf("run failure signs = %v, want %v", got, wantFailed)
	}
	if len(runErr.Blocked) != 0 {
		t.Fatalf("run blocked = %v, want none", runErr.Blocked)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1", got)
	}
}

func functionSummaryTestFunction(path string, name string, src string, summary string, calls ...*store.CallRef) *store.FunctionDecl {
	return &store.FunctionDecl{
		Name:    name,
		Path:    path,
		Src:     src,
		Summary: summary,
		Calls:   calls,
	}
}

func functionSummaryTestMethod(path string, receiver string, name string, src string, summary string, calls ...*store.CallRef) *store.FunctionDecl {
	return &store.FunctionDecl{
		Name:         name,
		Receiver:     receiver,
		FunctionType: store.FunctionTypeMethod,
		Path:         path,
		Src:          src,
		Summary:      summary,
		Calls:        calls,
	}
}

func functionSummaryTestInternalCall(path string, name string) *store.CallRef {
	return &store.CallRef{
		Name:      name,
		Path:      path,
		Ownership: store.OwnershipInternal,
	}
}

func functionSummaryTestInternalMethodCall(path string, receiver string, name string) *store.CallRef {
	call := functionSummaryTestInternalCall(path, name)
	call.Receiver = receiver
	return call
}

func functionSummaryTestExternalCall(path string, name string) *store.CallRef {
	return &store.CallRef{
		Name:      name,
		Path:      path,
		Ownership: store.OwnershipExternal,
	}
}

func functionSummaryTestSigns(keys []*fnKey) []FuncSign {
	signs := make([]FuncSign, 0, len(keys))
	for _, key := range keys {
		signs = append(signs, key.Sign())
	}
	return signs
}

func functionSummaryTestBatchSigns(batches []*batch) [][]FuncSign {
	signs := make([][]FuncSign, 0, len(batches))
	for _, currentBatch := range batches {
		if currentBatch == nil {
			signs = append(signs, nil)
			continue
		}
		signs = append(signs, functionSummaryTestSigns(currentBatch.keys))
	}
	return signs
}

func functionSummaryTestAgent(client llm.Client, contextBudget int, maxRetries int) *FunctionSummaryAgent {
	return NewFunctionSummaryAgent(client, &FunctionSummaryConfig{
		Model:         "test-model",
		MaxTokens:     512,
		ContextBudget: contextBudget,
		MaxRetries:    maxRetries,
	})
}

func functionSummaryTestResponse(results ...*functionSummaryResult) string {
	payload, err := json.Marshal(&functionSummaryResponse{Functions: results})
	if err != nil {
		panic(err)
	}
	return string(payload)
}

func functionSummaryTestRunError(t *testing.T, err error) *FunctionSummaryRunError {
	t.Helper()
	if err == nil {
		t.Fatal("Run() error = nil, want FunctionSummaryRunError")
	}
	var runErr *FunctionSummaryRunError
	if !errors.As(err, &runErr) {
		t.Fatalf("Run() error = %T %v, want FunctionSummaryRunError", err, err)
	}
	return runErr
}

func functionSummaryTestFailureSigns(runErr *FunctionSummaryRunError) []FuncSign {
	if runErr == nil {
		return nil
	}
	signs := make([]FuncSign, 0, len(runErr.Failed))
	for sign := range runErr.Failed {
		signs = append(signs, sign)
	}
	sort.Slice(signs, func(i int, j int) bool {
		return signs[i] < signs[j]
	})
	return signs
}

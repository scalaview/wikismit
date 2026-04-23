package planner

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func TestFlowGraphResultJSONShape(t *testing.T) {
	t.Helper()

	result := &FlowGraphResult{
		Nodes:          []*FlowNode{{ID: "svc/handler.go#HandleRequest", Kind: "function", Label: "HandleRequest"}},
		Edges:          []*FlowEdge{{From: "svc/handler.go#HandleRequest", To: "svc/handler.go#process", Kind: "call", Confidence: "confirmed", Source: "call_graph"}},
		MissingLinks:   []*MissingLink{{From: "event:user.created", To: "svc/handler.go#HandleRequest", Problem: "handler unresolved"}},
		OpenQuestions:  []*OpenQuestion{{Question: "Which function publishes user.created?", Context: "auth flow"}},
		CandidateReads: []*CandidateRead{{Target: "svc/handler.go#HandleRequest", Reason: "entrypoint to inspect"}},
		Truncated:      true,
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{"nodes", "edges", "missing_links", "open_questions", "candidate_reads", "truncated"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("marshaled FlowGraphResult missing key %q: %s", key, raw)
		}
	}
	if _, ok := got["missingLinks"]; ok {
		t.Fatalf("marshaled FlowGraphResult used camelCase key: %s", raw)
	}
	if _, ok := got["candidateReads"]; ok {
		t.Fatalf("marshaled FlowGraphResult used camelCase key: %s", raw)
	}
}

func TestQueryCallChainDownstreamBuildsFunctionGraph(t *testing.T) {
	t.Helper()

	idx, callGraph := buildQueryCallChainFixture()
	seed := mustFuncID(t, idx, "svc/handler.go", "HandleRequest")

	got, err := QueryCallChain(idx, callGraph, store.EventFactIndex{}, CallChainQuery{
		FunctionRef: seed,
		Direction:   "downstream",
		Depth:       2,
	})
	if err != nil {
		t.Fatalf("QueryCallChain() error = %v", err)
	}

	wantNodeIDs := []string{
		mustFuncID(t, idx, "svc/handler.go", "HandleRequest"),
		mustFuncID(t, idx, "svc/handler.go", "doWork"),
		mustFuncID(t, idx, "svc/handler.go", "log"),
		mustFuncID(t, idx, "svc/handler.go", "process"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryCallChain() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"svc/handler.go#HandleRequest->svc/handler.go#log",
		"svc/handler.go#HandleRequest->svc/handler.go#process",
		"svc/handler.go#process->svc/handler.go#doWork",
	}
	if gotEdges := collectEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryCallChain() edges = %#v, want %#v", gotEdges, wantEdges)
	}
	if got.Truncated {
		t.Fatal("QueryCallChain() Truncated = true, want false")
	}
}

func TestQueryCallChainUpstreamBuildsFunctionGraph(t *testing.T) {
	t.Helper()

	idx, callGraph := buildQueryCallChainFixture()
	seed := mustFuncID(t, idx, "svc/handler.go", "log")

	got, err := QueryCallChain(idx, callGraph, store.EventFactIndex{}, CallChainQuery{
		FunctionRef: seed,
		Direction:   "upstream",
		Depth:       2,
	})
	if err != nil {
		t.Fatalf("QueryCallChain() error = %v", err)
	}

	wantNodeIDs := []string{
		mustFuncID(t, idx, "svc/handler.go", "HandleRequest"),
		mustFuncID(t, idx, "svc/handler.go", "doWork"),
		mustFuncID(t, idx, "svc/handler.go", "log"),
		mustFuncID(t, idx, "svc/handler.go", "process"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryCallChain() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"svc/handler.go#HandleRequest->svc/handler.go#log",
		"svc/handler.go#doWork->svc/handler.go#log",
		"svc/handler.go#process->svc/handler.go#doWork",
	}
	if gotEdges := collectEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryCallChain() edges = %#v, want %#v", gotEdges, wantEdges)
	}
}

func TestQueryCallChainRejectsUnknownFunction(t *testing.T) {
	t.Helper()

	idx, callGraph := buildQueryCallChainFixture()

	_, err := QueryCallChain(idx, callGraph, store.EventFactIndex{}, CallChainQuery{
		FunctionRef: "svc/handler.go#missing",
		Direction:   "downstream",
		Depth:       1,
	})
	if err == nil {
		t.Fatal("QueryCallChain() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown function ref") {
		t.Fatalf("QueryCallChain() error = %v, want unknown function ref context", err)
	}
}

func TestQueryCallChainSupportsMethodFunctionRefsAgainstRawCallGraphKeys(t *testing.T) {
	t.Helper()

	idx, callGraph := buildMethodCallChainFixture()
	seed := mustMethodFuncID(t, idx, "svc/service.go", "*Service", "HandleRequest")

	got, err := QueryCallChain(idx, callGraph, store.EventFactIndex{}, CallChainQuery{
		FunctionRef: seed,
		Direction:   "downstream",
		Depth:       2,
	})
	if err != nil {
		t.Fatalf("QueryCallChain() error = %v", err)
	}

	wantNodeIDs := []string{
		mustMethodFuncID(t, idx, "svc/service.go", "*Service", "HandleRequest"),
		mustMethodFuncID(t, idx, "svc/service.go", "*Service", "process"),
		mustFuncID(t, idx, "svc/service.go", "logAudit"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryCallChain() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"svc/service.go#*Service#HandleRequest->svc/service.go#*Service#process",
		"svc/service.go#*Service#process->svc/service.go#logAudit",
	}
	if gotEdges := collectEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryCallChain() edges = %#v, want %#v", gotEdges, wantEdges)
	}
	if got.Truncated {
		t.Fatal("QueryCallChain() Truncated = true, want false")
	}
	if hasDisconnectedFunctionNode(got) {
		t.Fatalf("QueryCallChain() returned disconnected function nodes: %#v %#v", collectNodeIDs(got.Nodes), collectTypedEdgeKeys(got.Edges))
	}
}

func TestQueryEventFlowBuildsPublisherAndHandlerGraph(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()

	got, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:        "user.created",
		ExpandPublishers: true,
		ExpandHandlers:   true,
		HandlerDepth:     1,
	})
	if err != nil {
		t.Fatalf("QueryEventFlow() error = %v", err)
	}

	wantNodeIDs := []string{
		"event:user.created",
		mustFuncID(t, idx, "svc/events.go", "PublishUserCreated"),
		mustFuncID(t, idx, "svc/handlers.go", "HandleUserCreated"),
		mustFuncID(t, idx, "svc/handlers.go", "PersistAuditRecord"),
		mustFuncID(t, idx, "svc/registry.go", "RegisterUserCreatedHandler"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryEventFlow() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"event:user.created->svc/handlers.go#HandleUserCreated:handle",
		"svc/events.go#PublishUserCreated->event:user.created:publish",
		"svc/handlers.go#HandleUserCreated->svc/handlers.go#PersistAuditRecord:call",
		"svc/registry.go#RegisterUserCreatedHandler->svc/handlers.go#HandleUserCreated:register",
	}
	if gotEdges := collectTypedEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryEventFlow() edges = %#v, want %#v", gotEdges, wantEdges)
	}
	if got.Truncated {
		t.Fatal("QueryEventFlow() Truncated = true, want false")
	}
}

func TestQueryEventFlowAddsRegistrationEdgesWhenPresent(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()

	got, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:        "user.created",
		ExpandPublishers: false,
		ExpandHandlers:   false,
	})
	if err != nil {
		t.Fatalf("QueryEventFlow() error = %v", err)
	}

	wantEdges := []string{
		"event:user.created->svc/handlers.go#HandleUserCreated:handle",
		"svc/events.go#PublishUserCreated->event:user.created:publish",
		"svc/registry.go#RegisterUserCreatedHandler->svc/handlers.go#HandleUserCreated:register",
	}
	if gotEdges := collectTypedEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryEventFlow() edges = %#v, want %#v", gotEdges, wantEdges)
	}
}

func TestQueryCallChainIncludeEventsAddsEventEdges(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()
	seed := mustFuncID(t, idx, "svc/registry.go", "RegisterUserCreatedHandler")

	got, err := QueryCallChain(idx, callGraph, eventIdx, CallChainQuery{
		FunctionRef:   seed,
		Direction:     "downstream",
		Depth:         1,
		IncludeEvents: true,
	})
	if err != nil {
		t.Fatalf("QueryCallChain() error = %v", err)
	}

	wantNodeIDs := []string{
		"event:user.created",
		mustFuncID(t, idx, "svc/handlers.go", "HandleUserCreated"),
		mustFuncID(t, idx, "svc/registry.go", "RegisterUserCreatedHandler"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryCallChain() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"event:user.created->svc/handlers.go#HandleUserCreated:handle",
		"svc/registry.go#RegisterUserCreatedHandler->svc/handlers.go#HandleUserCreated:call",
		"svc/registry.go#RegisterUserCreatedHandler->svc/handlers.go#HandleUserCreated:register",
	}
	if gotEdges := collectTypedEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryCallChain() edges = %#v, want %#v", gotEdges, wantEdges)
	}
}

func TestQueryEventFlowReportsMissingLinksWhenOnlyPublisherExists(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()
	eventIdx.Events = []*store.EventEntry{{
		EventName: "orphaned.event",
		Publishers: []*store.EventFact{{
			EventName: "orphaned.event",
			FuncID:    mustFuncID(t, idx, "svc/events.go", "PublishUserCreated"),
			Line:      12,
			Evidence:  "bus.Publish(orphaned.event)",
		}},
	}}

	got, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:        "orphaned.event",
		ExpandPublishers: true,
	})
	if err != nil {
		t.Fatalf("QueryEventFlow() error = %v", err)
	}

	wantMissing := []*MissingLink{{
		From:    "event:orphaned.event",
		Problem: "no confirmed handlers",
	}}
	if !reflect.DeepEqual(got.MissingLinks, wantMissing) {
		t.Fatalf("QueryEventFlow() MissingLinks = %#v, want %#v", got.MissingLinks, wantMissing)
	}
}

func TestQueryEventFlowSuggestsCandidateReadsForRegistrations(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()

	got, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName: "user.created",
	})
	if err != nil {
		t.Fatalf("QueryEventFlow() error = %v", err)
	}

	wantReads := []*CandidateRead{{
		Target: mustFuncID(t, idx, "svc/registry.go", "RegisterUserCreatedHandler"),
		Reason: "registration for event user.created",
	}}
	if !reflect.DeepEqual(got.CandidateReads, wantReads) {
		t.Fatalf("QueryEventFlow() CandidateReads = %#v, want %#v", got.CandidateReads, wantReads)
	}
}

func TestQueryEventFlowExpandsMethodHandlersAgainstRawCallGraphKeys(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildMethodEventFlowFixture()

	got, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:      "user.created",
		ExpandHandlers: true,
		HandlerDepth:   1,
	})
	if err != nil {
		t.Fatalf("QueryEventFlow() error = %v", err)
	}

	wantNodeIDs := []string{
		"event:user.created",
		mustMethodFuncID(t, idx, "svc/subscriber.go", "*Subscriber", "HandleUserCreated"),
		mustFuncID(t, idx, "svc/subscriber.go", "PublishUserCreated"),
		mustFuncID(t, idx, "svc/subscriber.go", "persistAuditRecord"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryEventFlow() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{
		"event:user.created->svc/subscriber.go#*Subscriber#HandleUserCreated:handle",
		"svc/subscriber.go#*Subscriber#HandleUserCreated->svc/subscriber.go#persistAuditRecord:call",
		"svc/subscriber.go#PublishUserCreated->event:user.created:publish",
	}
	if gotEdges := collectTypedEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryEventFlow() edges = %#v, want %#v", gotEdges, wantEdges)
	}
	if hasDisconnectedFunctionNode(got) {
		t.Fatalf("QueryEventFlow() returned disconnected function nodes: %#v %#v", collectNodeIDs(got.Nodes), collectTypedEdgeKeys(got.Edges))
	}
}

func TestQueryCallChainRespectsMaxNodesAndMarksTruncated(t *testing.T) {
	t.Helper()

	idx, callGraph := buildQueryCallChainFixture()
	seed := mustFuncID(t, idx, "svc/handler.go", "HandleRequest")

	got, err := QueryCallChain(idx, callGraph, store.EventFactIndex{}, CallChainQuery{
		FunctionRef: seed,
		Direction:   "downstream",
		Depth:       2,
		MaxNodes:    2,
	})
	if err != nil {
		t.Fatalf("QueryCallChain() error = %v", err)
	}

	wantNodeIDs := []string{
		mustFuncID(t, idx, "svc/handler.go", "HandleRequest"),
		mustFuncID(t, idx, "svc/handler.go", "log"),
	}
	if gotNodeIDs := collectNodeIDs(got.Nodes); !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("QueryCallChain() node IDs = %#v, want %#v", gotNodeIDs, wantNodeIDs)
	}

	wantEdges := []string{"svc/handler.go#HandleRequest->svc/handler.go#log"}
	if gotEdges := collectEdgeKeys(got.Edges); !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("QueryCallChain() edges = %#v, want %#v", gotEdges, wantEdges)
	}
	if !got.Truncated {
		t.Fatal("QueryCallChain() Truncated = false, want true")
	}
}

func TestFlowGraphResultOrderingIsDeterministic(t *testing.T) {
	t.Helper()

	idx, callGraph, eventIdx := buildEventFlowFixture()

	first, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:        "user.created",
		ExpandPublishers: true,
		ExpandHandlers:   true,
		HandlerDepth:     1,
	})
	if err != nil {
		t.Fatalf("first QueryEventFlow() error = %v", err)
	}
	second, err := QueryEventFlow(idx, callGraph, eventIdx, EventFlowQuery{
		EventName:        "user.created",
		ExpandPublishers: true,
		ExpandHandlers:   true,
		HandlerDepth:     1,
	})
	if err != nil {
		t.Fatalf("second QueryEventFlow() error = %v", err)
	}

	if got, want := collectNodeIDs(first.Nodes), collectNodeIDs(second.Nodes); !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryEventFlow() node ordering mismatch: got %#v want %#v", got, want)
	}
	if got, want := collectTypedEdgeKeys(first.Edges), collectTypedEdgeKeys(second.Edges); !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryEventFlow() edge ordering mismatch: got %#v want %#v", got, want)
	}
}

func buildQueryCallChainFixture() (store.FileIndex, store.CallGraph) {
	return store.FileIndex{
			"svc/handler.go": {
				Path:     "svc/handler.go",
				Language: "go",
				Functions: []*store.FunctionDecl{
					{Name: "HandleRequest", Path: "svc/handler.go", Exported: true, LineStart: 10},
					{Name: "process", Path: "svc/handler.go", LineStart: 20},
					{Name: "doWork", Path: "svc/handler.go", LineStart: 30},
					{Name: "log", Path: "svc/handler.go", LineStart: 40},
				},
			},
		}, store.CallGraph{
			"svc/handler.go#HandleRequest": {"svc/handler.go#process", "svc/handler.go#log", "svc/handler.go#process"},
			"svc/handler.go#process":       {"svc/handler.go#doWork"},
			"svc/handler.go#doWork":        {"svc/handler.go#log", "svc/handler.go#log"},
		}
}

func buildEventFlowFixture() (store.FileIndex, store.CallGraph, store.EventFactIndex) {
	idx := store.FileIndex{
		"svc/events.go": {
			Path:     "svc/events.go",
			Language: "go",
			Functions: []*store.FunctionDecl{{
				Name:      "PublishUserCreated",
				Path:      "svc/events.go",
				Exported:  true,
				LineStart: 10,
			}},
		},
		"svc/registry.go": {
			Path:     "svc/registry.go",
			Language: "go",
			Functions: []*store.FunctionDecl{{
				Name:      "RegisterUserCreatedHandler",
				Path:      "svc/registry.go",
				Exported:  true,
				LineStart: 20,
			}},
		},
		"svc/handlers.go": {
			Path:     "svc/handlers.go",
			Language: "go",
			Functions: []*store.FunctionDecl{
				{Name: "HandleUserCreated", Path: "svc/handlers.go", Exported: true, LineStart: 30},
				{Name: "PersistAuditRecord", Path: "svc/handlers.go", LineStart: 40},
			},
		},
	}

	callGraph := store.CallGraph{
		"svc/registry.go#RegisterUserCreatedHandler": {"svc/handlers.go#HandleUserCreated"},
		"svc/handlers.go#HandleUserCreated":          {"svc/handlers.go#PersistAuditRecord"},
	}

	eventIdx := store.EventFactIndex{
		Version: "epic14/v1",
		Events: []*store.EventEntry{{
			EventName: "user.created",
			Publishers: []*store.EventFact{{
				EventName: "user.created",
				FuncID:    "svc/events.go#PublishUserCreated",
				Line:      12,
				Evidence:  "bus.Publish(user.created)",
			}},
			Handlers: []*store.EventFact{{
				EventName:  "user.created",
				HandlerRef: "svc/handlers.go#HandleUserCreated",
				FuncID:     "svc/handlers.go#HandleUserCreated",
				Line:       31,
				Evidence:   "case user.created:",
			}},
			Registrations: []*store.EventFact{{
				EventName:  "user.created",
				HandlerRef: "svc/handlers.go#HandleUserCreated",
				FuncID:     "svc/registry.go#RegisterUserCreatedHandler",
				Line:       22,
				Evidence:   "bus.Register(user.created, HandleUserCreated)",
			}},
		}},
	}

	return idx, callGraph, eventIdx
}

func buildMethodCallChainFixture() (store.FileIndex, store.CallGraph) {
	return store.FileIndex{
			"svc/service.go": {
				Path:     "svc/service.go",
				Language: "go",
				Functions: []*store.FunctionDecl{
					{Name: "HandleRequest", Receiver: "*Service", FunctionType: store.FunctionTypeMethod, Path: "svc/service.go", Exported: true, LineStart: 10},
					{Name: "process", Receiver: "*Service", FunctionType: store.FunctionTypeMethod, Path: "svc/service.go", LineStart: 20},
					{Name: "logAudit", Path: "svc/service.go", LineStart: 30},
				},
			},
		}, store.CallGraph{
			"svc/service.go#HandleRequest": {"svc/service.go#process"},
			"svc/service.go#process":       {"svc/service.go#logAudit"},
		}
}

func buildMethodEventFlowFixture() (store.FileIndex, store.CallGraph, store.EventFactIndex) {
	idx := store.FileIndex{
		"svc/subscriber.go": {
			Path:     "svc/subscriber.go",
			Language: "go",
			Functions: []*store.FunctionDecl{
				{Name: "PublishUserCreated", Path: "svc/subscriber.go", Exported: true, LineStart: 10},
				{Name: "HandleUserCreated", Receiver: "*Subscriber", FunctionType: store.FunctionTypeMethod, Path: "svc/subscriber.go", Exported: true, LineStart: 20},
				{Name: "persistAuditRecord", Path: "svc/subscriber.go", LineStart: 30},
			},
		},
	}

	callGraph := store.CallGraph{
		"svc/subscriber.go#HandleUserCreated": {"svc/subscriber.go#persistAuditRecord"},
	}

	eventIdx := store.EventFactIndex{
		Version: "epic14/v1",
		Events: []*store.EventEntry{{
			EventName: "user.created",
			Publishers: []*store.EventFact{{
				EventName: "user.created",
				FuncID:    "svc/subscriber.go#PublishUserCreated",
				Line:      11,
				Evidence:  "bus.Publish(user.created)",
			}},
			Handlers: []*store.EventFact{{
				EventName:  "user.created",
				HandlerRef: "svc/subscriber.go#*Subscriber#HandleUserCreated",
				FuncID:     "svc/subscriber.go#*Subscriber#HandleUserCreated",
				Line:       21,
				Evidence:   "case user.created:",
			}},
		}},
	}

	return idx, callGraph, eventIdx
}

func mustFuncID(t *testing.T, idx store.FileIndex, path string, name string) string {
	t.Helper()

	entry := idx[path]
	if entry == nil {
		t.Fatalf("missing file entry for %q", path)
	}
	for _, fn := range entry.Functions {
		if fn != nil && fn.Name == name {
			return store.FuncID(fn)
		}
	}
	t.Fatalf("missing function %q in %q", name, path)
	return ""
}

func mustMethodFuncID(t *testing.T, idx store.FileIndex, path string, receiver string, name string) string {
	t.Helper()

	entry := idx[path]
	if entry == nil {
		t.Fatalf("missing file entry for %q", path)
	}
	for _, fn := range entry.Functions {
		if fn != nil && fn.Name == name && fn.Receiver == receiver {
			return store.FuncID(fn)
		}
	}
	t.Fatalf("missing method %q with receiver %q in %q", name, receiver, path)
	return ""
}

func collectNodeIDs(nodes []*FlowNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		ids = append(ids, node.ID)
	}
	return ids
}

func collectEdgeKeys(edges []*FlowEdge) []string {
	keys := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		keys = append(keys, edge.From+"->"+edge.To)
	}
	return keys
}

func collectTypedEdgeKeys(edges []*FlowEdge) []string {
	keys := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		keys = append(keys, edge.From+"->"+edge.To+":"+edge.Kind)
	}
	return keys
}

func hasDisconnectedFunctionNode(result *FlowGraphResult) bool {
	if result == nil {
		return false
	}
	connected := make(map[string]bool)
	for _, edge := range result.Edges {
		if edge == nil {
			continue
		}
		connected[edge.From] = true
		connected[edge.To] = true
	}
	for _, node := range result.Nodes {
		if node == nil || node.Kind != "function" {
			continue
		}
		if !connected[node.ID] && len(result.Nodes) > 1 {
			return true
		}
	}
	return false
}

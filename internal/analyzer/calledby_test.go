package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func buildCalledByTestIndex() store.FileIndex {
	return store.FileIndex{
		"svc/handler.go": {
			Path:     "svc/handler.go",
			Language: "go",
			Functions: []*store.FunctionDecl{
				{
					Name: "HandleRequest", Path: "svc/handler.go",
					Exported: true, LineStart: 10,
					Calls: []*store.CallRef{
						{Name: "process", Path: "svc/handler.go", Ownership: store.OwnershipInternal, ResolvedTarget: "svc/handler.go#process"},
						{Name: "log", Path: "svc/handler.go", Ownership: store.OwnershipInternal, ResolvedTarget: "svc/handler.go#log"},
					},
				},
				{
					Name: "process", Path: "svc/handler.go",
					Exported: false, LineStart: 25,
					Calls: []*store.CallRef{
						{Name: "doWork", Path: "svc/handler.go", Ownership: store.OwnershipInternal, ResolvedTarget: "svc/handler.go#doWork"},
					},
				},
				{
					Name: "log", Path: "svc/handler.go",
					Exported: false, LineStart: 35,
				},
				{
					Name: "doWork", Path: "svc/handler.go",
					Exported: false, LineStart: 40,
					Calls: []*store.CallRef{
						{Name: "log", Path: "svc/handler.go", Ownership: store.OwnershipInternal, ResolvedTarget: "svc/handler.go#log"},
					},
				},
			},
		},
	}
}

func buildCalledByTestGraph() store.CallGraph {
	return store.CallGraph{
		"svc/handler.go#HandleRequest": {"svc/handler.go#process", "svc/handler.go#log"},
		"svc/handler.go#process":       {"svc/handler.go#doWork"},
		"svc/handler.go#doWork":        {"svc/handler.go#log"},
	}
}

func TestBuildCalledByIndexPopulatesReverseReferences(t *testing.T) {
	idx := buildCalledByTestIndex()
	graph := buildCalledByTestGraph()

	BuildCalledByIndex(idx, graph)

	// process is called by HandleRequest
	processFn := findFunction(t, idx, "svc/handler.go", "process")
	if len(processFn.CalledBy) != 1 {
		t.Fatalf("process.CalledBy = %d entries, want 1", len(processFn.CalledBy))
	}
	if processFn.CalledBy[0].Name != "HandleRequest" {
		t.Fatalf("process.CalledBy[0].Name = %q, want %q", processFn.CalledBy[0].Name, "HandleRequest")
	}
	if processFn.CalledBy[0].Path != "svc/handler.go" {
		t.Fatalf("process.CalledBy[0].Path = %q, want %q", processFn.CalledBy[0].Path, "svc/handler.go")
	}

	// log is called by HandleRequest AND doWork, sorted by name
	logFn := findFunction(t, idx, "svc/handler.go", "log")
	if len(logFn.CalledBy) != 2 {
		t.Fatalf("log.CalledBy = %d entries, want 2", len(logFn.CalledBy))
	}
	if logFn.CalledBy[0].Name != "HandleRequest" {
		t.Fatalf("log.CalledBy[0].Name = %q, want %q", logFn.CalledBy[0].Name, "HandleRequest")
	}
	if logFn.CalledBy[1].Name != "doWork" {
		t.Fatalf("log.CalledBy[1].Name = %q, want %q", logFn.CalledBy[1].Name, "doWork")
	}

	// HandleRequest has no callers
	handleFn := findFunction(t, idx, "svc/handler.go", "HandleRequest")
	if len(handleFn.CalledBy) != 0 {
		t.Fatalf("HandleRequest.CalledBy = %d entries, want 0", len(handleFn.CalledBy))
	}

	// doWork is called by process
	doWorkFn := findFunction(t, idx, "svc/handler.go", "doWork")
	if len(doWorkFn.CalledBy) != 1 {
		t.Fatalf("doWork.CalledBy = %d entries, want 1", len(doWorkFn.CalledBy))
	}
	if doWorkFn.CalledBy[0].Name != "process" {
		t.Fatalf("doWork.CalledBy[0].Name = %q, want %q", doWorkFn.CalledBy[0].Name, "process")
	}
}

func TestBuildCalledByIndexHandlesEmptyGraph(t *testing.T) {
	idx := store.FileIndex{
		"pkg/util.go": {
			Path: "pkg/util.go",
			Functions: []*store.FunctionDecl{
				{Name: "Helper", Path: "pkg/util.go", LineStart: 1},
			},
		},
	}

	BuildCalledByIndex(idx, store.CallGraph{})

	helperFn := findFunction(t, idx, "pkg/util.go", "Helper")
	if len(helperFn.CalledBy) != 0 {
		t.Fatalf("Helper.CalledBy = %d, want 0 with empty graph", len(helperFn.CalledBy))
	}
}

func TestBuildCalledByIndexHandlesEmptyFileIndex(t *testing.T) {
	idx := store.FileIndex{}
	graph := store.CallGraph{
		"svc/handler.go#HandleRequest": {"svc/handler.go#process"},
	}

	BuildCalledByIndex(idx, graph)
	// Should not panic
}

func TestBuildCalledByIndexHandlesMethodKeyMismatch(t *testing.T) {
	idx := store.FileIndex{
		"pkg/db/client.go": {
			Path: "pkg/db/client.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "Query", Path: "pkg/db/client.go",
					Receiver: "Client", FunctionType: store.FunctionTypeMethod,
					LineStart: 20,
				},
			},
		},
		"svc/handler.go": {
			Path: "svc/handler.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "Handle", Path: "svc/handler.go",
					LineStart: 10,
					Calls: []*store.CallRef{
						{Name: "Query", Path: "pkg/db/client.go", Ownership: store.OwnershipInternal, ResolvedTarget: "pkg/db/client.go#Query"},
					},
				},
			},
		},
	}

	graph := store.CallGraph{
		"svc/handler.go#Handle": {"pkg/db/client.go#Query"},
	}

	BuildCalledByIndex(idx, graph)

	queryFn := findFunction(t, idx, "pkg/db/client.go", "Query")
	if len(queryFn.CalledBy) != 1 {
		t.Fatalf("Query.CalledBy = %d entries, want 1 (method key mismatch not handled)", len(queryFn.CalledBy))
	}
	if queryFn.CalledBy[0].Name != "Handle" {
		t.Fatalf("Query.CalledBy[0].Name = %q, want %q", queryFn.CalledBy[0].Name, "Handle")
	}
}

func TestBuildCalledByIndexDeduplicatesCallers(t *testing.T) {
	idx := store.FileIndex{
		"svc/handler.go": {
			Path: "svc/handler.go",
			Functions: []*store.FunctionDecl{
				{Name: "Handle", Path: "svc/handler.go", LineStart: 10},
				{Name: "helper", Path: "svc/handler.go", LineStart: 20},
			},
		},
	}
	graph := store.CallGraph{
		"svc/handler.go#Handle": {"svc/handler.go#helper"},
	}

	BuildCalledByIndex(idx, graph)

	helperFn := findFunction(t, idx, "svc/handler.go", "helper")
	if len(helperFn.CalledBy) != 1 {
		t.Fatalf("helper.CalledBy = %d entries, want 1 (deduplicated)", len(helperFn.CalledBy))
	}
}

func TestBuildCalledByIndexPreservesCallerReceiver(t *testing.T) {
	idx := store.FileIndex{
		"svc/handler.go": {
			Path: "svc/handler.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "Process", Path: "svc/handler.go",
					Receiver: "Service", FunctionType: store.FunctionTypeMethod,
					LineStart: 10,
				},
				{Name: "Run", Path: "svc/handler.go", LineStart: 30},
			},
		},
	}
	graph := store.CallGraph{
		"svc/handler.go#Process": {"svc/handler.go#Run"},
	}

	BuildCalledByIndex(idx, graph)

	runFn := findFunction(t, idx, "svc/handler.go", "Run")
	if len(runFn.CalledBy) != 1 {
		t.Fatalf("Run.CalledBy = %d entries, want 1", len(runFn.CalledBy))
	}
	if runFn.CalledBy[0].Receiver != "Service" {
		t.Fatalf("Run.CalledBy[0].Receiver = %q, want %q", runFn.CalledBy[0].Receiver, "Service")
	}
}

func TestBuildCalledByIndexSampleRepo(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	cfg := generateConfigForTest(t)
	a := NewAnalyzer(cfg)

	idx, err := a.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	graph := LinkCalls(idx)
	BuildCalledByIndex(idx, graph)

	// ValidateToken is called by Handle and Middleware
	validateFn := findFunction(t, idx, "internal/auth/jwt.go", "ValidateToken")
	if validateFn == nil {
		t.Fatal("ValidateToken not found in index")
	}
	if len(validateFn.CalledBy) != 2 {
		t.Fatalf("ValidateToken.CalledBy = %d callers, want exactly 2", len(validateFn.CalledBy))
	}

	callerNames := make(map[string]bool)
	for _, caller := range validateFn.CalledBy {
		callerNames[caller.Name] = true
	}
	if !callerNames["Handle"] {
		t.Fatal("ValidateToken should be called by Handle")
	}
	if !callerNames["Middleware"] {
		t.Fatal("ValidateToken should be called by Middleware")
	}

	// Entry point should have no callers
	handleFn := findFunction(t, idx, "internal/api/handler.go", "Handle")
	if len(handleFn.CalledBy) != 0 {
		t.Fatalf("Handle.CalledBy = %d, want 0 (entry point)", len(handleFn.CalledBy))
	}
}

package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/scalaview/wikismit/pkg/store"
)

func TestBuildImportAliasMapUsesResolvedInternalImports(t *testing.T) {
	entry := parseFixtureEntry(t, "import_alias.go", "internal/alias/import_alias.go")
	markInternalImport(t, entry, "internal/auth", "internal/auth/jwt.go")
	markInternalImport(t, entry, "pkg/math", "pkg/math/math.go")
	markInternalImport(t, entry, "pkg/driver", "pkg/driver/driver.go")

	aliases := buildImportAliasMap(entry)
	if got := aliases.aliases["authpkg"]; got != "internal/auth" {
		t.Fatalf("authpkg alias = %q, want %q", got, "internal/auth")
	}
	if len(aliases.dots) != 1 || aliases.dots[0] != "pkg/math" {
		t.Fatalf("dot imports = %#v, want []string{\"pkg/math\"}", aliases.dots)
	}
	if _, ok := aliases.aliases["_"]; ok {
		t.Fatal("blank import should not appear in alias map")
	}
}

func TestLinkCallsResolvesSampleRepoCallGraph(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "sample_repo")
	cfg := generateConfigForTest(t)
	analyzer := NewAnalyzer(cfg)

	idx, err := analyzer.Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	depBefore := BuildDepGraph(idx)
	graph := LinkCalls(idx)
	depAfter := BuildDepGraph(idx)

	want := store.CallGraph{
		"internal/api/handler.go#Handle": {
			"internal/auth/jwt.go#ValidateToken",
			"pkg/logger/logger.go#Info",
		},
		"internal/auth/jwt.go#GenerateToken": {
			"pkg/errors/errors.go#New",
			"pkg/logger/logger.go#Info",
		},
		"internal/auth/jwt.go#ValidateToken": {
			"pkg/logger/logger.go#Info",
		},
		"internal/auth/middleware.go#Middleware": {
			"internal/auth/jwt.go#ValidateToken",
			"pkg/logger/logger.go#Info",
		},
		"internal/db/client.go#Query": {
			"pkg/errors/errors.go#New",
			"pkg/logger/logger.go#Info",
		},
	}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("LinkCalls() = %#v, want %#v", graph, want)
	}
	if !reflect.DeepEqual(depBefore, depAfter) {
		t.Fatalf("dep graph changed after linking: before=%#v after=%#v", depBefore, depAfter)
	}

	handleFn := findFunction(t, idx, "internal/api/handler.go", "Handle")
	handleValidate := findCall(t, handleFn, "auth", "ValidateToken")
	if handleValidate.ResolvedTarget != "internal/auth/jwt.go#ValidateToken" {
		t.Fatalf("Handle ValidateToken resolved target = %q, want %q", handleValidate.ResolvedTarget, "internal/auth/jwt.go#ValidateToken")
	}
	if handleValidate.Ownership != store.OwnershipInternal {
		t.Fatalf("Handle ValidateToken ownership = %d, want %d", handleValidate.Ownership, store.OwnershipInternal)
	}

	middlewareFn := findFunction(t, idx, "internal/auth/middleware.go", "Middleware")
	middlewareValidate := findCall(t, middlewareFn, "", "ValidateToken")
	if middlewareValidate.ResolvedTarget != "internal/auth/jwt.go#ValidateToken" {
		t.Fatalf("Middleware ValidateToken resolved target = %q, want %q", middlewareValidate.ResolvedTarget, "internal/auth/jwt.go#ValidateToken")
	}
}

func TestLinkCallsResolvesSamePackageCallsInSingleFile(t *testing.T) {
	entry := parseFixtureEntry(t, "same_package.go", "internal/samepkg/same_package.go")
	idx := store.FileIndex{entry.Path: entry}

	graph := LinkCalls(idx)
	want := store.CallGraph{
		"internal/samepkg/same_package.go#Public": {
			"internal/samepkg/same_package.go#helper",
		},
		"internal/samepkg/same_package.go#helper": {
			"internal/samepkg/same_package.go#helper2",
		},
	}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("LinkCalls() = %#v, want %#v", graph, want)
	}

	publicFn := entry.Functions[0]
	publicCall := findCall(t, publicFn, "", "helper")
	if publicCall.ResolvedTarget != "internal/samepkg/same_package.go#helper" {
		t.Fatalf("Public helper resolved target = %q, want %q", publicCall.ResolvedTarget, "internal/samepkg/same_package.go#helper")
	}
}

func TestLinkCallsResolvesMethodCalls(t *testing.T) {
	entry := parseFixtureEntry(t, "method_calls.go", "internal/methodcalls/method_calls.go")
	markInternalImport(t, entry, "pkg/db", "pkg/db/client.go")

	idx := store.FileIndex{
		entry.Path: entry,
		"pkg/db/client.go": {
			Path: "pkg/db/client.go",
			Functions: []*store.FunctionDecl{
				{
					Name:         "Query",
					Receiver:     "Client",
					FunctionType: store.FunctionTypeMethod,
					Path:         "pkg/db/client.go",
				},
			},
		},
	}

	graph := LinkCalls(idx)
	want := store.CallGraph{
		"internal/methodcalls/method_calls.go#Process": {
			"internal/methodcalls/method_calls.go#execute",
		},
		"internal/methodcalls/method_calls.go#example": {
			"pkg/db/client.go#Query",
		},
		"internal/methodcalls/method_calls.go#local": {
			"internal/methodcalls/method_calls.go#execute",
		},
	}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("LinkCalls() = %#v, want %#v", graph, want)
	}

	processFn := findFunction(t, idx, "internal/methodcalls/method_calls.go", "Process")
	processCall := findCall(t, processFn, "c", "execute")
	if processCall.ResolvedTarget != "internal/methodcalls/method_calls.go#execute" {
		t.Fatalf("Process execute resolved target = %q, want %q", processCall.ResolvedTarget, "internal/methodcalls/method_calls.go#execute")
	}

	exampleFn := findFunction(t, idx, "internal/methodcalls/method_calls.go", "example")
	exampleCall := findCall(t, exampleFn, "c", "Query")
	if exampleCall.ResolvedTarget != "pkg/db/client.go#Query" {
		t.Fatalf("example Query resolved target = %q, want %q", exampleCall.ResolvedTarget, "pkg/db/client.go#Query")
	}
}

func TestLinkCallsResolvesDotImportFallback(t *testing.T) {
	entry := parseFixtureEntry(t, "import_alias.go", "internal/alias/import_alias.go")
	markInternalImport(t, entry, "internal/auth", "internal/auth/jwt.go")
	markInternalImport(t, entry, "pkg/math", "pkg/math/math.go")
	markInternalImport(t, entry, "pkg/driver", "pkg/driver/driver.go")

	idx := store.FileIndex{
		entry.Path: entry,
		"internal/auth/jwt.go": {
			Path: "internal/auth/jwt.go",
			Functions: []*store.FunctionDecl{
				{Name: "ValidateToken", Path: "internal/auth/jwt.go"},
			},
		},
		"pkg/math/math.go": {
			Path: "pkg/math/math.go",
			Functions: []*store.FunctionDecl{
				{Name: "Add", Path: "pkg/math/math.go"},
			},
		},
		"pkg/driver/driver.go": {
			Path:      "pkg/driver/driver.go",
			Functions: []*store.FunctionDecl{},
		},
	}

	graph := LinkCalls(idx)
	want := store.CallGraph{
		"internal/alias/import_alias.go#example": {
			"internal/auth/jwt.go#ValidateToken",
			"pkg/math/math.go#Add",
		},
	}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("LinkCalls() = %#v, want %#v", graph, want)
	}

	exampleFn := findFunction(t, idx, "internal/alias/import_alias.go", "example")
	fmtCall := findCall(t, exampleFn, "fmt", "Println")
	if fmtCall.ResolvedTarget != "" {
		t.Fatalf("fmt.Println resolved target = %q, want empty", fmtCall.ResolvedTarget)
	}
	if fmtCall.Ownership != store.OwnershipExternal {
		t.Fatalf("fmt.Println ownership = %d, want %d", fmtCall.Ownership, store.OwnershipExternal)
	}

	addCall := findCall(t, exampleFn, "", "Add")
	if addCall.ResolvedTarget != "pkg/math/math.go#Add" {
		t.Fatalf("Add resolved target = %q, want %q", addCall.ResolvedTarget, "pkg/math/math.go#Add")
	}
}

func TestDetectCyclesReturnsEmptyReportForEmptyAndAcyclicGraphs(t *testing.T) {
	tests := []struct {
		name  string
		graph store.CallGraph
	}{
		{name: "empty", graph: store.CallGraph{}},
		{
			name: "acyclic",
			graph: store.CallGraph{
				"cycles.go#acyclic": {"cycles.go#helper"},
				"cycles.go#helper":  {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := DetectCycles(tt.graph)
			if len(report.Cycles) != 0 {
				t.Fatalf("DetectCycles() = %#v, want empty cycles", report.Cycles)
			}
		})
	}
}

func TestDetectCyclesReportsRecursiveAndMutualCycles(t *testing.T) {
	graph := store.CallGraph{
		"pkg/cycles/cycles.go#factorial": {"pkg/cycles/cycles.go#factorial"},
		"pkg/cycles/cycles.go#even":      {"pkg/cycles/cycles.go#odd"},
		"pkg/cycles/cycles.go#odd":       {"pkg/cycles/cycles.go#even"},
		"pkg/cycles/cycles.go#acyclic":   {"pkg/cycles/cycles.go#helper"},
		"pkg/cycles/cycles.go#helper":    {},
		"pkg/cycles/cycles.go#a":         {"pkg/cycles/cycles.go#b"},
		"pkg/cycles/cycles.go#b":         {"pkg/cycles/cycles.go#c"},
		"pkg/cycles/cycles.go#c":         {"pkg/cycles/cycles.go#a"},
	}

	report := DetectCycles(graph)
	want := [][]string{
		{"pkg/cycles/cycles.go#a", "pkg/cycles/cycles.go#b", "pkg/cycles/cycles.go#c", "pkg/cycles/cycles.go#a"},
		{"pkg/cycles/cycles.go#even", "pkg/cycles/cycles.go#odd", "pkg/cycles/cycles.go#even"},
		{"pkg/cycles/cycles.go#factorial", "pkg/cycles/cycles.go#factorial"},
	}
	if !reflect.DeepEqual(report.Cycles, want) {
		t.Fatalf("DetectCycles() = %#v, want %#v", report.Cycles, want)
	}
}

func parseFixtureEntry(t *testing.T, fixtureName string, relPath string) *store.FileEntry {
	t.Helper()

	fixturePath := filepath.Join("..", "..", "testdata", "fixtures", "golang", fixtureName)
	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", fixturePath, err)
	}

	parser, ok := Lookup(".go")
	if !ok {
		t.Fatal("Lookup(\".go\") = false, want registered Go parser")
	}
	entry, err := parser.ExtractSymbols(relPath, relPath, src)
	if err != nil {
		t.Fatalf("ExtractSymbols(%q) error = %v", fixtureName, err)
	}
	return entry
}

func markInternalImport(t *testing.T, entry *store.FileEntry, importPath string, resolvedPath string) {
	t.Helper()
	for _, imp := range entry.Imports {
		if imp.Path != importPath {
			continue
		}
		imp.Internal = true
		imp.ResolvedPath = resolvedPath
		return
	}
	t.Fatalf("import %q not found", importPath)
}

func findFunction(t *testing.T, idx store.FileIndex, filePath string, name string) *store.FunctionDecl {
	t.Helper()
	entry, ok := idx[filePath]
	if !ok {
		t.Fatalf("file %q not found in index", filePath)
	}
	for _, fn := range entry.Functions {
		if fn.Name == name {
			return fn
		}
	}
	t.Fatalf("function %q not found in %q", name, filePath)
	return nil
}

func findCall(t *testing.T, fn *store.FunctionDecl, receiver string, name string) *store.CallRef {
	t.Helper()
	for _, call := range fn.Calls {
		if call.Receiver == receiver && call.Name == name {
			return call
		}
	}
	t.Fatalf("call %q on receiver %q not found in function %q", name, receiver, fn.Name)
	return nil
}

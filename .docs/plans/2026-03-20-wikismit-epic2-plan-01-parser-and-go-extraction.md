# Epic 2 Parser and Go Extraction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the parser contracts and the deterministic Go single-file analyzer that produces spec-shaped `FileEntry` values.

**Architecture:** Establish the parser contract and store-shape alignment first so the Go parser does not need rework later. Then build the Go parser in TDD slices: bootstrap, content hashing, function extraction, type extraction, import extraction, and golden fixture verification.

**Tech Stack:** Go, `github.com/tree-sitter/go-tree-sitter`, `github.com/tree-sitter/tree-sitter-go/bindings/go`, `github.com/google/go-cmp/cmp`, standard `testing`.

---

### Task 1: Align parser and artifact contracts before extraction work

**Files:**
- Modify: `go.mod`
- Modify: `pkg/store/artifacts.go`
- Create: `internal/analyzer/parser.go`
- Test: `internal/analyzer/parser_test.go`

**Step 1: Add Phase 1 dependencies**

Run:

```bash
go get github.com/tree-sitter/go-tree-sitter github.com/tree-sitter/tree-sitter-go/bindings/go github.com/google/go-cmp/cmp
go mod tidy
```

Expected: module graph resolves without manual workarounds.

**Step 2: Write the failing parser contract tests**

Add tests that prove all three behaviors:

- `Register` stores the same parser for every declared extension
- duplicate extension registration returns a deterministic error or panics by design; pick one behavior and assert it explicitly
- `TypeDecl` carries `LineEnd`

Suggested test names:

```go
func TestRegisterIndexesAllExtensions(t *testing.T) {}
func TestRegisterRejectsDuplicateExtension(t *testing.T) {}
func TestTypeDeclCarriesLineEnd(t *testing.T) {}
```

**Step 3: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer ./pkg/store -run 'TestRegister|TestTypeDeclCarriesLineEnd' -v
```

Expected: FAIL because parser registry and `TypeDecl.LineEnd` do not exist yet.

**Step 4: Write the minimal contract implementation**

Implement:

- `type LanguageParser interface { Extensions() []string; ExtractSymbols(path string, src []byte) (store.FileEntry, error) }`
- package-level registry keyed by extension
- `Register(p LanguageParser)`
- `TypeDecl.LineEnd int    \`json:"line_end"\``
- `Import.ResolvedPath string \`json:"-"\``

**Step 5: Run tests to confirm GREEN**

Run:

```bash
go test ./internal/analyzer ./pkg/store -run 'TestRegister|TestTypeDeclCarriesLineEnd' -v
```

Expected: PASS.

### Task 2: Add a parser bootstrap test before writing tree-sitter code

**Files:**
- Create: `internal/analyzer/lang/golang.go`
- Create: `internal/analyzer/lang/golang_test.go`

**Step 1: Write the failing parser bootstrap test**

Add a test that creates the Go parser, parses `package main`, and asserts the root node kind is `source_file`.

Suggested test:

```go
func TestNewGoParserParsesMinimalSource(t *testing.T) {}
```

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/analyzer/lang -run TestNewGoParserParsesMinimalSource -v
```

Expected: FAIL because `newGoParser()` does not exist.

**Step 3: Write minimal implementation**

Implement a Go parser type plus:

```go
func newGoParser() *sitter.Parser
func (p *goParser) Extensions() []string
```

Use the official grammar binding, check `SetLanguage`, and ensure parser/tree objects are closed where required.

**Step 4: Run test to confirm GREEN**

Run:

```bash
go test ./internal/analyzer/lang -run TestNewGoParserParsesMinimalSource -v
```

Expected: PASS.

### Task 3: Add failing tests for content hashing and empty-file extraction

**Files:**
- Modify: `internal/analyzer/lang/golang_test.go`
- Modify: `internal/analyzer/lang/golang.go`

**Step 1: Write failing tests**

Add:

```go
func TestContentHashIsStableForSameBytes(t *testing.T) {}
func TestExtractSymbolsReturnsLanguageAndHashForEmptyGoFile(t *testing.T) {}
```

Assert that:

- repeated calls over identical bytes produce identical hash
- changed bytes produce different hash
- empty extraction returns `Language: "go"`, populated `ContentHash`, and empty symbol slices

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer/lang -run 'TestContentHash|TestExtractSymbolsReturnsLanguageAndHashForEmptyGoFile' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func contentHash(src []byte) string
func (p *goParser) ExtractSymbols(path string, src []byte) (store.FileEntry, error)
```

At this stage it is acceptable for `ExtractSymbols` to return language + hash + empty slices while preserving parse success.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Lock function extraction with a small fixture first

**Files:**
- Create: `testdata/fixtures/golang/simple.go`
- Create: `testdata/fixtures/golang/simple.golden.json`
- Modify: `internal/analyzer/lang/golang_test.go`
- Modify: `internal/analyzer/lang/golang.go`

**Step 1: Create the simple fixture**

Fixture requirements:

- one exported function
- one unexported function
- one exported struct
- one import

**Step 2: Write the failing golden test**

Add:

```go
func TestExtractSymbolsMatchesSimpleGolden(t *testing.T) {}
```

Test flow:

- read fixture bytes
- call `ExtractSymbols`
- marshal to JSON
- diff with `simple.golden.json` using `cmp.Diff`

**Step 3: Run test to confirm RED**

Run:

```bash
go test ./internal/analyzer/lang -run TestExtractSymbolsMatchesSimpleGolden -v
```

Expected: FAIL because extraction is incomplete.

**Step 4: Write minimal query-based function and import extraction**

Implement the smallest amount of tree-sitter query/traversal code needed to satisfy the simple fixture:

- functions with `Name`, `Signature`, `LineStart`, `LineEnd`, `Exported`
- imports with unquoted path text
- types sufficient for the fixture

**Step 5: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Extend extraction to complex fixtures and type coverage

**Files:**
- Create: `testdata/fixtures/golang/complex.go`
- Create: `testdata/fixtures/golang/complex.golden.json`
- Modify: `internal/analyzer/lang/golang_test.go`
- Modify: `internal/analyzer/lang/golang.go`

**Step 1: Add the failing complex golden test**

Add:

```go
func TestExtractSymbolsMatchesComplexGolden(t *testing.T) {}
```

Fixture requirements:

- interface type
- method on struct
- grouped imports
- multi-return function
- alias-like type

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/analyzer/lang -run TestExtractSymbolsMatchesComplexGolden -v
```

Expected: FAIL with missing or incorrect extracted fields.

**Step 3: Write minimal implementation to satisfy the complex fixture**

Complete extraction for:

- `method_declaration`
- `type_spec` kind detection
- correct type `LineEnd`
- grouped imports
- consistent signature reconstruction without function bodies

**Step 4: Run focused tests to confirm GREEN**

Run:

```bash
go test ./internal/analyzer/lang -v
```

Expected: PASS.

### Task 6: Register the Go parser and verify integration

**Files:**
- Modify: `internal/analyzer/lang/golang.go`
- Modify: `internal/analyzer/parser_test.go`

**Step 1: Write the failing integration test**

Add:

```go
func TestGoParserRegistersGoExtension(t *testing.T) {}
```

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/analyzer ./internal/analyzer/lang -run TestGoParserRegistersGoExtension -v
```

Expected: FAIL.

**Step 3: Implement `init()` registration**

Register the Go parser for `.go`.

**Step 4: Run parser package tests**

Run:

```bash
go test ./internal/analyzer ./internal/analyzer/lang -v
```

Expected: PASS.

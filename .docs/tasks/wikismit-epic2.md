# wikismit — Epic 2: Phase 1 — AST Analysis (Go)

**Status:** `todo`  
**Depends on:** Epic 1  
**Goal:** Given a Go repository path, produce `file_index.json` and `dep_graph.json` with correct symbol extraction and `file:line` references preserved. No LLM calls in this phase.  
**Spec refs:** §4 Phase 1, §5 Artifact Schemas, §7 Key Interfaces

---

## S2.1 — tree-sitter Go parser: symbol extraction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/golang.go`. Parse a single `.go` file using `go-tree-sitter` and return a `FileEntry` with all function declarations, type declarations, and imports. Preserve `line_start`/`line_end` for every symbol. Compute `content_hash` as `sha256` of file bytes.

**Acceptance criteria:**
- Given `testdata/fixtures/golang/simple.go`, output matches `simple.golden.json` exactly
- `line_start` / `line_end` are 1-indexed and correct
- `exported` is `true` only for uppercase identifiers
- `content_hash` is stable across repeated calls and changes when content changes

**Files to create:**
```
internal/analyzer/lang/golang.go
internal/analyzer/lang/golang_test.go
testdata/fixtures/golang/simple.go
testdata/fixtures/golang/simple.golden.json
testdata/fixtures/golang/complex.go
testdata/fixtures/golang/complex.golden.json
```

### Subtasks

#### S2.1.1 — Add tree-sitter dependency and language grammar

- Add `github.com/smacker/go-tree-sitter` to `go.mod`
- Import the Go grammar: `github.com/smacker/go-tree-sitter/golang`
- Write a `newGoParser() *sitter.Parser` helper that initialises the parser with the Go language grammar
- Verify the parser can parse a minimal `package main` file without error

#### S2.1.2 — Design `LanguageParser` interface

- Define in `internal/analyzer/parser.go`:
  ```go
  type LanguageParser interface {
      Extensions() []string
      ExtractSymbols(path string, src []byte) (store.FileEntry, error)
  }
  ```
- Define a `Registry` map `map[string]LanguageParser` (extension → parser) in the same file
- Implement `Register(p LanguageParser)` to populate the registry
- `golang.go` calls `Register` in its `init()` function

#### S2.1.3 — Extract function declarations

- Use a tree-sitter query to match `function_declaration` and `method_declaration` nodes:
  ```scheme
  (function_declaration name: (identifier) @name
    parameters: (parameter_list) @params
    result: (_)? @result) @func
  ```
- For each match extract:
  - `Name`: the `@name` capture text
  - `Signature`: reconstruct from `func` keyword + name + params + result (strip body)
  - `LineStart`: `node.StartPoint().Row + 1` (tree-sitter is 0-indexed)
  - `LineEnd`: `node.EndPoint().Row + 1`
  - `Exported`: `unicode.IsUpper(rune(name[0]))`
- Store in `FileEntry.Functions []store.FunctionDecl`

#### S2.1.4 — Extract type declarations

- Use a query to match `type_declaration` → `type_spec` nodes
- For each `type_spec`, determine `Kind`:
  - Underlying type is `struct_type` → `"struct"`
  - Underlying type is `interface_type` → `"interface"`
  - Otherwise → `"alias"`
- Extract `Name`, `LineStart`, `Exported`
- Store in `FileEntry.Types []store.TypeDecl`

#### S2.1.5 — Extract import declarations

- Match `import_declaration` and `import_spec` nodes
- For each import path (unquoted string value):
  - Store as `store.Import{Path: path, Internal: false}` initially (resolution to `internal: true` happens in S2.3)
- Handle both single `import "pkg"` and grouped `import ( "pkg1"; "pkg2" )` forms

#### S2.1.6 — Compute `content_hash`

- Implement `contentHash(src []byte) string`:
  - `hex.EncodeToString(sha256.Sum256(src)[:])`
- Set `FileEntry.ContentHash` before returning

#### S2.1.7 — Create test fixtures

- `testdata/fixtures/golang/simple.go`: one exported function, one unexported function, one exported struct, one import
- `testdata/fixtures/golang/simple.golden.json`: hand-written expected `FileEntry` JSON
- `testdata/fixtures/golang/complex.go`: interface, method on struct, multiple imports, multi-return function
- `testdata/fixtures/golang/complex.golden.json`: expected output
- Golden file test: parse fixture, marshal result to JSON, diff against golden with `github.com/google/go-cmp/cmp`

---

## S2.2 — Repository file traverser

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/analyzer.go`. Recursively walk the repository, apply `exclude_patterns`, dispatch to the registered language parser for each file, and return a complete `FileIndex`.

**Acceptance criteria:**
- All `.go` files in `testdata/sample_repo/` are present in the returned `FileIndex`
- Files matching `exclude_patterns` are absent
- Unrecognised extensions are silently skipped
- A parse failure logs a warning and continues traversal

**Files to create:**
```
internal/analyzer/analyzer.go
internal/analyzer/analyzer_test.go
testdata/sample_repo/           (synthetic Go project — see subtask S2.2.1)
```

### Subtasks

#### S2.2.1 — Design `testdata/sample_repo/`

Create a synthetic Go project with enough structure to exercise all pipeline phases:

```
testdata/sample_repo/
├── go.mod                          (module: github.com/wikismit/sample)
├── README.md                       (brief description)
├── cmd/
│   └── main.go                     (entry point, imports internal/api)
├── internal/
│   ├── api/
│   │   └── handler.go              (exported: Handler struct, Handle func — imports pkg/logger, internal/auth)
│   ├── auth/
│   │   ├── jwt.go                  (exported: GenerateToken, ValidateToken — imports pkg/logger, pkg/errors)
│   │   └── middleware.go           (exported: Middleware — imports internal/auth/jwt, pkg/logger)
│   └── db/
│       └── client.go               (exported: Client struct, Query func — imports pkg/logger, pkg/errors)
├── pkg/
│   ├── logger/
│   │   └── logger.go               (exported: Logger, New, Info, Error — imported by 4+ modules → shared)
│   └── errors/
│       └── errors.go               (exported: Wrap, New, Is — imported by 3+ modules → shared)
```

This gives: 4 non-shared modules (`cmd`, `api`, `auth`, `db`) + 2 shared modules (`logger`, `errors`).

#### S2.2.2 — Implement `Analyzer` struct

- Constructor: `NewAnalyzer(cfg config.AnalysisConfig) *Analyzer`
- Holds the parser registry and compiled glob patterns from `cfg.ExcludePatterns`
- Use `github.com/bmatcuk/doublestar/v4` for glob matching (supports `**` patterns)

#### S2.2.3 — Implement `Analyze(repoPath string) (store.FileIndex, error)`

- Use `filepath.WalkDir` to traverse the repo
- For each regular file:
  1. Check extension against registry; skip if not registered
  2. Check path against exclude patterns; skip if matched
  3. Read file bytes
  4. Call `parser.ExtractSymbols(path, src)`
  5. On parse error: log `WARN` with file path and error, continue
  6. Add entry to `FileIndex` keyed by relative path from `repoPath`
- Return completed `FileIndex`

#### S2.2.4 — Traverser unit tests

- Test: `testdata/sample_repo/` → all 7 `.go` files present in `FileIndex`
- Test: add a `.py` file to the fixture → it is not present in `FileIndex` (no Python parser registered yet)
- Test: add `internal/auth/jwt_test.go` → absent when `exclude_patterns: ["*_test.go"]`
- Test: introduce a malformed `.go` file in a temp dir → traversal completes, warning logged, other files present
- Test: `vendor/` directory with `exclude_patterns: ["vendor/**"]` → no vendor files in output

---

## S2.3 — Dependency graph construction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/dep_graph.go`. From the `FileIndex`, build a directed adjacency list of internal import dependencies. Resolve which imports are internal by comparing against the module path from `go.mod`.

**Acceptance criteria:**
- `internal/auth/jwt.go` importing `pkg/logger` → edge in dep_graph
- Third-party imports → `Internal: false` in `FileEntry.Imports`, no dep_graph edge
- Output is a valid directed graph for `testdata/sample_repo/`

**Files to create:**
```
internal/analyzer/dep_graph.go
internal/analyzer/dep_graph_test.go
```

### Subtasks

#### S2.3.1 — Read module path from `go.mod`

- Implement `readModulePath(repoPath string) (string, error)`:
  - Read `go.mod` using `golang.org/x/mod/modfile`
  - Return the `Module.Mod.Path` field (e.g. `github.com/wikismit/sample`)
- Cache the result on the `Analyzer` struct (one read per run)

#### S2.3.2 — Classify imports as internal or external

- After `ExtractSymbols` returns, iterate `FileEntry.Imports`
- If `import.Path` has the module path as a prefix → `Internal: true`
- Resolve the internal import path to a file path:
  - Strip the module path prefix
  - Append to `repoPath`
  - Try `{path}.go` then `{path}/` (directory → find the first `.go` file with a matching package declaration)
- Update `FileEntry.Imports` in-place with `Internal: true` and store the resolved file path

#### S2.3.3 — Build adjacency list

- Implement `BuildDepGraph(idx store.FileIndex) store.DepGraph`:
  - For each file in `idx`, for each `Import` with `Internal: true`:
    - Add edge: `depGraph[filePath] = append(depGraph[filePath], import.ResolvedPath)`
  - Files with no internal imports get an empty slice entry (not omitted from the map)
- Return the completed `DepGraph`

#### S2.3.4 — Dep graph unit tests

- Test: `testdata/sample_repo/` → `internal/auth/jwt.go` has edges to `pkg/logger/logger.go` and `pkg/errors/errors.go`
- Test: `pkg/logger/logger.go` has an empty edge list (no internal imports)
- Test: third-party import (`github.com/golang-jwt/jwt`) → not in any edge list
- Test: all files from `FileIndex` are present as keys in `DepGraph` (even with empty edge lists)

---

## S2.4 — Phase 1 orchestration and artifact write

**Status:** `todo`

**Description:**  
Wire S2.1–S2.3 into the Phase 1 step of `wikismit generate`. Run the analyzer, write both artifacts via `pkg/store`, log progress. Phase 1 must complete before Phase 2 begins.

**Acceptance criteria:**
- `wikismit generate --repo ./testdata/sample_repo` produces `artifacts/file_index.json` and `artifacts/dep_graph.json`
- Both files are valid JSON matching §5 schemas
- Running twice on unchanged repo produces byte-identical artifacts
- Elapsed time and file count logged at `INFO`

**Files to create:**
```
internal/analyzer/phase1.go
```

### Subtasks

#### S2.4.1 — Implement `RunPhase1(cfg *config.Config) error`

- Instantiate `Analyzer` with `cfg.Analysis`
- Call `analyzer.Analyze(cfg.RepoPath)` → `FileIndex`
- Call `BuildDepGraph(fileIndex)` → `DepGraph`
- Write both via `store.WriteFileIndex` and `store.WriteDepGraph`
- Return any error immediately (no partial writes)

#### S2.4.2 — Progress logging

- Before traversal: `INFO "Phase 1: analysing {repoPath}"`
- After traversal: `INFO "Phase 1 complete: {n} files indexed in {elapsed}"`
- If any files were skipped due to parse errors: `WARN "Phase 1: {n} files skipped due to parse errors"`

#### S2.4.3 — Wire into `generate` command

- In `cmd/wikismit/generate.go`, replace the `"not implemented"` stub with a call to `RunPhase1`
- Subsequent phases (Phase 2–5) remain as stubs for now
- End-to-end smoke test: run `wikismit generate --repo ./testdata/sample_repo --config ./config.yaml`, assert both artifact files exist and are non-empty

#### S2.4.4 — Idempotency test

- Run `RunPhase1` twice on `testdata/sample_repo/` in a temp artifacts dir
- Read both `file_index.json` outputs, assert they are byte-identical
- This guarantees `content_hash` and JSON serialisation are deterministic

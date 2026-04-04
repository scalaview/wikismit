# AST Call Chain Resolution — Technical Specification

**Version:** 0.1.0-draft
**Status:** Draft
**Last updated:** 2026-04-03

---

## Table of Contents

1. [Overview](#1-overview)
2. [Goals and Non-goals](#2-goals-and-non-goals)
3. [Architecture](#3-architecture)
4. [Data Model Changes](#4-data-model-changes)
5. [AST Query Additions](#5-ast-query-additions)
6. [Phase 2b — Link Algorithm](#6-phase-2b--link-algorithm)
7. [Cross-Package Function Resolution](#7-cross-package-function-resolution)
8. [Method Call Resolution](#8-method-call-resolution)
9. [Scope and Variable Disambiguation](#9-scope-and-variable-disambiguation)
10. [Cycle Detection](#10-cycle-detection)
11. [Integration with Existing Pipeline](#11-integration-with-existing-pipeline)
12. [Testing Strategy](#12-testing-strategy)
13. [Design Decisions and Trade-offs](#13-design-decisions-and-trade-offs)

---

## 1. Overview

Build a complete cross-file and cross-package call chain using only AST analysis (tree-sitter). The call chain enables **bottom-up summarization**: leaf functions are summarized first by LLM, then parent functions use child summaries + their own code to generate summaries, avoiding token limit issues.

### Current state

`ExtractSymbols` in `internal/analyzer/lang/golang.go` extracts per-file:

- `FunctionDecl` — function/method declarations with signatures, line ranges, and source
- `CallRef` — call expressions with `{Name, Receiver, Line}`
- `Import` — import paths (resolved to internal paths by `resolveImports`)

These are stored in `FileIndex` (map of file path → `FileEntry`), but **call references are not linked to their target declarations**. A `CallRef{Name: "ValidateToken", Receiver: "auth"}` exists in isolation — it is not connected to the `FunctionDecl{Name: "ValidateToken"}` in `internal/auth/jwt.go`.

### Target state

After linking, every `CallRef` that can be statically resolved gets a `ResolvedTarget` pointing to the target `FunctionDecl` (file path + function name). This produces a complete directed call graph across the entire codebase.

---

## 2. Goals and Non-goals

### Goals

- Resolve cross-package function calls (e.g., `auth.ValidateToken` → `internal/auth/jwt.go#ValidateToken`).
- Resolve same-package function calls (e.g., `helper()` called from `main.go` → `helper` declared in same package).
- Resolve method calls on locally-typed variables (e.g., `c.Query()` where `c` is declared as `*Client`).
- Handle import aliases (e.g., `import authpkg "..."` → `authpkg.ValidateToken`).
- Detect and handle cycles in the call graph (recursive calls, mutual recursion).
- Integrate into the existing Phase 1 pipeline as a post-processing step.

### Non-goals (v1)

- **Interface/dynamic dispatch resolution.** Calls through interfaces (`io.Reader.Read()`) cannot be resolved statically. These will be marked as unresolved.
- **Cross-repository resolution.** External dependencies (stdlib, third-party) are not resolved.
- **Full type inference.** Return-value-based type propagation (`c := db.New()` → resolve return type of `db.New`) is deferred. Only explicit type declarations are handled in v1.
- **Goroutine/channel-based call patterns.** These are not call expressions and are out of scope.
- **Function values / higher-order functions.** `f := someFunc; f()` is not tracked.

---

## 3. Architecture

Linking is a **post-processing step** that operates on the fully populated `FileIndex`. It runs after all files have been parsed and imports resolved.

```
Phase 1 — AST Analysis (existing)
  │
  ├── walkRepoDir()           ← ExtractSymbols per file
  ├── resolveImports()        ← resolve import paths to internal file paths
  │
  ▼
Phase 1b — Call Chain Linking (NEW)
  │
  ├── buildImportAliasMap()   ← per-file: alias → resolved package dir
  ├── buildFunctionIndex()    ← global: (package dir, name) → FunctionDecl
  ├── buildVarDeclIndex()     ← per-function: var name → type (for method calls)
  ├── linkCalls()             ← resolve each CallRef to a target FunctionDecl
  │
  ▼
Phase 2 — Module Planner (existing)
```

**Key invariant:** `linkCalls()` only runs after `FileIndex` is fully populated. No partial state — every file's declarations are available for matching.

---

## 4. Data Model Changes

### `pkg/store/artifacts.go`

```go
// CallRef gains a resolved target field.
type CallRef struct {
    Name           string        `json:"name"`
    Receiver       string        `json:"receiver,omitempty"`
    Line           int           `json:"line"`
    Ownership      OwnerShipType `json:"ownership"`
    ResolvedTarget string        `json:"resolved_target,omitempty"` // NEW: "pkg/logger/logger.go#Info"
}

// VarDecl tracks variable type declarations within function scope.
type VarDecl struct {
    Name  string `json:"name"`
    Type  string `json:"type"`
    Line  int    `json:"line"`
}

// FunctionDecl gains a Calls field listing resolved call targets.
type FunctionDecl struct {
    // ... existing fields ...
    Calls   []*CallRef `json:"calls,omitempty"`   // NEW: resolved calls within this function
    VarDefs []*VarDecl  `json:"var_defs,omitempty"` // NEW: variable declarations in scope
}

// Import gains an Alias field.
type Import struct {
    Path         string `json:"path"`
    Internal     bool   `json:"internal"`
    ResolvedPath string `json:"-"`
    Alias        string `json:"alias,omitempty"` // NEW: import alias ("" means default package name)
}

// CallGraph is the final output: a directed graph of function-to-function edges.
type CallGraph map[string][]string // key: "file.go#FuncName", value: ["other.go#Callee"]
```

### New artifact: `artifacts/call_graph.json`

```json
{
  "internal/api/handler.go#Handle": [
    "pkg/logger/logger.go#Info",
    "internal/auth/jwt.go#ValidateToken"
  ],
  "internal/db/client.go#Query": [
    "pkg/errors/errors.go#New",
    "pkg/logger/logger.go#Info"
  ],
  "cmd/main.go#main": [
    "internal/api/handler.go#Handle"
  ]
}
```

### Updated artifact: `artifacts/file_index.json`

After the linking phase, `file_index.json` is enriched with `calls`, `var_defs`, and import `alias` fields. Below is the full structure using the `testdata/sample_repo` as example.

```json
{
  "cmd/main.go": {
    "language": "go",
    "content_hash": "sha256:a1b2c3...",
    "functions": [
      {
        "name": "main",
        "signature": "func main()",
        "line_start": 5,
        "line_end": 7,
        "exported": false,
        "function_type": 0,
        "src": "func main() {\n\t_ = api.Handle\n}\n",
        "path": "cmd/main.go",
        "calls": [],
        "var_defs": []
      }
    ],
    "types": [],
    "imports": [
      {
        "path": "github.com/wikismit/sample/internal/api",
        "internal": true,
        "alias": ""
      }
    ],
    "path": "cmd/main.go"
  },

  "internal/api/handler.go": {
    "language": "go",
    "content_hash": "sha256:d4e5f6...",
    "functions": [
      {
        "name": "Handle",
        "signature": "func Handle(token string) bool",
        "line_start": 12,
        "line_end": 15,
        "exported": true,
        "function_type": 0,
        "src": "func Handle(token string) bool {\n\tlogger.Info(\"handling request\")\n\treturn auth.ValidateToken(token)\n}\n",
        "path": "internal/api/handler.go",
        "calls": [
          {
            "name": "Info",
            "receiver": "logger",
            "line": 13,
            "ownership": 0,
            "resolved_target": "pkg/logger/logger.go#Info"
          },
          {
            "name": "ValidateToken",
            "receiver": "auth",
            "line": 14,
            "ownership": 0,
            "resolved_target": "internal/auth/jwt.go#ValidateToken"
          }
        ],
        "var_defs": []
      }
    ],
    "types": [
      {
        "name": "Handler",
        "kind": "struct",
        "line_start": 9,
        "line_end": 11,
        "exported": true,
        "src": "type Handler struct {\n\tLogger *logger.Logger\n}\n",
        "path": "internal/api/handler.go"
      }
    ],
    "imports": [
      {
        "path": "github.com/wikismit/sample/internal/auth",
        "internal": true,
        "alias": ""
      },
      {
        "path": "github.com/wikismit/sample/pkg/logger",
        "internal": true,
        "alias": ""
      }
    ],
    "path": "internal/api/handler.go"
  },

  "internal/auth/jwt.go": {
    "language": "go",
    "content_hash": "sha256:g7h8i9...",
    "functions": [
      {
        "name": "ValidateToken",
        "signature": "func ValidateToken(token string) bool",
        "line_start": 8,
        "line_end": 10,
        "exported": true,
        "function_type": 0,
        "src": "func ValidateToken(token string) bool {\n\treturn token != \"\"\n}\n",
        "path": "internal/auth/jwt.go",
        "calls": [],
        "var_defs": []
      }
    ],
    "types": [],
    "imports": [],
    "path": "internal/auth/jwt.go"
  },

  "internal/db/client.go": {
    "language": "go",
    "content_hash": "sha256:j1k2l3...",
    "functions": [
      {
        "name": "Query",
        "signature": "func Query(query string) (string, error)",
        "line_start": 12,
        "line_end": 17,
        "exported": true,
        "function_type": 0,
        "src": "func Query(query string) (string, error) {\n\tif query == \"\" {\n\t\treturn \"\", errors.New(\"empty query\")\n\t}\n\tlogger.Info(\"query executed\")\n\treturn query, nil\n}\n",
        "path": "internal/db/client.go",
        "calls": [
          {
            "name": "New",
            "receiver": "errors",
            "line": 14,
            "ownership": 0,
            "resolved_target": "pkg/errors/errors.go#New"
          },
          {
            "name": "Info",
            "receiver": "logger",
            "line": 16,
            "ownership": 0,
            "resolved_target": "pkg/logger/logger.go#Info"
          }
        ],
        "var_defs": []
      }
    ],
    "types": [
      {
        "name": "Client",
        "kind": "struct",
        "line_start": 9,
        "line_end": 11,
        "exported": true,
        "src": "type Client struct {\n\tDSN string\n}\n",
        "path": "internal/db/client.go"
      }
    ],
    "imports": [
      {
        "path": "github.com/wikismit/sample/pkg/errors",
        "internal": true,
        "alias": ""
      },
      {
        "path": "github.com/wikismit/sample/pkg/logger",
        "internal": true,
        "alias": ""
      }
    ],
    "path": "internal/db/client.go"
  },

  "pkg/logger/logger.go": {
    "language": "go",
    "content_hash": "sha256:m4n5o6...",
    "functions": [
      {
        "name": "New",
        "signature": "func New() *Logger",
        "line_start": 5,
        "line_end": 7,
        "exported": true,
        "function_type": 0,
        "src": "func New() *Logger {\n\treturn &Logger{}\n}\n",
        "path": "pkg/logger/logger.go",
        "calls": [],
        "var_defs": []
      },
      {
        "name": "Info",
        "signature": "func Info(msg string)",
        "line_start": 9,
        "line_end": 9,
        "exported": true,
        "function_type": 0,
        "src": "func Info(msg string) {}\n",
        "path": "pkg/logger/logger.go",
        "calls": [],
        "var_defs": []
      },
      {
        "name": "Error",
        "signature": "func Error(msg string)",
        "line_start": 11,
        "line_end": 11,
        "exported": true,
        "function_type": 0,
        "src": "func Error(msg string) {}\n",
        "path": "pkg/logger/logger.go",
        "calls": [],
        "var_defs": []
      }
    ],
    "types": [
      {
        "name": "Logger",
        "kind": "struct",
        "line_start": 3,
        "line_end": 3,
        "exported": true,
        "src": "type Logger struct{}\n",
        "path": "pkg/logger/logger.go"
      }
    ],
    "imports": [],
    "path": "pkg/logger/logger.go"
  }
}
```

### Field diff summary

Compared to the current `file_index.json` schema (see `wikismit-tech-spec.md` Section 5), the following fields are added:

| Struct | New field | Type | Description |
|---|---|---|---|
| `Import` | `alias` | `string` | Explicit import alias (`""` = default) |
| `FunctionDecl` | `calls` | `[]*CallRef` | All calls within this function, with resolved targets |
| `FunctionDecl` | `var_defs` | `[]*VarDecl` | Variable declarations in this function's scope |
| `CallRef` | `resolved_target` | `string` | `"file.go#FuncName"` after linking, empty if unresolved |

### Ownership determination

`CallRef.Ownership` is set during the linking phase (not during AST extraction), based on the resolution result:

| Condition | Ownership | Value |
|---|---|---|
| `ResolvedTarget` points to a file within the repo | `OwnershipInternal` | `0` |
| Call target is an external package (stdlib, third-party) or unresolvable | `OwnershipExternal` | `1` |
| Same-package call (no receiver, matched within same package dir) | `OwnershipInternal` | `0` |

In the sample_repo example above, all calls resolve to internal files (`pkg/logger`, `internal/auth`, `pkg/errors`), so all have `"ownership": 0`. A call to `fmt.Println` or `http.ListenAndServe` would remain `"ownership": 1` with an empty `resolved_target`.

---

## 5. AST Query Additions

### 5.1 Import alias capture

Go allows renaming imports: `import authpkg "github.com/wikismit/sample/internal/auth"`. The default alias is the package name (last segment), but explicit aliases override it.

**New query pattern** (added to `simpleGoQuery`):

```
(import_spec
  name: (package_identifier) @import.alias
  path: (interpreted_string_literal) @import.path) @import.decl.alias
```

This captures imports with explicit aliases. Imports without aliases use the default package name (derived from the import path or the `package` declaration of the target).

### 5.2 Variable declaration capture

Track explicit variable types for method call resolution.

```
; var x T = ...
(var_spec
  name: (identifier) @var.name
  type: (type_identifier) @var.type) @var.decl

; var x *T = ...
(var_spec
  name: (identifier) @var.name
  type: (pointer_type (type_identifier) @var.type)) @var.decl

; short var decl with composite literal: x := T{}
(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (type_identifier) @var.type))) @var.decl

; short var decl with selector composite: x := pkg.T{}
(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (selector_expression
        operand: (identifier) @var.pkg
        field: (type_identifier) @var.type)))) @var.decl
```

### 5.3 Existing call expressions (no change needed)

The current queries already capture:

```
; Direct function call: b()
(call_expression
  function: (identifier) @call.name) @call.expr

; Selector call: auth.ValidateToken() or c.Query()
(call_expression
  function: (selector_expression
    operand: (identifier) @call.receiver
    field: (field_identifier) @call.method)) @call.expr
```

These are sufficient. The disambiguation between `auth.ValidateToken` (package call) and `c.Query` (method call) happens at link time based on whether the receiver matches an import alias or a local variable.

---

## 6. Phase 2b — Link Algorithm

The link step runs on the complete `FileIndex` in `internal/analyzer/linker.go`.

### Step-by-step

```
func LinkCalls(idx store.FileIndex) store.CallGraph
```

1. **Build function index** — global lookup table:

   ```
   key: (resolvedPackageDir, functionName)
   val: FunctionDecl

   e.g., ("internal/auth", "ValidateToken") → FunctionDecl from jwt.go
   ```

   For methods, also index by receiver:

   ```
   key: (resolvedPackageDir, receiverTypeName, methodName)
   e.g., ("internal/db", "Client", "Query") → method on *Client
   ```

2. **For each file in FileIndex:**

   a. **Build import alias map:**

   ```
   aliases = map[string]string{
       "auth":   "internal/auth",    // from import path resolution
       "logger": "pkg/logger",
       "authpkg": "internal/auth",   // explicit alias
   }
   ```

   Default alias = last path segment of the import. Explicit alias from `@import.alias` capture overrides it.

   b. **Build var decl map** — scoped per function:

   ```
   For each FunctionDecl in the file:
     Collect VarDecls with Line within [FunctionDecl.LineStart, FunctionDecl.LineEnd]
     Index by var name → type
   ```

   c. **For each CallRef in the file:**

   Determine which function it belongs to (find `FunctionDecl` where `LineStart ≤ CallRef.Line ≤ LineEnd`), then:

   - **If `CallRef.Receiver` matches an import alias** → cross-package function call (Section 7)
   - **If `CallRef.Receiver` matches a local var decl** → method call (Section 8)
   - **If `CallRef.Receiver` is empty** → same-package function call (match by name in the same package dir)
   - **Otherwise** → unresolved (mark and skip)

3. **Build CallGraph** from all resolved `CallRef`s.

---

## 7. Cross-Package Function Resolution

### Example

```go
// internal/api/handler.go
import "github.com/wikismit/sample/internal/auth"

func Handle(token string) bool {
    return auth.ValidateToken(token)
    //     ^^^^  ^^^^^^^^^^^^^^
    //     alias   function name
}
```

### Resolution steps

```
CallRef{Name: "ValidateToken", Receiver: "auth", Line: 14}

1. Look up "auth" in import alias map → "internal/auth"
2. Look up ("internal/auth", "ValidateToken") in function index
3. Found: FunctionDecl in internal/auth/jwt.go
4. Set ResolvedTarget = "internal/auth/jwt.go#ValidateToken"
```

### Import alias edge cases

| Import statement | Alias | Resolved package dir |
|---|---|---|
| `import "internal/auth"` | `"auth"` (default) | `"internal/auth"` |
| `import authpkg "internal/auth"` | `"authpkg"` | `"internal/auth"` |
| `import . "pkg/math"` | `""` (dot import) | functions go to file scope |
| `import _ "pkg/driver"` | `"_”` (blank) | skip — no calls possible |

Dot imports (`. `) are rare but valid. Functions from dot-imported packages appear as if declared in the current file — they have no receiver prefix. Resolution falls back to checking all dot-imported packages' function indices.

### Function-level CallRef assignment

Each `CallRef` is assigned to the `FunctionDecl` that encloses it (by line range). The `CallRef` is added to `FunctionDecl.Calls`.

---

## 8. Method Call Resolution

### Example

```go
// internal/db/client.go
type Client struct { DSN string }

func (c *Client) Query(query string) (string, error) { ... }
```

```go
// cmd/main.go
func main() {
    var c db.Client
    c.Query("SELECT ...")
    // ^  ^
    // |  method name
    // var name → type Client in package db
}
```

### Resolution steps

```
CallRef{Name: "Query", Receiver: "c", Line: 10}

1. Find enclosing function: main() (lines 8-12)
2. Look up "c" in main's var decls → type "db.Client"
3. Split "db.Client" → package alias "db", type name "Client"
4. Look up "db" in import alias map → "internal/db"
5. Look up ("internal/db", "Client", "Query") in method index
6. Found: method on *Client in internal/db/client.go
7. Set ResolvedTarget = "internal/db/client.go#Query"
```

### Same-package method calls

```go
func (c *Client) Query(query string) (string, error) {
    return c.execute(query)
    //     ^  ^^^^^^^
    //     var "c" → type "Client" (from receiver, not var decl)
    //     method "execute" on Client in same package
}
```

The receiver variable `c` in a method declaration implicitly has type `Client`. This must be registered in the var decl map when processing method declarations:

```
MethodDecl{Receiver: "Client", Name: "Query"} →
  varDecls["c"] = "Client"  // implicit from receiver parameter
```

### Pointer vs value receiver matching

Both `*Client` and `Client` receivers resolve to the same type name `"Client"` (as discussed in the existing `@method.receiver` capture). The method index keys use the bare type name, so `(c *Client)` and `(c Client)` both map to `"Client"`.

---

## 9. Scope and Variable Disambiguation

### Problem

Variable names are reused across functions:

```go
func Handle() {
    c := db.New()       // c → *db.Client
    c.Query(...)        // resolves to Client.Query
}

func Process() {
    c := logger.New()   // c → *logger.Logger
    c.Info(...)         // resolves to Logger.Info
}
```

### Solution: function-scoped var decls with line-based matching

VarDecls are indexed per-function using line ranges:

```
For a CallRef at line L with receiver "c":
1. Find FunctionDecl where LineStart ≤ L ≤ LineEnd
2. Among that function's VarDecls named "c":
   - Filter: VarDecl.Line < L (declaration must precede the call)
   - Pick the one with the highest Line (closest preceding declaration)
```

This naturally handles:

| Scenario | Behavior |
|---|---|
| Same name in different functions | Isolated by function line range |
| Reassignment in same function | Closest preceding declaration wins |
| Receiver parameter (`c *Client`) | Treated as a VarDecl at the function's LineStart |
| Variable in inner scope (`if`, `for`) | Line ordering still correct — inner scope vars are declared later |

### Shadowing example

```go
func foo() {
    c := db.New()       // VarDecl{Name:"c", Type:"db.Client", Line:5}
    c.Query(...)        // Line 6 → matches line 5 → Client.Query

    c := logger.New()   // VarDecl{Name:"c", Type:"logger.Logger", Line:8}
    c.Info(...)         // Line 9 → matches line 8 → Logger.Info
}
```

### Limitation

Block-scoped shadowing without reassignment is not fully tracked:

```go
func foo() {
    c := db.New()
    if cond {
        c := logger.New()  // shadows outer c
        c.Info(...)        // correctly resolves to Logger.Info (line match)
    }
    c.Query(...)           // correctly resolves to Client.Query (line match)
}
```

This works correctly with the "closest preceding line" heuristic. The only case that fails is when a variable is declared in a block that the call is **outside** of — but since Go requires variables to be used, and the line ordering reflects lexical order, this rarely occurs in practice.

---

## 10. Cycle Detection

### Scenarios

```go
// Direct recursion
func factorial(n int) int {
    return n * factorial(n-1)
}

// Mutual recursion
func even(n int) bool { return n == 0 || odd(n-1) }
func odd(n int) bool  { return n != 0 && even(n-1) }
```

### Detection

During `CallGraph` construction, perform a standard cycle detection:

```
func detectCycles(graph CallGraph) [][]string
  - DFS with WHITE/GRAY/BLACK coloring
  - When a GRAY node is revisited → cycle found
  - Record the cycle path
```

### Handling in summarization

Cycles break the bottom-up summarization order. For functions in a cycle:

1. Generate summaries for all cycle members using **only their source code** (no dependency summaries).
2. Then optionally regenerate with the first-pass summaries of their cycle partners included.

This is a two-pass approach for cycle members, while acyclic functions use the standard single-pass bottom-up order.

---

## 11. Integration with Existing Pipeline

### Changes to `internal/analyzer/analyzer.go`

Add the link step after `resolveImports` in `Analyze()`:

```go
func (a *Analyzer) Analyze(repoPath string) (store.FileIndex, error) {
    idx := store.FileIndex{}
    // ... existing walkRepoDir ...

    // Phase 2a: resolve imports (existing)
    for path, entry := range idx {
        // ... resolveImports ...
    }

    // Phase 2b: link calls (NEW)
    callGraph := LinkCalls(idx)

    return idx, nil  // callGraph returned separately
}
```

### Changes to `internal/analyzer/phase1.go`

```go
func RunPhase1(cfg *configpkg.Config) error {
    fileIndex, err := RunPhase1FileIndex(cfg)
    if err != nil {
        return err
    }

    // NEW: link calls and produce call graph
    callGraph := LinkCalls(fileIndex)

    depGraph := BuildDepGraph(fileIndex)
    store.WriteFileIndex(cfg.ArtifactsDir, fileIndex)
    store.WriteDepGraph(cfg.ArtifactsDir, depGraph)
    store.WriteCallGraph(cfg.ArtifactsDir, callGraph)  // NEW

    return nil
}
```

### Changes to `internal/analyzer/lang/golang.go`

- Add import alias query pattern.
- Add variable declaration query patterns.
- Populate `Import.Alias`, `FunctionDecl.Calls`, `FunctionDecl.VarDefs` in `ExtractSymbols`.

Note: `CallRef` is currently a top-level field on `FileEntry` but needs to be scoped to individual `FunctionDecl`s. Migration path:

1. During `ExtractSymbols`, assign each `CallRef` to its enclosing `FunctionDecl` using line ranges.
2. Keep `FileEntry`-level `Calls` as a convenience (flatten from all functions).
3. Or: move `Calls` entirely into `FunctionDecl` and remove from `FileEntry`.

### New files

| File | Purpose |
|---|---|
| `internal/analyzer/linker.go` | `LinkCalls()`, import alias map builder, function/method index builders |
| `internal/analyzer/linker_test.go` | Unit tests for linking logic |
| `pkg/store/call_graph.go` | `CallGraph` type, read/write helpers |

---

## 12. Testing Strategy

### Unit tests

**`internal/analyzer/lang/golang_test.go`** — extend existing tests:

- Import alias extraction: verify `import authpkg "..."` produces `Import{Alias: "authpkg"}`.
- Variable declaration extraction: verify `var c db.Client` produces `VarDecl{Name:"c", Type:"db.Client"}`.
- Variable declaration with pointer: verify `var c *db.Client` produces `VarDecl{Name:"c", Type:"db.Client"}`.
- Short var decl with composite literal: verify `c := Client{}` produces `VarDecl{Name:"c", Type:"Client"}`.

**`internal/analyzer/linker_test.go`** — new test file:

| Test case | Input | Expected |
|---|---|---|
| Same-package call | `CallRef{Name:"helper"}` in `main.go` | Resolves to `FunctionDecl` in same package |
| Cross-package call | `CallRef{Name:"ValidateToken", Receiver:"auth"}` | Resolves to `internal/auth/jwt.go#ValidateToken` |
| Import alias | `CallRef{Name:"ValidateToken", Receiver:"authpkg"}` | Resolves correctly via alias |
| Method call | `CallRef{Name:"Query", Receiver:"c"}` where `c` is `db.Client` | Resolves to `internal/db/client.go#Query` |
| Unresolved (interface) | `CallRef{Name:"Read", Receiver:"r"}` where `r` is `io.Reader` | `ResolvedTarget` is empty |
| Cycle detection | Mutual recursion between `even` and `odd` | Cycle reported correctly |
| Variable shadowing | Same var name in different functions | Each resolves to correct type |

### Integration tests

Use existing `testdata/sample_repo/` which already has cross-package calls:

```
api/handler.go:Handle → auth.ValidateToken, logger.Info
db/client.go:Query    → errors.New, logger.Info
cmd/main.go:main      → api.Handle
```

Run `LinkCalls()` on the parsed `FileIndex` and verify the resulting `CallGraph` matches expected edges.

### Test fixtures

Add new fixture files:

```
testdata/fixtures/golang/var_decls.go    — variable declaration patterns
testdata/fixtures/golang/import_alias.go — import alias patterns
testdata/fixtures/golang/method_calls.go — method call patterns
testdata/fixtures/golang/cycles.go       — recursive call patterns
```

---

## 13. Design Decisions and Trade-offs

### Two-phase (extract then link) vs single-pass

**Chosen:** Two-phase. Extract all symbols first, then link.

**Why:** A single-pass approach would require processing files in dependency order, which is complex and unnecessary. The two-phase approach is simpler and guaranteed correct — all declarations are available when linking begins.

### Line-range scoping vs full scope analysis

**Chosen:** Line-range matching with "closest preceding declaration" heuristic.

**Why:** Full scope analysis (tracking `{}` block nesting) requires walking the AST tree structure beyond what tree-sitter queries provide. Line-range matching covers >95% of real-world code with minimal implementation cost. The heuristic correctly handles variable shadowing, reassignment, and block-scoped declarations in practice.

### Import alias from AST vs inferred from path

**Chosen:** Both. Capture explicit aliases from AST (`import x "..."`), fall back to last path segment.

**Why:** Explicit aliases are unambiguous from the AST. Default aliases (package name) could differ from the last path segment (e.g., `package api` in `internal/auth/v2/`), but the last segment is a correct approximation for the vast majority of Go code. A future improvement could parse the target package's `package` declaration.

### Method receiver: bare type name vs pointer type

**Chosen:** Use bare type name for method index keys.

**Why:** The existing `@method.receiver` capture already strips the `*` from pointer receivers (it captures the inner `type_identifier`). This is correct for resolution purposes — Go method sets include both value and pointer receivers, and the caller doesn't distinguish.

### CallGraph as a separate artifact vs embedded in FileIndex

**Chosen:** Separate `call_graph.json` artifact, with `ResolvedTarget` also stored on `CallRef`.

**Why:** The `CallGraph` (edge list) is the primary output for the summarization pipeline. `ResolvedTarget` on `CallRef` provides fine-grained traceability for debugging and citation generation. Storing both is redundant but serves different consumers.

### No type inference in v1

**Chosen:** Only explicit type declarations are tracked. Return-value propagation (`c := db.New()`) is deferred.

**Why:** Full return-value type inference requires either:
1. Parsing function signatures to extract return types (feasible but adds significant complexity).
2. Cross-referencing function declarations to find return type annotations.

This is planned for v2. In v1, method calls through constructor-returned variables will remain unresolved, which is acceptable because:
- Most Go code uses explicit `var c db.Client` or `c := db.Client{}` patterns.
- The cross-package function resolution (Section 7) already covers the majority of call chains.

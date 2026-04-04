# wikismit — Epic 10b: Call Chain — AST Extraction Enhancement

**Status:** `todo`
**Depends on:** Epic 10a (data model types must exist)
**Goal:** Extend `ExtractSymbols` in `golang.go` to capture import aliases, variable declarations, and scope `CallRef`s to their enclosing `FunctionDecl`. This provides the raw data that the linker (Epic 10c) will resolve.
**Spec refs:** `ast-call-chain-resolution.md` Sections 5 (AST Query Additions), 9 (Scope and Variable Disambiguation)

---

## S10b.1 — Import alias extraction

**Status:** `todo`

**Description:**
Add a new tree-sitter query pattern to capture explicit import aliases (`import authpkg "..."`) and populate `Import.Alias`.

**Acceptance criteria:**
- `import authpkg "github.com/foo/bar"` → `Import{Path: "github.com/foo/bar", Alias: "authpkg"}`
- `import "github.com/foo/bar"` → `Import{Path: "github.com/foo/bar", Alias: ""}` (no alias)
- `import . "github.com/foo/bar"` → `Import{Path: "github.com/foo/bar", Alias: ""}` (dot import, alias empty)
- `import _ "github.com/foo/bar"` → `Import{Path: "github.com/foo/bar", Alias: "_"}` (blank import)

**Files to modify:**
```
internal/analyzer/lang/golang.go
internal/analyzer/lang/golang_test.go
testdata/fixtures/golang/import_alias.go
```

### Subtasks

#### S10b.1.1 — Add import alias query pattern

Add to `simpleGoQuery`:
```
(import_spec
  name: (package_identifier) @import.alias
  path: (interpreted_string_literal) @import.path) @import.decl.alias
```

This captures imports with explicit aliases. The existing `@import.decl` pattern continues to capture imports without aliases.

#### S10b.1.2 — Handle alias in match processing

In the `@import.decl.alias` match branch:
- Extract `@import.alias` text as the alias value
- Create `Import` entry with `Alias` field populated

Ensure the existing `@import.decl` match still works for non-aliased imports. Handle deduplication: if both `@import.decl` and `@import.decl.alias` match the same import spec, only one `Import` entry should appear.

#### S10b.1.3 — Create import alias fixture

Create `testdata/fixtures/golang/import_alias.go`:
```go
package alias

import (
    "fmt"
    authpkg "internal/auth"
    . "pkg/math"
    _ "pkg/driver"
)
```

#### S10b.1.4 — Unit tests

- Test fixture produces: `fmt` (Alias:""), `internal/auth` (Alias:"authpkg"), `pkg/math` (Alias:"", dot import), `pkg/driver` (Alias:"_")
- Test that alias field is `omitempty` — no `"alias"` key in JSON when empty

---

## S10b.2 — Variable declaration extraction

**Status:** `todo`

**Description:**
Add tree-sitter query patterns to capture variable type declarations within functions. These enable method call resolution in the linker.

**Acceptance criteria:**
- `var c db.Client` → `VarDecl{Name:"c", Type:"db.Client", Line:5}`
- `var c *db.Client` → `VarDecl{Name:"c", Type:"db.Client", Line:5}` (pointer stripped)
- `c := Client{}` → `VarDecl{Name:"c", Type:"Client", Line:6}`
- `c := db.Client{}` → `VarDecl{Name:"c", Type:"db.Client", Line:7}` (pkg.Type format)
- Variable declarations outside functions (package-level `var`) are captured but not scoped

**Files to modify:**
```
internal/analyzer/lang/golang.go
internal/analyzer/lang/golang_test.go
testdata/fixtures/golang/var_decls.go
```

### Subtasks

#### S10b.2.1 — Add var_spec query patterns

Add to `simpleGoQuery`:

```
; var x T
(var_spec
  name: (identifier) @var.name
  type: (type_identifier) @var.type) @var.decl

; var x *T
(var_spec
  name: (identifier) @var.name
  type: (pointer_type (type_identifier) @var.type)) @var.decl
```

#### S10b.2.2 — Add short_var_declaration query patterns

```
; x := T{}
(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (type_identifier) @var.type))) @var.decl

; x := pkg.T{}
(short_var_declaration
  left: (expression_list
    (identifier) @var.name)
  right: (expression_list
    (composite_literal
      type: (selector_expression
        operand: (identifier) @var.pkg
        field: (type_identifier) @var.type)))) @var.decl
```

#### S10b.2.3 — Handle var.decl matches in ExtractSymbols

In the match processing loop, add a `@var.decl` branch:
- Extract `@var.name` and `@var.type` text
- If `@var.pkg` is present, format type as `{pkg}.{type}` (e.g., `db.Client`)
- For pointer types (`pointer_type` wrapper), strip the `*` and use the inner type name
- Compute line number
- Store as `VarDecl` in a temporary collection (will be scoped to functions in S10b.3)

#### S10b.2.4 — Create var_decls fixture

Create `testdata/fixtures/golang/var_decls.go`:
```go
package vardecls

import "pkg/db"

func example() {
    var a db.Client
    var b *db.Client
    c := db.Client{}
    d := Client{}
}
```

#### S10b.2.5 — Unit tests

- Parse `var_decls.go` fixture and verify all 4 `VarDecl` entries are extracted with correct `Name`, `Type`, `Line`

---

## S10b.3 — Scope CallRefs and VarDecls to FunctionDecls

**Status:** `todo`

**Description:**
After extracting all symbols from a file, assign each `CallRef` to its enclosing `FunctionDecl` using line ranges. Similarly, scope `VarDecl`s to their enclosing functions. Populate `FunctionDecl.Calls` and `FunctionDecl.VarDefs`.

**Acceptance criteria:**
- A `CallRef` at line 14 is assigned to the `FunctionDecl` where `LineStart ≤ 14 ≤ LineEnd`
- A `VarDecl` at line 5 is assigned to the `FunctionDecl` where `LineStart ≤ 5 ≤ LineEnd`
- `CallRef`s outside any function (package-level init-like code) remain unassigned
- For method declarations, the receiver parameter variable is added as an implicit `VarDecl` at `LineStart`

**Files to modify:**
```
internal/analyzer/lang/golang.go
```

### Subtasks

#### S10b.3.1 — Implement scope assignment function

```go
func scopeToFunctions(calls []*store.CallRef, varDefs []store.VarDecl, functions []store.FunctionDecl) {
    for i := range functions {
        fn := &functions[i]
        for _, call := range calls {
            if call.Line >= fn.LineStart && call.Line <= fn.LineEnd {
                fn.Calls = append(fn.Calls, call)
            }
        }
        for _, vd := range varDefs {
            if vd.Line >= fn.LineStart && vd.Line <= fn.LineEnd {
                fn.VarDefs = append(fn.VarDefs, &vd)
            }
        }
    }
}
```

#### S10b.3.2 — Add receiver parameter as implicit VarDecl

For method declarations, the receiver variable has an implicit type. When processing a `@method.decl` match:
- Extract the receiver parameter name (currently not captured — need a new capture `@method.receiver.name`)
- Add `VarDecl{Name: receiverParamName, Type: receiverTypeName, Line: LineStart}` to the function's `VarDefs`

This requires adding a query capture for the receiver parameter identifier:
```
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier) @method.receiver.name
      type: (type_identifier) @method.receiver))
  name: (field_identifier) @method.name) @method.decl
```

#### S10b.3.3 — Wire scope assignment into ExtractSymbols

After all matches are processed (functions, calls, var decls are collected), call `scopeToFunctions` before returning the `FileEntry`.

#### S10b.3.4 — Integration test with sample_repo

- Run `ExtractSymbols` on `testdata/sample_repo/internal/api/handler.go`
- Verify `Handle` function's `Calls` contains the `logger.Info` and `auth.ValidateToken` `CallRef`s
- Verify `Handle` function's `VarDefs` is empty (no var declarations in that function)
- Run on a file with var declarations and verify scoping

---

## S10b.4 — Move FileEntry-level Calls into FunctionDecl

**Status:** `todo`

**Description:**
Decide on the canonical location for `CallRef` data. Currently `CallRef`s are stored at `FileEntry` level. After this epic, they should live in `FunctionDecl.Calls`. Decide whether to keep the `FileEntry`-level field for convenience or remove it.

**Acceptance criteria:**
- `FunctionDecl.Calls` is populated for all functions
- Decision documented: either remove `FileEntry`-level calls field or keep as flattened convenience accessor
- All existing tests updated to use the new field location

**Files to modify:**
```
pkg/store/artifacts.go
internal/analyzer/lang/golang.go
internal/analyzer/lang/golang_test.go
```

### Subtasks

#### S10b.4.1 — Remove or deprecate FileEntry-level calls

Option A (recommended): Remove `Calls` from `FileEntry` entirely. All call data lives in `FunctionDecl.Calls`. Downstream code iterates functions to get calls.

Option B: Keep `FileEntry.Calls` as a flattened view, populated after scoping for backward compatibility.

#### S10b.4.2 — Update existing tests

- `golang_test.go`: update golden files and assertions to check `FunctionDecl.Calls` instead of `FileEntry.Calls`
- `parser_test.go`: update if it references `FileEntry`-level calls

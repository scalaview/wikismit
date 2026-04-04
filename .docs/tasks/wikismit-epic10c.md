# wikismit — Epic 10c: Call Chain — Linker Implementation

**Status:** `todo`
**Depends on:** Epic 10a (data model), Epic 10b (AST extraction with scoped calls/var decls/import aliases)
**Goal:** Implement the `LinkCalls` function that resolves every `CallRef` to a target `FunctionDecl` across the codebase. This is the core algorithm that produces the `CallGraph`.
**Spec refs:** `ast-call-chain-resolution.md` Sections 6-9 (Link Algorithm, Cross-Package Resolution, Method Call Resolution, Scope/Disambiguation)

---

## S10c.1 — Build global function and method index

**Status:** `todo`

**Description:**
Create lookup indices from the `FileIndex` that enable fast resolution of call targets.

**Acceptance criteria:**
- Function index: `(packageDir, functionName) → []FunctionDecl` — supports same-name functions in different files of the same package
- Method index: `(packageDir, receiverType, methodName) → FunctionDecl`
- Package dir is derived from each file's `Path` field (directory portion)
- Index is built once from the complete `FileIndex` before linking begins

**Files to create:**
```
internal/analyzer/linker.go
```

### Subtasks

#### S10c.1.1 — Define index types

```go
type functionKey struct {
    PackageDir string
    Name       string
}

type methodKey struct {
    PackageDir   string
    ReceiverType string
    Name         string
}

type linkIndex struct {
    functions map[functionKey][]*store.FunctionDecl
    methods   map[methodKey]*store.FunctionDecl
    fileIndex store.FileIndex
}
```

#### S10c.1.2 — Implement `buildLinkIndex`

```go
func buildLinkIndex(idx store.FileIndex) *linkIndex {
    li := &linkIndex{
        functions: make(map[functionKey][]*store.FunctionDecl),
        methods:   make(map[methodKey]*store.FunctionDecl),
        fileIndex: idx,
    }
    for _, entry := range idx {
        pkgDir := filepath.Dir(entry.Path)
        for i := range entry.Functions {
            fn := &entry.Functions[i]
            if fn.FunctionType == store.FunctionTypeMethod && fn.Receiver != "" {
                li.methods[methodKey{pkgDir, fn.Receiver, fn.Name}] = fn
            } else {
                li.functions[functionKey{pkgDir, fn.Name}] = append(
                    li.functions[functionKey{pkgDir, fn.Name}], fn)
            }
        }
    }
    return li
}
```

#### S10c.1.3 — Unit test for index building

- Build `linkIndex` from `testdata/sample_repo` FileIndex
- Verify `("internal/auth", "ValidateToken")` resolves correctly
- Verify `("internal/db", "Client", "Query")` resolves to the method on Client
- Verify `("pkg/logger", "Info")` resolves to the package-level function

---

## S10c.2 — Build per-file import alias map

**Status:** `todo`

**Description:**
For each file, construct a map from import alias → resolved package directory. This map is used to resolve `CallRef.Receiver` to a target package.

**Acceptance criteria:**
- `import authpkg "internal/auth"` → `{"authpkg": "internal/auth"}`
- `import "internal/auth"` → `{"auth": "internal/auth"}` (default alias = last path segment)
- `import . "pkg/math"` → dot-import recorded separately
- `import _ "pkg/driver"` → skipped (blank import, no calls possible)
- Alias map is built fresh for each file during linking

**Files to modify:**
```
internal/analyzer/linker.go
```

### Subtasks

#### S10c.2.1 — Implement `buildImportAliasMap`

```go
type importAliasMap struct {
    aliases map[string]string // alias → package dir
    dots    []string          // package dirs from dot imports
}

func buildImportAliasMap(entry *store.FileEntry) *importAliasMap {
    am := &importAliasMap{
        aliases: make(map[string]string),
    }
    for _, imp := range entry.Imports {
        if !imp.Internal || imp.ResolvedPath == "" {
            continue
        }
        pkgDir := filepath.Dir(imp.ResolvedPath)

        if imp.Alias == "_" {
            continue // blank import
        }
        if imp.Alias == "." {
            am.dots = append(am.dots, pkgDir)
            continue
        }
        alias := imp.Alias
        if alias == "" {
            // default: last segment of import path
            parts := strings.Split(imp.Path, "/")
            alias = parts[len(parts)-1]
        }
        am.aliases[alias] = pkgDir
    }
    return am
}
```

#### S10c.2.2 — Unit test for import alias map

Test with `testdata/sample_repo/internal/api/handler.go`:
- `{"auth": "internal/auth", "logger": "pkg/logger"}`

Test with a synthetic entry that has:
- Explicit alias → `{"authpkg": "internal/auth"}`
- Dot import → recorded in `dots`
- Blank import → skipped
- External import → skipped

---

## S10c.3 — Resolve cross-package function calls

**Status:** `todo`

**Description:**
When `CallRef.Receiver` matches an import alias, look up the target function in the function index by `(resolvedPkgDir, callName)`.

**Acceptance criteria:**
- `CallRef{Name:"ValidateToken", Receiver:"auth"}` in `handler.go` → `ResolvedTarget: "internal/auth/jwt.go#ValidateToken"`
- `CallRef{Name:"Info", Receiver:"logger"}` → `ResolvedTarget: "pkg/logger/logger.go#Info"`
- Unresolvable calls (function not found in target package) → `ResolvedTarget: ""`, `Ownership: OwnershipExternal`
- When multiple files in same package declare the same function name, pick the first match and log a warning

**Files to modify:**
```
internal/analyzer/linker.go
```

### Subtasks

#### S10c.3.1 — Implement cross-package resolution

```go
func (li *linkIndex) resolveCrossPackage(call *store.CallRef, am *importAliasMap) bool {
    pkgDir, ok := am.aliases[call.Receiver]
    if !ok {
        return false
    }
    candidates, ok := li.functions[functionKey{pkgDir, call.Name}]
    if !ok || len(candidates) == 0 {
        return false
    }
    call.ResolvedTarget = candidates[0].Path + "#" + candidates[0].Name
    call.Ownership = store.OwnershipInternal
    return true
}
```

#### S10c.3.2 — Resolve same-package calls

When `CallRef.Receiver` is empty and `CallRef.Name` matches a function in the same package:

```go
func (li *linkIndex) resolveSamePackage(call *store.CallRef, currentPkgDir string) bool {
    candidates, ok := li.functions[functionKey{currentPkgDir, call.Name}]
    if !ok || len(candidates) == 0 {
        return false
    }
    call.ResolvedTarget = candidates[0].Path + "#" + candidates[0].Name
    call.Ownership = store.OwnershipInternal
    return true
}
```

#### S10c.3.3 — Handle dot imports

When `CallRef.Receiver` is empty and same-package resolution fails, try each dot-imported package's function index as fallback.

#### S10c.3.4 — Unit tests

Test with `testdata/sample_repo`:
- `handler.go`: `auth.ValidateToken` → `internal/auth/jwt.go#ValidateToken`
- `handler.go`: `logger.Info` → `pkg/logger/logger.go#Info`
- `client.go`: `errors.New` → `pkg/errors/errors.go#New`
- `client.go`: `logger.Info` → `pkg/logger/logger.go#Info`

---

## S10c.4 — Resolve method calls

**Status:** `todo`

**Description:**
When `CallRef.Receiver` matches a local variable in the enclosing function's `VarDefs`, resolve the method call through the method index.

**Acceptance criteria:**
- `c.Query()` where `c` is `db.Client` → `ResolvedTarget: "internal/db/client.go#Query"`
- `c.Execute()` where `c` is `Client` (same package, no pkg prefix) → resolves in same package's method index
- Same-name variables in different functions resolve independently
- Reassigned variables resolve to the closest preceding declaration
- Unresolvable (variable type is external, or method not found) → `ResolvedTarget: ""`

**Files to modify:**
```
internal/analyzer/linker.go
```

### Subtasks

#### S10c.4.1 — Implement var decl lookup

```go
func findVarType(fn *store.FunctionDecl, receiverName string, callLine int) string {
    var best *store.VarDecl
    for _, vd := range fn.VarDefs {
        if vd.Name == receiverName && vd.Line < callLine {
            if best == nil || vd.Line > best.Line {
                best = vd
            }
        }
    }
    if best == nil {
        return ""
    }
    return best.Type
}
```

#### S10c.4.2 — Implement method resolution

```go
func (li *linkIndex) resolveMethod(call *store.CallRef, fn *store.FunctionDecl, am *importAliasMap, currentPkgDir string) bool {
    varType := findVarType(fn, call.Receiver, call.Line)
    if varType == "" {
        return false
    }

    // Split "pkg.Type" into package alias and type name
    var pkgAlias, typeName string
    if idx := strings.LastIndex(varType, "."); idx >= 0 {
        pkgAlias = varType[:idx]
        typeName = varType[idx+1:]
    } else {
        typeName = varType
    }

    var pkgDir string
    if pkgAlias != "" {
        dir, ok := am.aliases[pkgAlias]
        if !ok {
            return false
        }
        pkgDir = dir
    } else {
        pkgDir = currentPkgDir
    }

    candidate, ok := li.methods[methodKey{pkgDir, typeName, call.Name}]
    if !ok {
        return false
    }
    call.ResolvedTarget = candidate.Path + "#" + call.Name
    call.Ownership = store.OwnershipInternal
    return true
}
```

#### S10c.4.3 — Unit tests

Create test fixture `testdata/fixtures/golang/method_calls.go`:
```go
package methodcalls

import "pkg/db"

func example() {
    var c db.Client
    c.Query("SELECT 1")
}

func other() {
    var c db.Client
    c.Query("SELECT 2")
}
```

Test:
- Both calls resolve to the same target (same type, same method)
- Variable `c` in `example()` and `other()` are scoped independently

Create test for receiver parameter:
```go
func (c *Client) Process() {
    c.execute()
}
```
Verify `c.execute()` resolves via implicit receiver `VarDecl`.

---

## S10c.5 — LinkCalls orchestrator

**Status:** `todo`

**Description:**
Wire all resolution strategies into a single `LinkCalls` function that iterates all files and all calls, applying resolution in priority order.

**Acceptance criteria:**
- `LinkCalls(idx) → CallGraph` produces a complete call graph
- Resolution priority: (1) import alias match → cross-package, (2) var decl match → method call, (3) empty receiver → same-package, (4) unresolved
- All resolved calls have `Ownership: OwnershipInternal`, unresolved have `Ownership: OwnershipExternal`
- `CallGraph` keys are `"file.go#FuncName"`, values are sorted edge lists

**Files to modify:**
```
internal/analyzer/linker.go
```

### Subtasks

#### S10c.5.1 — Implement LinkCalls

```go
func LinkCalls(idx store.FileIndex) store.CallGraph {
    li := buildLinkIndex(idx)
    graph := make(store.CallGraph)

    for _, entry := range idx {
        am := buildImportAliasMap(&entry)
        currentPkgDir := filepath.Dir(entry.Path)

        for i := range entry.Functions {
            fn := &entry.Functions[i]
            fnKey := fn.Path + "#" + fn.Name
            var edges []string

            for _, call := range fn.Calls {
                resolved := false

                // Priority 1: cross-package (receiver matches import alias)
                if call.Receiver != "" {
                    resolved = li.resolveCrossPackage(call, am)
                }

                // Priority 2: method call (receiver matches local var)
                if !resolved && call.Receiver != "" {
                    resolved = li.resolveMethod(call, fn, am, currentPkgDir)
                }

                // Priority 3: same-package call
                if !resolved && call.Receiver == "" {
                    resolved = li.resolveSamePackage(call, currentPkgDir)
                    if !resolved {
                        resolved = li.resolveDotImport(call, am)
                    }
                }

                // Unresolved
                if !resolved {
                    call.Ownership = store.OwnershipExternal
                }

                if call.ResolvedTarget != "" {
                    edges = append(edges, call.ResolvedTarget)
                }
            }

            if len(edges) > 0 {
                sort.Strings(edges)
                graph[fnKey] = edges
            }
        }
    }

    return graph
}
```

#### S10c.5.2 — Integration test with sample_repo

Build FileIndex from `testdata/sample_repo`, run `LinkCalls`, verify:

```go
expected := store.CallGraph{
    "internal/api/handler.go#Handle": {
        "internal/auth/jwt.go#ValidateToken",
        "pkg/logger/logger.go#Info",
    },
    "internal/db/client.go#Query": {
        "pkg/errors/errors.go#New",
        "pkg/logger/logger.go#Info",
    },
}
```

#### S10c.5.3 — Edge case tests

- File with no internal imports: all calls remain unresolved
- File calling function in same package: resolves without import alias
- File with explicit import alias: resolves correctly
- File with dot import: resolves via fallback
- Function with no calls: not present in CallGraph (or present with empty edge list)

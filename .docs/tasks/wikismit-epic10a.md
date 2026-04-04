# wikismit — Epic 10a: Call Chain — Data Model & Store Layer

**Status:** `todo`
**Depends on:** None (standalone foundation)
**Goal:** Add new types (`VarDecl`, `CallGraph`) and extend existing types (`CallRef.ResolvedTarget`, `Import.Alias`, `FunctionDecl.Calls/VarDefs`) in `pkg/store`. Add `CallGraph` read/write helpers. No AST or linking changes in this epic.
**Spec refs:** `ast-call-chain-resolution.md` Section 4 (Data Model Changes)

---

## S10a.1 — Extend store types

**Status:** `todo`

**Description:**
Add new fields and types to `pkg/store/artifacts.go` as defined in the spec. These are pure data model changes with no logic — just struct definitions.

**Acceptance criteria:**
- `CallRef` has `ResolvedTarget string` field with `json:"resolved_target,omitempty"`
- `Import` has `Alias string` field with `json:"alias,omitempty"`
- `FunctionDecl` has `Calls []*CallRef` field with `json:"calls,omitempty"`
- `FunctionDecl` has `VarDefs []*VarDecl` field with `json:"var_defs,omitempty"`
- New `VarDecl` struct with `Name`, `Type`, `Line` fields
- New `CallGraph` type: `map[string][]string`
- All new fields are backward-compatible (omitempty) — existing `file_index.json` without them still deserializes correctly

**Files to modify:**
```
pkg/store/artifacts.go
```

### Subtasks

#### S10a.1.1 — Add `VarDecl` struct

```go
type VarDecl struct {
    Name  string `json:"name"`
    Type  string `json:"type"`
    Line  int    `json:"line"`
}
```

#### S10a.1.2 — Extend `CallRef`

Add `ResolvedTarget string` field with `json:"resolved_target,omitempty"` tag.

#### S10a.1.3 — Extend `Import`

Add `Alias string` field with `json:"alias,omitempty"` tag.

#### S10a.1.4 — Extend `FunctionDecl`

Add `Calls []*CallRef` with `json:"calls,omitempty"` and `VarDefs []*VarDecl` with `json:"var_defs,omitempty"`.

#### S10a.1.5 — Add `CallGraph` type

```go
type CallGraph map[string][]string
```

#### S10a.1.6 — Backward compatibility test

- Load an existing `file_index.json` (without new fields) and verify it deserializes without error
- Marshal a `FunctionDecl` with empty `Calls` and `VarDefs` and verify no extra fields appear in output (omitempty)

---

## S10a.2 — CallGraph store helpers

**Status:** `todo`

**Description:**
Add read/write helpers for `call_graph.json` in `pkg/store`, following the same pattern as existing `WriteDepGraph`/`ReadDepGraph`.

**Acceptance criteria:**
- `WriteCallGraph(dir string, graph store.CallGraph) error` writes `call_graph.json`
- `ReadCallGraph(dir string) (store.CallGraph, error)` reads it back
- Round-trip: write then read produces identical `CallGraph`
- File is written atomically (tmp + rename, same as existing helpers)

**Files to create:**
```
pkg/store/call_graph.go
pkg/store/call_graph_test.go
```

### Subtasks

#### S10a.2.1 — Implement `WriteCallGraph`

- Use existing `writeJSON` helper to write to `{dir}/call_graph.json`
- Sort edge lists for deterministic output

#### S10a.2.2 — Implement `ReadCallGraph`

- Use existing `readJSON` helper to read from `{dir}/call_graph.json`
- Return `ErrArtifactNotFound` if file does not exist

#### S10a.2.3 — Round-trip test

- Create a `CallGraph` with 3+ entries and multiple edges
- Write to temp dir, read back, compare with `cmp.Equal`
- Verify file is valid JSON with sorted keys

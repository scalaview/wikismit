# Epic 8B.3 Analyzer Helper Objectization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap `BuildDepGraph` and `ComputeAffected` free functions in narrow helper structs (`DepGraphBuilder` and `ImpactAnalyzer`) that encapsulate their state dependencies. The original free functions remain as thin wrappers so all existing call sites continue to work without modification.

**Architecture:** Two new structs:
1. `DepGraphBuilder` — owns a `store.FileIndex`, provides a `Build() store.DepGraph` method.
2. `ImpactAnalyzer` — owns a `*store.NavPlan` and `store.DepGraph`, provides an `Analyze(changedFiles []gitdiff.FileChange) []store.Module` method.

The existing free functions become one-line delegators to the structs. This preserves the current API while enabling future state encapsulation (caching, configuration) on the struct.

**Tech Stack:** Go, `internal/analyzer`, `pkg/store`, `pkg/gitdiff`, standard `testing`.

---

### Task 1: Add behavior-locking tests where coverage is thin

**Files:**
- Modify: `internal/analyzer/affected_test.go`
- Modify: `internal/analyzer/analyzer_test.go`

- [ ] **Step 1: Review existing test coverage**

Run existing tests with coverage:

```bash
go test ./internal/analyzer/... -cover -v
```

Note which functions/paths lack coverage. Based on the exploration, `owningModules` and `buildReverseGraph` are private helpers — test them indirectly through `ComputeAffected`.

- [ ] **Step 2: Add a test for reverse-graph edge case (empty graph)**

Add a test that verifies `ComputeAffected` handles an empty DepGraph gracefully:

```go
func TestComputeAffectedReturnsEmptyForEmptyGraph(t *testing.T) {
    changes := []gitdiff.FileChange{{Path: "pkg/foo/foo.go", ChangeType: gitdiff.Modified}}
    plan := &store.NavPlan{Modules: []store.Module{
        {ID: "foo", Files: []string{"pkg/foo/foo.go"}},
    }}
    graph := store.DepGraph{} // empty
    result := analyzer.ComputeAffected(changes, plan, graph)
    if len(result) != 1 || result[0].ID != "foo" {
        t.Fatalf("expected exactly the directly-affected module 'foo', got: %v", result)
    }
}
```

- [ ] **Step 3: Add a test for self-referencing module in dep graph**

```go
func TestComputeAffectedHandlesSelfReferencingModule(t *testing.T) {
    // Module that depends on itself — should not infinite loop
    idx := buildSampleRepoIndex()
    graph := analyzer.BuildDepGraph(idx)
    // Inject a self-edge
    graph["internal/auth/jwt.go"] = append(graph["internal/auth/jwt.go"], "internal/auth/jwt.go")
    changes := []gitdiff.FileChange{{Path: "internal/auth/jwt.go", ChangeType: gitdiff.Modified}}
    plan := buildSampleRepoNavPlan()
    result := analyzer.ComputeAffected(changes, &plan, graph)
    // Should return without hanging, including auth module
    found := false
    for _, m := range result {
        if m.ID == "auth" { found = true }
    }
    if !found {
        t.Fatal("expected auth module in affected set")
    }
}
```

- [ ] **Step 4: Verify new tests pass**

```bash
go test ./internal/analyzer/... -v
```

Expected: **PASS** — existing and new tests pass. These tests lock behavior before the refactor.

---

### Task 2: Introduce `DepGraphBuilder`

**Files:**
- Modify: `internal/analyzer/dep_graph.go`

- [ ] **Step 1: Add the DepGraphBuilder struct**

Add after the existing `BuildDepGraph` function (after line ~130):

```go
// DepGraphBuilder encapsulates a FileIndex and builds a dependency graph from it.
// Use this instead of calling BuildDepGraph directly when you want to attach
// the graph construction to a stable owner.
type DepGraphBuilder struct {
    idx store.FileIndex
}

// NewDepGraphBuilder creates a builder for the given file index.
func NewDepGraphBuilder(idx store.FileIndex) *DepGraphBuilder {
    return &DepGraphBuilder{idx: idx}
}

// Build constructs the dependency graph from the builder's file index.
func (b *DepGraphBuilder) Build() store.DepGraph {
    return buildDepGraph(b.idx)
}
```

- [ ] **Step 2: Extract the core logic into a private function**

Rename the current `BuildDepGraph` body to a private `buildDepGraph` function:

```go
// buildDepGraph is the internal implementation shared by the free function and the builder.
func buildDepGraph(idx store.FileIndex) store.DepGraph {
    // ... existing body of BuildDepGraph ...
}
```

- [ ] **Step 3: Make BuildDepGraph a thin wrapper**

```go
// BuildDepGraph builds a dependency graph from the given file index.
// Deprecated: Use DepGraphBuilder for new code.
func BuildDepGraph(idx store.FileIndex) store.DepGraph {
    return buildDepGraph(idx)
}
```

The `Deprecated` comment signals intent without breaking anything.

- [ ] **Step 4: Verify analyzer tests still pass**

```bash
go test ./internal/analyzer/... -v
```

Expected: **PASS** — all existing tests pass because `BuildDepGraph` still delegates to the same core logic.

---

### Task 3: Introduce `ImpactAnalyzer`

**Files:**
- Modify: `internal/analyzer/affected.go`

- [ ] **Step 1: Add the ImpactAnalyzer struct**

Add after the existing `ComputeAffected` function:

```go
// ImpactAnalyzer determines which modules are affected by a set of file changes,
// using a navigation plan and dependency graph.
type ImpactAnalyzer struct {
    plan  *store.NavPlan
    graph store.DepGraph
}

// NewImpactAnalyzer creates an analyzer for the given plan and dependency graph.
func NewImpactAnalyzer(plan *store.NavPlan, graph store.DepGraph) *ImpactAnalyzer {
    return &ImpactAnalyzer{plan: plan, graph: graph}
}

// Analyze returns the set of modules affected by the given file changes.
func (a *ImpactAnalyzer) Analyze(changedFiles []gitdiff.FileChange) []store.Module {
    return computeAffected(changedFiles, a.plan, a.graph)
}
```

- [ ] **Step 2: Extract the core logic into a private function**

Rename the current `ComputeAffected` body to a private `computeAffected` function:

```go
func computeAffected(changedFiles []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
    // ... existing body of ComputeAffected ...
}
```

- [ ] **Step 3: Make ComputeAffected a thin wrapper**

```go
// ComputeAffected determines which modules are affected by a set of file changes.
// Deprecated: Use ImpactAnalyzer for new code.
func ComputeAffected(changedFiles []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module {
    return computeAffected(changedFiles, plan, graph)
}
```

- [ ] **Step 4: Verify analyzer tests still pass**

```bash
go test ./internal/analyzer/... -v
```

Expected: **PASS** — all existing tests pass because the free function wrappers delegate to identical logic.

---

### Task 4: Verify analyzer regressions

**Files:**
- No file changes — verification only

- [ ] **Step 1: Run focused analyzer tests**

```bash
go test ./internal/analyzer/... -v
```

Expected: **PASS**.

- [ ] **Step 2: Verify downstream pipeline still compiles and passes**

```bash
go test ./internal/pipeline/... -v
```

Expected: **PASS** — pipeline uses `analyzer.ComputeAffected` via its variable wrapper, which still works as a thin wrapper.

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -count=1
```

Expected: **PASS** — zero failures across all packages.

- [ ] **Step 4: Confirm thin wrappers are the only public API change**

```bash
grep -rn "DepGraphBuilder\|ImpactAnalyzer" internal/analyzer/
```

Expected: The new structs are defined in `dep_graph.go` and `affected.go`. No call sites are updated — they still use the free functions.

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/
git commit -m "refactor: introduce DepGraphBuilder and ImpactAnalyzer helper structs

Add narrow helper structs in internal/analyzer that encapsulate state
for dependency graph construction and impact analysis. The existing
BuildDepGraph and ComputeAffected free functions become thin wrappers
over the new structs, preserving all call sites without modification.

Behavior-locking tests added for edge cases (empty graph, self-referencing
modules) before the refactor."
```

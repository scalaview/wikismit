# wikismit — Epic 10d: Call Chain — Pipeline Integration, Cycle Detection & Testing

**Status:** `todo`
**Depends on:** Epic 10a (data model), Epic 10b (AST extraction), Epic 10c (linker)
**Goal:** Wire the linker into the Phase 1 pipeline, produce `call_graph.json` artifact, implement cycle detection, and add comprehensive integration tests.
**Spec refs:** `ast-call-chain-resolution.md` Sections 10-12 (Cycle Detection, Pipeline Integration, Testing)

---

## S10d.1 — Wire LinkCalls into Phase 1 pipeline

**Status:** `todo`

**Description:**
Integrate `LinkCalls` into the existing Phase 1 orchestration so that `call_graph.json` is produced alongside `file_index.json` and `dep_graph.json`.

**Acceptance criteria:**
- `wikismit build --repo ./testdata/sample_repo` produces `artifacts/call_graph.json`
- `file_index.json` now includes `resolved_target` on resolved `CallRef`s
- Existing `dep_graph.json` output is unchanged
- Phase 1 timing log includes call linking duration
- Running twice on unchanged repo produces byte-identical artifacts (idempotency)

**Files to modify:**
```
internal/analyzer/phase1.go
internal/analyzer/analyzer.go
```

### Subtasks

#### S10d.1.1 — Update `RunPhase1`

```go
func RunPhase1(cfg *configpkg.Config) error {
    fileIndex, err := RunPhase1FileIndex(cfg)
    if err != nil {
        return err
    }

    // NEW: link calls and produce call graph
    callGraph := LinkCalls(fileIndex)

    depGraph := BuildDepGraph(fileIndex)

    if err := store.WriteFileIndex(cfg.ArtifactsDir, fileIndex); err != nil {
        return err
    }
    if err := store.WriteDepGraph(cfg.ArtifactsDir, depGraph); err != nil {
        return err
    }
    if err := store.WriteCallGraph(cfg.ArtifactsDir, callGraph); err != nil {
        return err
    }

    return nil
}
```

#### S10d.1.2 — Update `Analyze` to return call graph

Modify `Analyzer.Analyze()` signature or add a separate method to return the `CallGraph` alongside the `FileIndex`. Two options:

Option A: Add `LinkCalls` call inside `Analyze()` and return both:
```go
func (a *Analyzer) Analyze(repoPath string) (store.FileIndex, store.CallGraph, error)
```

Option B: Keep `Analyze` returning `FileIndex` only, call `LinkCalls` separately in `RunPhase1`. This maintains backward compatibility.

**Recommendation:** Option B. `Analyze` does extraction, `LinkCalls` does linking. Clear separation.

#### S10d.1.3 — Add progress logging

- Before linking: `INFO "Phase 1b: linking call chains"`
- After linking: `INFO "Phase 1b complete: {n} edges in call graph ({m} resolved, {u} unresolved)"`
- Log unresolved calls at `DEBUG` level (receiver + name + file + line)

#### S10d.1.4 — Idempotency test

- Run `RunPhase1` twice on `testdata/sample_repo/` in temp artifact dirs
- Compare both `call_graph.json` outputs: must be byte-identical
- Compare both `file_index.json` outputs: must be byte-identical (resolved targets are deterministic)

---

## S10d.2 — Cycle detection

**Status:** `todo`

**Description:**
Detect cycles in the `CallGraph` using DFS with WHITE/GRAY/BLACK coloring. Report cycles for use by downstream summarization logic.

**Acceptance criteria:**
- Direct recursion (`factorial → factorial`) is detected
- Mutual recursion (`even → odd → even`) is detected
- Acyclic graphs return empty cycle list
- Multiple independent cycles are all found
- Cycle paths are reported in a usable format

**Files to modify:**
```
internal/analyzer/linker.go
internal/analyzer/linker_test.go
```

### Subtasks

#### S10d.2.1 — Implement cycle detection

```go
type CycleReport struct {
    Cycles [][]string // each cycle is a list of "file.go#Func" forming a loop
}

func DetectCycles(graph store.CallGraph) *CycleReport {
    const (
        white = 0
        gray  = 1
        black = 2
    )

    color := make(map[string]int)
    parent := make(map[string]string)
    var cycles [][]string

    var dfs func(node string)
    dfs = func(node string) {
        color[node] = gray
        for _, neighbor := range graph[node] {
            if color[neighbor] == gray {
                // cycle found — reconstruct path
                cycle := []string{neighbor}
                cur := node
                for cur != neighbor {
                    cycle = append(cycle, cur)
                    cur = parent[cur]
                }
                cycle = append(cycle, neighbor)
                // reverse to get correct order
                for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
                    cycle[i], cycle[j] = cycle[j], cycle[i]
                }
                cycles = append(cycles, cycle)
            } else if color[neighbor] == white {
                parent[neighbor] = node
                dfs(neighbor)
            }
        }
        color[node] = black
    }

    for node := range graph {
        if color[node] == white {
            dfs(node)
        }
    }

    return &CycleReport{Cycles: cycles}
}
```

#### S10d.2.2 — Unit tests for cycle detection

Create test fixture `testdata/fixtures/golang/cycles.go`:
```go
package cycles

func factorial(n int) int {
    if n <= 1 { return 1 }
    return n * factorial(n-1)
}

func even(n int) bool { return n == 0 || odd(n-1) }
func odd(n int) bool  { return n != 0 && even(n-1) }

func acyclic() { helper() }
func helper() {}
```

Test cases:
- `factorial → factorial`: single-node cycle detected
- `even → odd → even`: two-node cycle detected
- `acyclic → helper`: no cycle
- Empty call graph: no cycles
- Three-node cycle: `a → b → c → a` detected

#### S10d.2.3 — Add CycleReport to store (optional)

If downstream summarization needs cycle information, add `CycleReport` to `pkg/store` and write it as `artifacts/cycle_report.json`. This is optional in v1 — the `DetectCycles` function can be called on-demand by the summarization pipeline.

---

## S10d.3 — Comprehensive test fixtures

**Status:** `todo`

**Description:**
Create dedicated test fixtures covering all resolution patterns, edge cases, and error scenarios.

**Acceptance criteria:**
- Each fixture covers a distinct resolution scenario
- Golden files define expected `CallGraph` output
- Fixtures are self-contained (no external dependencies)

**Files to create:**
```
testdata/fixtures/golang/import_alias.go
testdata/fixtures/golang/var_decls.go
testdata/fixtures/golang/method_calls.go
testdata/fixtures/golang/cycles.go
testdata/fixtures/golang/dot_import.go
testdata/fixtures/golang/blank_import.go
testdata/fixtures/golang/same_package.go
```

### Subtasks

#### S10d.3.1 — import_alias.go fixture

```go
package importalias

import (
    authpkg "internal/auth"
    "pkg/logger"
)

func example() {
    authpkg.ValidateToken("token")
    logger.Info("msg")
}
```

Expected: both calls resolve via import alias map.

#### S10d.3.2 — method_calls.go fixture

```go
package methodcalls

import "pkg/db"

func example() {
    var c db.Client
    c.Query("SELECT 1")
}

func withComposite() {
    c := db.Client{DSN: "test"}
    c.Query("SELECT 2")
}
```

Expected: both `c.Query` calls resolve to `pkg/db/client.go#Query`.

#### S10d.3.3 — same_package.go fixture

```go
package samepkg

func Public() {
    helper()
}

func helper() {
    helper2()
}

func helper2() {}
```

Expected: `Public → helper`, `helper → helper2`, all resolved within same package.

#### S10d.3.4 — dot_import.go fixture

```go
package dotpkg

import . "pkg/math"

func example() {
    Add(1, 2)
}
```

Expected: `Add` resolves via dot-import fallback.

#### S10d.3.5 — unresolvable.go fixture

```go
package unresolvable

import "fmt"

func example(r io.Reader) {
    fmt.Println("hello")     // external → unresolved
    r.Read(nil)              // interface dispatch → unresolved
    externalFunc()           // no declaration found → unresolved
}
```

Expected: all three calls have empty `ResolvedTarget` and `Ownership: OwnershipExternal`.

---

## S10d.4 — End-to-end integration test

**Status:** `todo`

**Description:**
Run the full Phase 1 pipeline on `testdata/sample_repo` and verify all artifacts are correct.

**Acceptance criteria:**
- `artifacts/file_index.json`: all functions have populated `Calls` with `resolved_target` where applicable
- `artifacts/call_graph.json`: matches expected edges for sample_repo
- `artifacts/dep_graph.json`: unchanged from pre-call-chain output
- All artifacts are valid JSON
- No regressions in existing tests

**Files to create:**
```
internal/analyzer/linker_integration_test.go
```

### Subtasks

#### S10d.4.1 — Full pipeline integration test

```go
func TestLinkCallsIntegration(t *testing.T) {
    cfg := configFromSampleRepo(t)
    fileIndex, err := RunPhase1FileIndex(cfg)
    require.NoError(t, err)

    callGraph := LinkCalls(fileIndex)

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

    for key, expectedEdges := range expected {
        actualEdges, ok := callGraph[key]
        assert.True(t, ok, "missing key in call graph: %s", key)
        assert.Equal(t, expectedEdges, actualEdges)
    }
}
```

#### S10d.4.2 — Verify dep_graph unchanged

Run `BuildDepGraph` before and after linking, compare outputs. The dep graph should not be affected by call chain linking.

#### S10d.4.3 — Verify existing tests pass

Run full test suite:
```bash
go test ./internal/analyzer/... ./pkg/store/... -v
```

All existing tests must pass without modification (except for tests that reference removed `FileEntry`-level calls field from S10b.4).

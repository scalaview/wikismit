# Epic 8B.2 Remove Package-Level Logger State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the two package-level mutable logger variables (`logger` in `internal/planner/skeleton.go` and `sharedLogger` in `internal/preprocessor/shared_context.go`) by threading explicit logger parameters through the call chain. Existing warning behavior (skeleton truncation, hallucinated refs) must remain testable.

**Architecture:** Add a `logpkg.Logger` parameter to the functions that currently read package-level loggers. Callers (pipeline layer) create and pass the logger. For backward compatibility, functions accept `nil` logger meaning "no warnings". The package-level vars are deleted, not replaced with globals or setters.

**Tech Stack:** Go, `internal/planner`, `internal/preprocessor`, `internal/pipeline`, `pkg/logpkg`, standard `testing`.

---

### Task 1: Add failing warning-behavior tests

**Files:**
- Modify: `internal/planner/skeleton_test.go`
- Create (if needed): `internal/preprocessor/shared_context_test.go`

- [ ] **Step 1: Add a planner skeleton truncation warning test**

Add a test in `internal/planner/skeleton_test.go` that:
1. Creates a `logpkg.Logger` with verbose mode enabled (or a capturing logger)
2. Calls `BuildSkeleton` or `BuildFullSkeleton` with inputs that trigger truncation (e.g., a very large module with many symbols)
3. Asserts the warning message `"planner skeleton truncated"` was emitted

The test should pass a logger explicitly as a parameter. Since the function signature doesn't accept a logger yet, this test will not compile:

```go
func TestBuildSkeletonWarnsOnTruncation(t *testing.T) {
    buf := &bytes.Buffer{}
    log := logpkg.New(true) // verbose logger that writes to stderr
    // Build a module large enough to trigger truncation
    entries := make(map[string]store.FileEntry)
    for i := 0; i < 200; i++ {
        entries[fmt.Sprintf("pkg/file%d.go", i)] = store.FileEntry{
            Exports: []store.Export{{Name: fmt.Sprintf("Symbol%d", i)}},
        }
    }
    mod := store.Module{ID: "large", Files: []string{}}
    for k := range entries {
        mod.Files = append(mod.Files, k)
    }
    _ = BuildSkeleton(mod, entries, log)
    // Check that the logger emitted a truncation warning
    output := buf.String()
    if !strings.Contains(output, "planner skeleton truncated") {
        t.Fatalf("expected truncation warning, got: %q", output)
    }
}
```

Note: The exact signature and logger capture mechanism will be finalized during implementation. The key constraint is: logger must be an explicit parameter, not a package-level var.

Expected: **compile error** — `BuildSkeleton` doesn't accept a logger parameter.

- [ ] **Step 2: Add a preprocessor hallucinated-ref warning test**

Add a test in `internal/preprocessor/shared_context_test.go` (create if needed):

```go
func TestGroundSharedSummaryWarnsOnHallucinatedRef(t *testing.T) {
    // Create a summary with a hallucinated ref (ref that doesn't exist in the nav plan)
    summary := &store.SharedSummary{
        KeyFunctions: []store.KeyFunc{
            {Name: "DoThing", Ref: "pkg/nonexistent/file.go#L1"},
        },
    }
    plan := &store.NavPlan{
        Modules: []store.Module{
            {ID: "auth", Files: []string{"internal/auth/jwt.go"}},
        },
    }
    log := logpkg.New(true)
    GroundSharedSummary(summary, plan, log)
    // Verify warning was emitted for hallucinated ref
}
```

Expected: **compile error** — `GroundSharedSummary` doesn't accept a logger parameter.

- [ ] **Step 3: Verify RED**

```bash
go test ./internal/planner/... ./internal/preprocessor/... -v
```

Expected: **compile errors** for the new tests. Existing tests should still pass.

---

### Task 2: Thread logger through planner helpers

**Files:**
- Modify: `internal/planner/skeleton.go`
- Modify: `internal/planner/planner.go`

- [ ] **Step 1: Add logger parameter to BuildSkeleton / BuildFullSkeleton**

In `internal/planner/skeleton.go`, update the function signatures:

```go
// Before:
func BuildSkeleton(mod store.Module, idx store.FileIndex) string
func BuildFullSkeleton(idx store.FileIndex) map[string]string

// After:
func BuildSkeleton(mod store.Module, idx store.FileIndex, log logpkg.Logger) string
func BuildFullSkeleton(idx store.FileIndex, log logpkg.Logger) map[string]string
```

Replace the package-level `logger` usage (line 100-102 in the warning call) with the `log` parameter:

```go
// Before:
logger.Warn("planner skeleton truncated", "dropped_symbols", droppedSymbols)

// After:
if log != nil {
    log.Warn("planner skeleton truncated", "dropped_symbols", droppedSymbols)
}
```

Delete the package-level variable:

```go
// DELETE this line:
var logger logpkg.Logger = logpkg.New(false)
```

- [ ] **Step 2: Update RunPlanner to pass logger**

In `internal/planner/planner.go`, update `RunPlanner` to:
1. Accept a logger parameter (or create one from config verbose flag)
2. Remove the package-level logger assignment + deferred reset (lines 18-23)
3. Pass the logger to `BuildSkeleton` / `BuildFullSkeleton` calls

The current pattern in planner.go:

```go
// Before (lines 18-23 approximately):
plannerLogger := logger
if plannerLogger == nil {
    plannerLogger = logpkg.New(cfg.Verbose)
    logger = plannerLogger
    defer func() { logger = nil }()
}
```

Replace with explicit parameter threading:

```go
// After: logger comes from the caller or is created locally
plannerLogger := logpkg.New(cfg.Verbose)
// ... pass plannerLogger to BuildSkeleton/BuildFullSkeleton calls
```

The exact signature change for `RunPlanner` depends on how callers pass config. Check if `RunPlanner` already receives a config struct with `Verbose` field. If so, create the logger inside `RunPlanner` from config — no new parameter needed. If not, add a `log logpkg.Logger` parameter.

- [ ] **Step 3: Update all callers of BuildSkeleton / BuildFullSkeleton**

Find all call sites and add the logger argument. This includes:
- `internal/planner/planner.go` (calls from RunPlanner)
- `internal/planner/skeleton_test.go` (existing tests — pass `logpkg.New(false)` or `nil`)
- Any other callers in the codebase

Use `grep -rn "BuildSkeleton\|BuildFullSkeleton" internal/` to find all callers.

- [ ] **Step 4: Verify planner tests pass**

```bash
go test ./internal/planner/... -v
```

Expected: **PASS** — all planner tests pass with explicit logger parameter.

---

### Task 3: Thread logger through shared-context grounding

**Files:**
- Modify: `internal/preprocessor/shared_context.go`
- Modify: `internal/preprocessor/preprocessor.go`

- [ ] **Step 1: Add logger parameter to grounding functions**

In `internal/preprocessor/shared_context.go`, update functions that use `sharedLogger`:

```go
// Before:
func GroundSharedSummary(summary *store.SharedSummary, plan *store.NavPlan) error

// After:
func GroundSharedSummary(summary *store.SharedSummary, plan *store.NavPlan, log logpkg.Logger) error
```

Replace `sharedLogger.Warn(...)` with nil-safe logging:

```go
// Before (line 41-43):
sharedLogger.Warn("hallucinated ref", "ref", ref)

// After:
if log != nil {
    log.Warn("hallucinated ref", "ref", ref)
}
```

Delete the package-level variable:

```go
// DELETE this line:
var sharedLogger logpkg.Logger = logpkg.New(false)
```

- [ ] **Step 2: Update RunPreprocessor to pass logger**

In `internal/preprocessor/preprocessor.go`, update the function that calls grounding:

1. Create logger from config verbose flag (or accept as parameter)
2. Pass logger to `GroundSharedSummary` and any other functions that previously read `sharedLogger`

- [ ] **Step 3: Update all callers**

Find all call sites of `GroundSharedSummary` and related functions:

```bash
grep -rn "GroundSharedSummary" internal/
```

Add the logger argument to each call site. Existing tests should pass `nil` or `logpkg.New(false)`.

- [ ] **Step 4: Verify preprocessor tests pass**

```bash
go test ./internal/preprocessor/... -v
```

Expected: **PASS** — all preprocessor tests pass.

---

### Task 4: Verify logger-state removal

**Files:**
- No file changes — verification only

- [ ] **Step 1: Confirm no package-level logger vars remain in touched files**

```bash
grep -rn "^var.*logpkg.Logger\|^var.*log.*Logger\|^var logger\|^var sharedLogger" internal/planner/ internal/preprocessor/
```

Expected: **no matches** — all package-level logger variables removed.

- [ ] **Step 2: Run focused planner and preprocessor tests**

```bash
go test ./internal/planner/... ./internal/preprocessor/... -v
```

Expected: **PASS**.

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -count=1
```

Expected: **PASS** — zero failures. Pipeline layer may need updates to pass loggers through — fix any compilation errors from the signature changes.

- [ ] **Step 4: Commit**

```bash
git add internal/planner/ internal/preprocessor/ internal/pipeline/
git commit -m "refactor: thread logger through planner and preprocessor

Remove package-level logger variables from internal/planner/skeleton.go
and internal/preprocessor/shared_context.go. Functions now accept explicit
logpkg.Logger parameters, with nil meaning no logging.

Callers in internal/pipeline create loggers from config.Verbose and pass
them through. Warning behavior (skeleton truncation, hallucinated refs)
is preserved and now testable."
```

# wikismit — Epic 8B: Runtime State Cleanup + Narrow Boundaries

**Status:** `todo`
**Depends on:** Epic 7H
**Goal:** Remove hidden package-level runtime state, consolidate command-side client creation, and introduce narrow helper objects where they simplify testing without forcing a full package rewrite.
**Spec refs:** code-review-oop-refactoring.md §2.2, §4.1 B-C, §5.1-§5.3

---

## S8B.1 — Shared command-side client factory

**Status:** `todo`

**Description:**
Replace duplicated client-construction logic in `generate`, `plan`, and `update` with one helper. The helper should preserve the existing test-factory seams while making retry and logger wiring consistent across commands.

**Acceptance criteria:**
- `generate`, `plan`, and `update` use one shared helper for runtime client creation
- Existing `agentClientFactory`, `updateClientFactory`, and planner-specific mock seams continue to work in tests
- Shared client creation remains compatible with future retry/logging enhancements
- Command behavior remains unchanged when no retries or verbose logging are enabled

**Files to modify:**
```
cmd/wikismit/generate.go
cmd/wikismit/plan.go
cmd/wikismit/update.go
cmd/wikismit/main_test.go
cmd/wikismit/helpers.go
```

### Subtasks

#### S8B.1.1 — Add failing command tests for shared client creation

- Add or update tests proving each command still honors its current mock factory override path
- Cover the fallback order from command-specific factory to shared default client creation

#### S8B.1.2 — Introduce one client-construction helper

- Create a helper in `cmd/wikismit/` that centralizes runtime client creation
- Preserve command-specific overrides by passing the command’s preferred factory into the helper
- Avoid coupling the helper to any one command’s output behavior

#### S8B.1.3 — Migrate commands to the helper

- Update `generate`, `plan`, and `update` to call the helper instead of reimplementing client creation inline
- Keep the helper small and command-package-local

#### S8B.1.4 — Verify command regressions

- Run focused CLI tests covering `generate`, `plan`, and `update`
- Re-run `go test ./cmd/wikismit -v`

---

## S8B.2 — Remove package-level logger state

**Status:** `todo`

**Description:**
Eliminate package-level logger variables in planner and preprocessor helper code. Replace them with explicit logger parameters or receiver fields so warning/debug behavior is testable and no hidden mutable runtime state remains.

**Acceptance criteria:**
- `internal/planner/skeleton.go` no longer owns a package-level `logger`
- `internal/preprocessor/shared_context.go` no longer owns a package-level `sharedLogger`
- Callers can inject a logger or `nil` explicitly
- Existing warning behavior (`planner skeleton truncated`, `hallucinated ref`) remains testable

**Files to modify:**
```
internal/planner/skeleton.go
internal/planner/planner.go
internal/preprocessor/shared_context.go
internal/preprocessor/preprocessor.go
internal/planner/skeleton_test.go
internal/preprocessor/shared_context_test.go
```

### Subtasks

#### S8B.2.1 — Add failing warning-behavior tests

- Add or extend tests that can capture logger output for skeleton truncation warnings
- Add or extend tests that can capture hallucinated-ref warnings during shared-context grounding

#### S8B.2.2 — Thread logger through planner helpers

- Change `BuildSkeleton` / `BuildFullSkeleton` signatures or surrounding planner wiring so logger ownership is explicit
- Keep planner call sites easy to follow; avoid introducing a global setter

#### S8B.2.3 — Thread logger through shared-context grounding

- Update shared summary grounding so warning emission uses an injected logger
- Keep the grounding logic pure apart from explicit warning output

#### S8B.2.4 — Verify logger-state removal

- Run focused planner and preprocessor tests
- Confirm no package-level logger vars remain in the touched files

---

## S8B.3 — Analyzer helper objectization

**Status:** `todo`

**Description:**
Introduce narrow helper structs around dependency graph construction and impact analysis while preserving current function entrypoints. The goal is localized state ownership, not a wholesale analyzer rewrite.

**Acceptance criteria:**
- `BuildDepGraph` remains callable from existing code paths
- `ComputeAffected` remains callable from existing code paths
- New helper structs encapsulate the state currently spread across free functions
- Existing analyzer tests continue to pass with no change in external behavior

**Files to modify:**
```
internal/analyzer/dep_graph.go
internal/analyzer/affected.go
internal/analyzer/affected_test.go
internal/analyzer/analyzer_test.go
```

### Subtasks

#### S8B.3.1 — Add behavior-locking tests where coverage is thin

- Expand tests around reverse-graph construction and affected-module propagation only where current coverage would not protect a wrapper refactor

#### S8B.3.2 — Introduce `DepGraphBuilder`

- Add a small builder struct that owns `store.FileIndex`
- Keep `BuildDepGraph(idx)` as a thin wrapper over the builder so downstream call sites stay stable

#### S8B.3.3 — Introduce `ImpactAnalyzer`

- Add a helper struct that owns `plan` and `graph`
- Keep `ComputeAffected(...)` as a thin wrapper over the helper so incremental pipeline call sites remain stable

#### S8B.3.4 — Verify analyzer regressions

- Run `go test ./internal/analyzer -v`
- Re-run `go test ./...` after all Epic 8B changes land

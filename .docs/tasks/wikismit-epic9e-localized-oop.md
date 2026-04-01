# wikismit — Epic 9D: Localized Package Objectization (Preprocessor + Composer)

**Status:** `todo`
**Depends on:** Epic 9E
**Goal:** Introduce internal structs for preprocessor and composer behind stable function entrypoints so package responsibilities are easier to follow and test, without committing to a full agent/pipeline redesign.
**Spec refs:** code-review-oop-refactoring.md §4.1 D-E, §5.1, code-review-improvement.md §2.4-§2.5

---

## S9E.1 — Preprocessor Façade Struct

**Status:** `todo`

**Description:**
Refactor preprocessor orchestration into a `Preprocessor` struct with `Run` and `RunFor` methods while keeping the existing package-level functions as thin wrappers. Limit this epic to the preprocessor package.

**Acceptance criteria:**
- `RunPreprocessor` and `RunPreprocessorFor` remain exported and keep their current signatures
- A new `Preprocessor` struct owns `cfg`, `client`, and logger collaborators explicitly
- Shared-subgraph and topological-order behavior remain unchanged
- Existing preprocessor tests continue to pass without widening the public API surface
- Package-level `sharedLogger` variable removed

**Files to modify:**
```
internal/preprocessor/preprocessor.go
internal/preprocessor/preprocessor_test.go
internal/preprocessor/shared_context.go
```

### Subtasks

#### S9E.1.1 — Add Delegation-Focused Tests
Legacy - Add tests proving wrapper-to-struct delegation behavior
- Test that struct constructor properly initializes all dependencies
- Reuse existing preprocessor behavior tests wherever possible

#### S9E.1.2 — Introduce `Preprocessor` Struct and Constructor
Legacy - Add struct that owns config, client, and logger
- Add `Run` and `RunFor` methods that hold current orchestration behavior
- Inject logger into `groundSharedSummaryRefs` to eliminate `sharedLogger` global

#### S9E.1.3 — Preserve Package-Level Wrapper Entrypoints
Legacy - Keep `RunPreprocessor` and `RunPreprocessorFor` as thin wrappers around struct methods
- Avoid changing downstream call sites in `pipeline`

#### S9E.1.4 — Remove `sharedLogger` Package-Level Variable
Legacy - Thread logger through to `groundSharedSummaryRefs` as parameter
- Remove `var sharedLogger logpkg.Logger = logpkg.New(false)` from `shared_context.go`
- Update all callers

#### S9E.1.5 — Verify Preprocessor Behavior Parity
Legacy - Run `go test ./internal/preprocessor -v`
- Re-run any incremental pipeline tests that rely on partial preprocessor execution

---

## S9E.2 — Composer Façade Struct

**Status:** `todo`

**Description:**
Refactor `internal/composer` so the package has a `Composer` struct that owns config, plan, file index, and dependency graph while preserving the existing `RunComposer` entrypoint.

**Acceptance criteria:**
- `RunComposer` remains exported and keeps its current signature
- `Composer` owns config/plan/index/graph explicitly
- Copying module docs, index generation, validation report writing, and VitePress asset writing still happen in the same order
- Existing composer tests continue to pass with no user-visible behavior change

**Files to modify:**
```
internal/composer/renderer.go
internal/composer/renderer_test.go
internal/composer/citation.go
internal/composer/vitepress.go
```

### Subtasks

#### S9E.2.1 — Lock Current Composer Ordering with Tests
Legacy - Add or extend tests only where current coverage would not catch ordering regressions
- Verify the step order: copy → index → validate → vitepress

#### S9E.2.2 — Introduce `Composer` Struct and Constructor
Legacy - Add struct that owns config, plan, index, and graph
- Move orchestration logic from `RunComposer` into a `Run` method on the struct
- Convert helper functions to methods only when they consume composer-owned state

#### S9E.2.3 — Keep Helper Boundaries Local
Legacy - Leave pure helpers (e.g., `GenerateTOC`, `anchorForHeading`) as free functions
- Convert only state-dependent helpers to methods
- Avoid over-objectizing for its own sake

#### S9E.2.4 — Preserve the Existing Package Entrypoint
Legacy - Keep `RunComposer` as a thin wrapper over the new struct method
- Avoid changing external call sites in `cmd/` or `pipeline`

#### S9E.2.5 — Verify Composer Regressions
Legacy - Run `go test ./internal/composer -v`
- Re-run `go test ./...` after all S9E.2 changes land

---

## S9E.3 — Clean Up Analyzer Dual API Pattern

**Status:** `todo`

**Description:**
`DepGraphBuilder` is an empty-shell struct coexisting with standalone `BuildDepGraph` function. `ImpactAnalyzer` is a thin wrapper over private functions. Consolidate to single API style per component.

**Acceptance criteria:**
- `DepGraphBuilder` removed or fully adopted (no dual API)
- `ImpactAnalyzer` methods contain actual logic (not just forwarding to private functions)
- Private functions (`owningModules`, `buildReverseGraph`, `computeAffected`) moved to `ImpactAnalyzer` methods
- Existing call sites updated to use the preferred API
- Existing tests pass unchanged

**Files to modify:**
```
internal/analyzer/dep_graph.go
internal/analyzer/affected.go
internal/analyzer/affected_test.go
internal/analyzer/dep_graph_test.go
```

### Subtasks

#### S9E.3.1 — Consolidate DepGraphBuilder
Legacy - Decide: remove `DepGraphBuilder` (prefer `BuildDepGraph` function) or remove `BuildDepGraph` (prefer builder)
- Update `ResolveImportPaths` and `reanalyzeChanged` to use the chosen API
- Add tests for the chosen path

#### S9E.3.2 — Flesh Out ImpactAnalyzer
Legacy - Move `owningModules` to `(a *ImpactAnalyzer) owningModules` method
- Move `buildReverseGraph` to `(a *ImpactAnalyzer) buildReverseGraph` method
- Move `computeAffected` logic into `Analyze` method directly
- Remove `ComputeAffected` free function or keep as thin wrapper

#### S9E.3.3 — Verify Analyzer Consolidation
Legacy - Run `go test ./internal/analyzer -v`
- Verify pipeline still works with updated analyzer APIs

---

## S9E.4 — Clean Up Dead Code and Comments

**Status:** `todo`

**Description:**
Remove dead code and add necessary comments to complex algorithms.

**Acceptance criteria:**
- `topoSort` in preprocessor.go removed or annotated as test-only
- Tarjan SCC implementation has comments explaining algorithm choice
- No other behavior changes

**Files to modify:**
```
internal/preprocessor/preprocessor.go
```

### Subtasks

#### S9E.4.1 — Clean Up `topoSort`
Legacy - Remove `topoSort` if no production code uses it (test uses can be updated)
- Or annotate with `// only used in tests` if removal is too disruptive

#### S9E.4.2 — Document Tarjan SCC
Legacy - Add comment block explaining why SCC is used instead of simple topological sort
- Note: shared modules may have circular dependencies, SCC finds strongly connected components first

---

## S9E.5 — Final Verification

**Status:** `todo`

**Description:**
Verify all localized refactoring is complete without scope creep.

**Acceptance criteria:**
- No changes to `internal/agent` orchestration APIs
- No changes to `internal/pipeline` public orchestration APIs
- `go test ./...` passes with zero failures
- Changed files are diagnostics-clean
- The final diff is limited to preprocessor/composer/analyzer-localized work plus tests

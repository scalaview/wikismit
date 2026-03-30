# wikismit — Epic 8C: Localized Package Objectization

**Status:** `todo`
**Depends on:** Epic 8B
**Goal:** Introduce internal structs for composer and preprocessor behind stable function entrypoints so package responsibilities are easier to follow and test without committing to a full agent/pipeline redesign.
**Spec refs:** code-review-oop-refactoring.md §4.1 D-E, §6

---

## S8C.1 — Preprocessor façade struct

**Status:** `todo`

**Description:**
Refactor preprocessor orchestration into a `Preprocessor` struct with `Run` and `RunFor` methods while keeping the existing package-level functions as thin wrappers. Limit this epic to the preprocessor package and do not redesign agent or pipeline orchestration here.

**Acceptance criteria:**
- `RunPreprocessor` and `RunPreprocessorFor` remain exported and keep their current signatures
- A new `Preprocessor` struct owns `cfg`, `client`, and logger/runtime collaborators explicitly
- Shared-subgraph and topological-order behavior remain unchanged
- Existing preprocessor tests continue to pass without widening the public API surface

**Files to modify:**
```
internal/preprocessor/preprocessor.go
internal/preprocessor/preprocessor_test.go
internal/preprocessor/shared_context.go
```

### Subtasks

#### S8C.1.1 — Add delegation-focused tests if needed

- Add only the smallest tests needed to lock wrapper-to-struct delegation behavior
- Reuse existing preprocessor behavior tests wherever possible

#### S8C.1.2 — Introduce `Preprocessor` struct and constructor

- Add a struct that owns config, client, and logger/runtime collaborators
- Add `Run` and `RunFor` methods that hold the current orchestration behavior

#### S8C.1.3 — Preserve package-level wrapper entrypoints

- Keep `RunPreprocessor` and `RunPreprocessorFor` as thin wrappers around the struct methods
- Avoid changing downstream call sites in `pipeline` during this epic

#### S8C.1.4 — Verify preprocessor behavior parity

- Run `go test ./internal/preprocessor -v`
- Re-run any incremental pipeline tests that rely on partial preprocessor execution

---

## S8C.2 — Composer façade struct

**Status:** `todo`

**Description:**
Refactor `internal/composer` so the package has a `Composer` struct that owns config, plan, file index, and dependency graph while preserving the existing `RunComposer` entrypoint. Keep helper extraction local to this package.

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

#### S8C.2.1 — Lock current composer ordering with tests

- Add or extend tests only where current coverage would not catch ordering regressions during the wrapper refactor

#### S8C.2.2 — Introduce `Composer` struct and constructor

- Add a struct that owns config, plan, index, and graph
- Move orchestration logic from `RunComposer` into a method on the struct

#### S8C.2.3 — Keep helper boundaries local to the package

- Convert helper functions to methods only when they consume composer-owned state
- Leave pure helpers as free functions when objectizing them adds no value

#### S8C.2.4 — Preserve the existing package entrypoint

- Keep `RunComposer` as a thin wrapper over the new struct method
- Avoid changing external call sites in `cmd/` or `pipeline`

#### S8C.2.5 — Verify composer regressions

- Run `go test ./internal/composer -v`
- Re-run `go test ./...` after all Epic 8C changes land

---

## S8C.3 — Stop before agent/pipeline redesign

**Status:** `todo`

**Description:**
Finish this epic by verifying that localized objectization delivered cleaner boundaries without forcing a full redesign of `internal/agent` or `internal/pipeline`. This is a deliberate scope gate, not a placeholder for hidden follow-up work.

**Acceptance criteria:**
- No changes are made to `internal/agent` orchestration APIs in this epic
- No changes are made to `internal/pipeline` public orchestration APIs in this epic
- The final diff is limited to composer/preprocessor-localized objectization work plus tests
- The repository still passes full regression after the refactor lands

**Files to modify:**
```
internal/preprocessor/preprocessor.go
internal/composer/renderer.go
internal/composer/renderer_test.go
```

### Subtasks

#### S8C.3.1 — Re-check package boundaries before final verification

- Confirm the epic did not grow to include agent or pipeline redesign work
- Remove any speculative interface or DI scaffolding that is not required by the accepted scope

#### S8C.3.2 — Run final regression for localized refactors

- Run focused preprocessor and composer tests
- Run `go test ./...`
- Confirm changed files remain diagnostics-clean

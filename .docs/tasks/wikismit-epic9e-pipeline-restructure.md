# wikismit — Epic 9E: Pipeline Restructure

**Status:** `todo`
**Depends on:** Epic 9D
**Goal:** Introduce `Pipeline` struct to replace seven global function variables, define domain interfaces for core components, and encapsulate agent orchestration. This is the largest refactoring effort — only undertake after localized objectization is proven stable.
**Spec refs:** code-review-oop-refactoring.md §4.1 A-F, §7, code-review-improvement.md §2.3

---

## S9E.1 — Define Domain Interfaces

**Status:** `todo`

**Description:**
Define interfaces for core components so the Pipeline can accept dependencies via injection rather than package-level function variables.

**Acceptance criteria:**
- `CodeAnalyzer` interface covers `RunPhase1` and `BuildDepGraph`
- `Planner` interface covers `Plan` method
- `Preprocessor` interface covers `Run` and `RunFor` methods
- `AgentRunner` interface covers agent execution
- `Composer` interface covers composition
- All interfaces live in a shared location (e.g., `internal/interfaces/` or alongside their implementations)

**Files to modify:**
```
internal/interfaces/interfaces.go (new file)
```

### Subtasks

#### S9E.1.1 — Create Interfaces Package
Legacy - Create `internal/interfaces/` package
- Define `CodeAnalyzer`, `PlannerService`, `PreprocessorService`, `AgentRunner`, `ComposerService` interfaces
- Keep interfaces minimal — only methods actually called by pipeline

#### S9E.1.2 — Verify Interface Compatibility
Legacy - Verify that existing struct implementations (from Epic 9D) satisfy the new interfaces
- Add compile-time checks if needed

---

## S9E.2 — Pipeline Struct

**Status:** `todo`

**Description:**
Replace the seven global function variables in `pipeline/incremental.go` with a `Pipeline` struct that accepts dependencies via constructor injection.

**Acceptance criteria:**
- `Pipeline` struct owns config, client, logger, and all phase collaborators
- `NewPipeline` constructor creates default implementations from config
- `RunFull` and `RunIncremental` are methods on the struct
- Seven global `var` declarations removed from `incremental.go`
- Existing `RunFullGenerate` and `RunIncremental` remain as thin wrappers for backward compatibility
- All existing tests pass

**Files to modify:**
```
internal/pipeline/incremental.go
internal/pipeline/pipeline.go (new or refactored from incremental.go)
internal/pipeline/incremental_test.go
```

### Subtasks

#### S9E.2.1 — Add Pipeline Struct Tests
Legacy - Add tests for `Pipeline.RunFull` that use mock collaborators
- Add tests for `Pipeline.RunIncremental` that use mock collaborators
- Replicate existing test coverage using struct methods instead of global variable overrides

#### S9E.2.2 — Introduce `Pipeline` Struct
Legacy - Create `Pipeline` struct with injected collaborators (analyzer, planner, preprocessor, agentRunner, composer)
- Implement `RunFull` and `RunIncremental` as methods
- Replace global variable calls with struct field calls

#### S9E.2.3 — Migrate Tests from Global Vars to Struct
Legacy - Convert existing tests from overriding global vars to injecting mocks into struct
- Ensure same test coverage is maintained
- Remove global variable test seam pattern

#### S9E.2.4 — Keep Backward-Compatible Function Entrypoints
Legacy - `RunFullGenerate` creates a default `Pipeline` and calls `RunFull`
- `RunIncremental` creates a default `Pipeline` and calls `RunIncremental`
- `IncrementalOptions` preserved

#### S9E.2.5 — Verify Pipeline Regression
Legacy - Run `go test ./internal/pipeline -v`
- Run `go test ./cmd/wikismit -v`
- Verify `generate` and `update` commands still work end-to-end

---

## S9E.3 — Agent Orchestrator Struct

**Status:** `todo`

**Description:**
Encapsulate agent execution logic into an `AgentOrchestrator` struct with proper dependency ownership.

**Acceptance criteria:**
- `AgentOrchestrator` struct owns client, artifactsDir, concurrency, logger
- `Run` and `RunFor` methods on the struct
- Existing `agent.Run` and `agent.RunFor` remain as thin wrappers
- All existing agent tests pass

**Files to modify:**
```
internal/agent/agent.go
internal/agent/scheduler.go
internal/agent/types.go
internal/agent/agent_test.go
internal/agent/scheduler_test.go
```

### Subtasks

#### S9E.3.1 — Add Agent Orchestrator Tests
Legacy - Add tests for `AgentOrchestrator` with mock client
- Test concurrency behavior is preserved

#### S9E.3.2 — Introduce `AgentOrchestrator` Struct
Legacy - Create struct with client, artifactsDir, concurrency, logger fields
- Move `Run` and `RunFor` logic to methods
- Extract `runAgent` into a per-module execution method

#### S9E.3.3 — Preserve Backward-Compatible Entrypoints
Legacy - `agent.Run` creates default orchestrator and delegates
- `agent.RunFor` creates default orchestrator and delegates

#### S9E.3.4 — Verify Agent Regression
Legacy - Run `go test ./internal/agent -v`
- Verify concurrent execution still works correctly

---

## S9E.4 — Final Verification

**Status:** `todo`

**Description:**
Full regression and integration verification after all pipeline restructuring.

**Acceptance criteria:**
- `go test ./...` passes with zero failures
- `go vet ./...` shows no issues
- No package-level mutable state remaining in pipeline, agent, planner, preprocessor, or composer packages
- All backward-compatible function entrypoints still work
- No diagnostic errors in changed files

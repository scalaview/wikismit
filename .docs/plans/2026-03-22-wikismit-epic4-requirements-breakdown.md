# wikismit Epic 4 Requirements Breakdown

## Goal

Split Epic 4 into smaller requirement slices before writing implementation plans.

Epic 4 target from `.docs/tasks/wikismit-epic4.md` and `.docs/spec/wikismit-tech-spec.md` is:

- build Phase 4 prompts from per-module code skeletons plus the relevant subset of shared module summaries
- enforce the shared-module ownership rule so module agents link shared modules instead of re-describing them
- run one module-documentation agent per non-shared module concurrently with a configured semaphore limit
- collect per-module results into `artifacts/module_docs/` while preserving partial success behavior
- wire Phase 4 into `wikismit generate` after the Phase 3 preprocessor completes
- report timing and failure summaries without failing the process with a non-zero exit code in v1

## Requirement slices

### R1 — Agent prompt construction and Phase 4 type foundation

Source: `S4.1`, spec §4 Phase 4, spec §7 Key Interfaces, spec §13 testing strategy.

Includes:

- create `internal/agent/types.go`, `internal/agent/prompt.go`, and `internal/agent/prompt_test.go`
- define `AgentInput` and `ModuleDoc` types using existing `store.Module`, `store.FileIndex`, and `store.SharedContext`
- build module-level skeleton text via `planner.BuildSkeleton`
- format shared-context prompt sections from `Module.DependsOnShared`
- inject ownership constraint instructions and explicit citation format rules
- verify prompt structure with snapshot-style tests and `MockClient` call assertions

Output:

- deterministic Phase 4 prompts exist for modules with and without shared dependencies
- prompt ownership constraints are locked before any goroutine scheduling work begins

### R2 — Concurrency-controlled scheduling and result collection

Source: `S4.2`, spec §4 Phase 4 concurrency model, spec §12 Error Handling.

Includes:

- create `internal/agent/scheduler.go` and `internal/agent/scheduler_test.go`
- implement semaphore-based goroutine scheduling bounded by `cfg.Agent.Concurrency`
- collect `ModuleDoc` results from a buffered channel until all workers finish
- write successful outputs to `artifacts/module_docs/{moduleID}.md`
- accumulate failed module results, log them, and return summary error metadata without stopping in-flight workers
- verify concurrency limits, no goroutine leak, and partial-success collection behavior

Output:

- Phase 4 can fan out safely over non-shared modules with deterministic collection semantics
- partial failure handling is explicit and testable before agent runtime integration begins

### R3 — Single-agent execution, reporting, and `generate` integration

Source: `S4.3`, spec §4 Phase 4, spec §10 CLI design, spec §12 Error Handling.

Includes:

- create `internal/agent/agent.go` and `internal/agent/agent_test.go`
- implement `runAgent` to build prompts, call `llm.Client`, and return `ModuleDoc`
- record per-module timing and log success/failure outcomes
- emit a final Phase 4 success/failure summary to `stderr`
- modify `cmd/wikismit/generate.go` to load Phase 2+3 artifacts, filter non-shared modules, and invoke the scheduler
- verify that failed modules do not produce partial output files while successful modules still write their Markdown

Output:

- `wikismit generate` can execute Phase 4 after Phase 3 and write module docs for non-shared modules
- failures are surfaced as reports rather than silently disappearing or aborting the whole phase

## Dependency order

Implement in this order:

1. `R1` agent types + prompt construction
2. `R2` scheduler semaphore + collector
3. `R3` single-agent execution + Phase 4 integration

## Implementation document map

- `R1` → `.docs/plans/2026-03-22-wikismit-epic4-plan-01-agent-prompt-foundation.md`
- `R2` → `.docs/plans/2026-03-22-wikismit-epic4-plan-02-scheduler-and-collection.md`
- `R3` → `.docs/plans/2026-03-22-wikismit-epic4-plan-03-agent-runner-and-generate-integration.md`

## Out of scope for Epic 4

Do not implement yet:

- Phase 5 composer, citation injection, and VitePress generation
- incremental update mode and affected-module diff orchestration
- Phase 4 rate limiting and advanced agent quality follow-up epics
- monorepo plan splitting or cross-subproject scheduling
- any new CLI command beyond extending the existing `generate` flow

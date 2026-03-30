# wikismit Epic 8B Plan Index

Use this index instead of executing `.docs/tasks/wikismit-epic8b.md` directly.

## Read first

1. `.docs/tasks/wikismit-epic8b.md`
2. `.docs/spec/code-review-oop-refactoring.md` (§2.2, §4.1 B-C, §5.1-§5.3)

## Execution order

1. `.docs/plans/2026-03-30-wikismit-epic8b-plan-01-shared-client-factory.md`
2. `.docs/plans/2026-03-30-wikismit-epic8b-plan-02-remove-package-level-logger-state.md`
3. `.docs/plans/2026-03-30-wikismit-epic8b-plan-03-analyzer-helper-objectization.md`

## Why this split exists

Epic 8B has three independent concern areas that happen to share the "runtime state cleanup" theme but touch completely different packages. S8B.1 consolidates `cmd/wikismit/` client factories into a shared helper. S8B.2 removes package-level mutable logger state from `internal/planner` and `internal/preprocessor`. S8B.3 wraps free functions in the `internal/analyzer` package with narrow helper structs. Each plan is independently shippable and has its own verification loop.

## Pre-implementation alignment notes

1. `cmd/wikismit/helpers.go` already exists (21 lines, has `newStubCmd` and `writeCommandOutput`). S8B.1 adds the shared client factory there.
2. Three factory variables are identical in structure: `agentClientFactory` (generate.go:14-16), `plannerClientFactory` (plan.go:15-17), `updateClientFactory` (update.go:13-15). All default to `func() llm.Client { return nil }` and fall back to `llm.NewClient(cfg.LLM)`.
3. `update.go` has a cross-command dependency: when `updateClientFactory()` returns nil, it cascades to `agentClientFactory()` before falling back to `llm.NewClient`. The shared helper must preserve this fallback chain.
4. Tests mock factories via variable reassignment (`original := factory; factory = mock; defer func() { factory = original }()`). This seam pattern must be preserved.
5. Package-level logger vars: `var logger logpkg.Logger = logpkg.New(false)` in `internal/planner/skeleton.go:12` and `var sharedLogger logpkg.Logger = logpkg.New(false)` in `internal/preprocessor/shared_context.go:11`. Neither has a setter function.
6. `internal/planner/planner.go:18-23` conditionally assigns a verbose logger and defers a reset to nil. S8B.2 must thread logger ownership through the call chain instead.
7. `internal/analyzer/analyzer.go` already has an `Analyzer` struct with `registry`, `excludePatterns`, `modulePath`, `skippedFiles` fields. `BuildDepGraph` and `ComputeAffected` are pure free functions that don't use it. S8B.3 wraps them as methods on a new thin struct, keeping the original free functions as thin wrappers.
8. Final Epic 8B verification should run focused package tests first and only then run `go test ./...`.

## Commit flow

Recommended commit checkpoints:

1. shared client factory helper + command migration + regression tests
2. planner logger threading + preprocessor logger threading + regression tests
3. DepGraphBuilder + ImpactAnalyzer structs + regression tests
4. final Epic 8B regression verification (`go test ./...`)

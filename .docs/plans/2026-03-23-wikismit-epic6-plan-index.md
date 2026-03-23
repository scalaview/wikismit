# wikismit Epic 6 Plan Index

Use this index instead of writing Epic 6 from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic6.md`
2. `.docs/plans/2026-03-23-wikismit-epic6-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-23-wikismit-epic6-plan-01-gitdiff-and-affected-module-foundation.md`
2. `.docs/plans/2026-03-23-wikismit-epic6-plan-02-incremental-pipeline-and-update-cli.md`
3. `.docs/plans/2026-03-23-wikismit-epic6-plan-03-ci-example-files.md`

## Why this split exists

Epic 6 mixes three different kinds of work: deterministic change detection, partial pipeline re-execution, and user-facing CI examples. Keeping them in one document would blur the line between file-diff semantics, incremental orchestration across Phases 1/3/4/5, and documentation/examples work. This split keeps one plan for change detection and affected-module computation, one for the actual incremental runtime and `wikismit update` wiring, and one for the GitHub Actions examples and setup docs.

## Pre-implementation alignment notes

These items must be treated as part of Epic 6, not discovered midway through coding:

1. `cmd/wikismit/update.go` is still a stub command in the Epic 6 worktree. Epic 6 must replace that stub rather than introducing a second incremental command surface.
2. `pkg/gitdiff/` does not exist yet. Epic 6 should create it rather than overloading `internal/analyzer/` with git-specific behavior.
3. `internal/pipeline/` does not exist yet. If Epic 6 needs a reusable incremental orchestration entrypoint, it should create that package explicitly instead of cramming orchestration into `cmd/wikismit/update.go`.
4. Existing full-run entrypoints already exist in `internal/analyzer/phase1.go`, `internal/preprocessor/preprocessor.go`, `internal/agent/scheduler.go`, and `internal/composer/renderer.go`. Epic 6 should add minimal partial-run seams alongside those flows rather than rewriting the full pipeline.
5. `pkg/store` already returns `ErrArtifactNotFound` for missing artifacts. Epic 6 should reuse that contract for the "fall back to full generate" path instead of inventing a second missing-artifact error shape.
6. `examples/github/` is effectively empty and there is no `Makefile` in this worktree. CI-example validation should use documented commands such as `actionlint` rather than inventing a new build surface.
7. Baseline verification in `.worktrees/epic6` is currently clean: the targeted repo-override test and `go test ./...` both pass before any Epic 6 edits. Treat any later failure as introduced by Epic 6 unless proven otherwise.
8. The current implementation uses value-based `llm.CompletionRequest` and `agent.AgentInput` types even though the spec examples show pointer-shaped interfaces. Epic 6 plans should follow the current codebase conventions unless a concrete bug forces a broader refactor.

## Commit flow

Recommended commit checkpoints:

1. `pkg/gitdiff` dependency + parser tests + changed-file parsing
2. affected-module computation helpers + analyzer tests
3. incremental pipeline package scaffold + changed-file reanalysis
4. partial shared/module rerun seams + `wikismit update` command wiring
5. fallback/incremental CLI verification + regression tests
6. GitHub Actions examples + README + workflow validation notes

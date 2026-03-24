# wikismit Epic 7H Plan Index

Use this index instead of writing Epic 7H from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic7h.md`
2. `.docs/plans/2026-03-24-wikismit-epic7h-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-24-wikismit-epic7h-plan-01-verbose-config-and-logger-seams.md`
2. `.docs/plans/2026-03-24-wikismit-epic7h-plan-02-llm-and-planner-debug-logging.md`
3. `.docs/plans/2026-03-24-wikismit-epic7h-plan-03-incremental-fallback-phase-logging.md`

## Why this split exists

Epic 7H mixes three adjacent but still separable concerns: CLI/config plumbing for verbose mode, request-level diagnostics around Phase 2 and LLM calls, and fallback full-generate phase timing in the incremental runtime. Keeping them in one document would blur the boundary between enabling verbose mode, capturing LLM/planner metadata, and instrumenting phase orchestration. This split keeps one plan for plumbing and test seams, one for LLM and planner diagnostics, and one for fallback phase timing and final verification.

## Pre-implementation alignment notes

These items must be treated as part of Epic 7H, not discovered midway through coding:

1. `cmd/wikismit/main.go` already declares `--verbose`; Epic 7H must reuse that flag instead of inventing a second one.
2. `internal/log/log.go` already provides the repo’s logger wrapper; Epic 7H should reuse it instead of introducing a new logging dependency or package.
3. `internal/config.Config` currently has no verbose/runtime logging field, so config plumbing is required before package-level logging can follow shared runtime state.
4. `internal/llm/client.go`, `internal/planner/planner.go`, and `internal/pipeline/incremental.go` currently contain no verbose debug logs, so tests must lock the desired metadata surface before implementation.
5. Prompt diagnostics must log size metadata only; do not log full prompt bodies or sensitive config values such as API keys.
6. The fallback phase logs belong only to the full-generate path inside incremental mode; do not broaden scope into unrelated command-progress reporting.
7. Baseline verification in `.worktrees/epic6-debug-log` is clean: `go test ./...` passes before any Epic 7H edits. Treat later failures as introduced by this feature unless proven otherwise.

## Commit flow

Recommended commit checkpoints:

1. verbose config plumbing + logger capture seam + focused tests
2. LLM client request diagnostics + tests
3. planner debug diagnostics + tests
4. incremental fallback phase timing logs + tests
5. final regression verification

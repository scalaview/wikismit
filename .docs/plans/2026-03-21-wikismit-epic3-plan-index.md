# wikismit Epic 3 Plan Index

Use this index instead of writing Epic 3 from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic3.md`
2. `.docs/plans/2026-03-21-wikismit-epic3-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-21-wikismit-epic3-plan-01-skeleton-and-planner-foundation.md`
2. `.docs/plans/2026-03-21-wikismit-epic3-plan-02-phase2-nav-plan-and-cli.md`
3. `.docs/plans/2026-03-21-wikismit-epic3-plan-03-shared-preprocessor.md`

## Why this split exists

Epic 3 mixes token-budget-aware skeleton building, one-shot planner prompting, JSON validation and retry behavior, shared-module dependency ordering, and serial shared-context generation. Keeping those in one document would make the Phase 2 vs Phase 3 boundary blurry and would hide the fact that planner validation and shared-module ordering are separate correctness concerns. This split keeps one foundation plan for skeleton construction, one plan for finishing Phase 2, and one plan for the Phase 3 preprocessor lane.

## Pre-implementation alignment notes

These items must be treated as part of Epic 3, not discovered midway through coding:

1. `pkg/store.SharedSummary` already includes `SourceRefs []string`, while the spec example for `shared_context.json` only shows `summary`, `key_types`, and `key_functions`. Epic 3 implementation should preserve the checked-in store type and make the plan explicit about when `source_refs` is populated versus when grounded refs stay inside `key_functions[].ref`.
2. `pkg/store.NavPlan` includes `GeneratedAt time.Time`, but the planner task text mostly discusses the `modules` array. The planner flow should explicitly set `generated_at` after successful validation so the artifact matches the checked-in schema.
3. `cmd/wikismit/plan.go` already exists as a stub. Epic 3 should replace that stub instead of inventing a second planning command surface.
4. Baseline verification in the fresh Epic 3 worktree currently shows a pre-existing `go test ./...` failure in `cmd/wikismit` due to `../../testdata/sample_repo/vendor` lookup. Treat that as existing master-state context unless Epic 3 changes require touching the same tests.

## Commit flow

Recommended commit checkpoints:

1. planner package scaffold + skeleton token accounting
2. skeleton truncation + serialization tests
3. planner prompt + JSON retry loop
4. nav plan validation + artifact write
5. `plan` command wiring
6. shared-module subgraph + topo sort
7. shared preprocessor prompt + summary grounding
8. preprocessor orchestration + final verification fixes

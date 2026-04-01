# wikismit Epic 9A Plan Index

Use this index instead of executing `.docs/tasks/wikismit-epic9a-bug-fixes.md` directly.

## Read first

1. `.docs/tasks/wikismit-epic9a-bug-fixes.md`
2. `.docs/spec/code-review-improvement.md` (§2.2, §2.6)

## Execution order

1. `.docs/plans/2026-04-01-wikismit-epic9a-plan-01-fix-dependency-depth-module-graph.md`
2. `.docs/plans/2026-04-01-wikismit-epic9a-plan-02-fix-continuation-loop-cap.md`
3. `.docs/plans/2026-04-01-wikismit-epic9a-plan-03-fix-broken-link-line-number.md`

## Why this split exists

Epic 9A contains three verified bugs in three separate packages (`composer/renderer`, `llm/client`, `composer/validator`). Each bug has a different root cause, different test file, and different verification loop. Splitting by bug keeps each plan self-contained and independently shippable — if one fix requires iteration, the others are unaffected.

## Pre-implementation alignment notes

1. `GenerateIndexPage` (renderer.go:88) receives a file-level `DepGraph` (keys = file paths) but calls `dependencyDepth` with module IDs as keys. The keys never match, so depth is always 0. Fix must construct a module-level graph from `NavPlan.Module.DependsOnShared` and `Module.ReferencedBy` fields.
2. The existing test `TestGenerateIndexPageListsModulesByDependencyDepth` (renderer_test.go:121) passes a synthetic graph with module IDs as keys — this masks the bug. The fix must add a test that uses a realistic file-level graph to prove the old code fails and the new code works.
3. `openAIClient.Complete` (client.go:73) uses `for true` with zero continuation limits. Epic 8A.3 already changed it to `for {}` in the plan but the current codebase still has `for true`. This plan adds the cap (max 5 continuations) on top.
4. `ValidateDocs` (validator.go:53) hardcodes `Line: 0` for every broken link. The regex match byte offsets from `FindAllStringSubmatchIndex` can be converted to 1-based line numbers by counting newlines before the match start position.
5. All three fixes are strictly additive — no refactoring, no signature changes to exported functions, no cross-package dependencies.

## Commit flow

Recommended commit checkpoints:

1. `buildModuleGraph` function + module-level sorting tests + `GenerateIndexPage` refactor
2. continuation cap constant + continuation limit tests + `for true` → capped loop
3. line number calculation tests + `Line: 0` → computed line number
4. final Epic 9A regression: `go test ./...`

# wikismit Epic 8A Plan Index

Use this index instead of executing `.docs/tasks/wikismit-epic8a.md` directly.

## Read first

1. `.docs/tasks/wikismit-epic8a.md`
2. `.docs/spec/code-review-oop-refactoring.md`

## Execution order

1. `.docs/plans/2026-03-28-wikismit-epic8a-plan-01-fresh-run-full-generate-orchestration.md`
2. `.docs/plans/2026-03-28-wikismit-epic8a-plan-02-build-output-directory-hardening.md`
3. `.docs/plans/2026-03-28-wikismit-epic8a-plan-03-llm-request-validation-and-loop-cleanup.md`

## Why this split exists

Epic 8A combines three adjacent but independently shippable runtime-correctness slices: the `generate` command entrypoint, the `build` command’s output-directory handling, and the LLM client boundary. Each slice has a different failure mode, different tests, and a different verification loop. Splitting the epic here keeps each plan small enough to execute safely while still rolling up to the single Epic 8A goal.

## Pre-implementation alignment notes

1. `cmd/wikismit/generate.go:42-90` still owns a stale command-side orchestration path, but `internal/pipeline/incremental.go:34-82` already contains the canonical five-phase full-generate implementation. Epic 8A.1 should delete duplication instead of extracting a second helper.
2. `cmd/wikismit/update.go:38-56` already prints the “fell back to full generate” success message by checking `file_index.json` before `pipeline.RunIncremental`. Preserve that behavior while rewiring `generate`.
3. Existing generate tests mix command responsibilities with pipeline responsibilities. Once `generate` becomes a thin delegate, command tests that assert direct phase-4 filtering or direct dep-graph reads will be stale and must be rewritten or removed only when equivalent pipeline coverage already exists.
4. `cmd/wikismit/build.go:39-72` currently uses `cfg.OutputDir` directly for config discovery, `filepath.Join`, and `cmd.Dir`. Epic 8A.2 should validate once, reuse the validated directory for subprocess execution, and keep the npm/npx branch structure unchanged.
5. Build coverage currently misses the “output path is a file” case and does not lock the bare `npx vitepress build docs` success path. Add those tests before touching build logic.
6. `internal/llm/client.go:62-90` still uses `for true` and performs no local request validation before `CreateChatCompletion` at line 119. Epic 8A.3 must fail locally for invalid requests without redesigning retry logic, provider transport, or logging scope.
7. Final Epic 8A verification should run focused package tests first and only then run `go test ./...`.

## Commit flow

Recommended commit checkpoints:

1. generate fresh-run delegation tests + command cleanup
2. build output-directory validation + branch-preserving tests
3. LLM request validation + continuation-loop cleanup
4. final Epic 8A regression verification

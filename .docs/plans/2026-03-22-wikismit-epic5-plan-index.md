# wikismit Epic 5 Plan Index

Use this index instead of writing Epic 5 from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic5.md`
2. `.docs/plans/2026-03-22-wikismit-epic5-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-22-wikismit-epic5-plan-01-citation-and-validation-foundation.md`
2. `.docs/plans/2026-03-22-wikismit-epic5-plan-02-renderer-and-phase5-generate-integration.md`
3. `.docs/plans/2026-03-22-wikismit-epic5-plan-03-vitepress-build-and-cli-surface.md`

## Why this split exists

Epic 5 mixes three different correctness concerns: grounding generated Markdown back to source via citations and broken-link validation, assembling deterministic `docs/` output from Phase 4 artifacts, and generating the VitePress deployment surface plus the `build` command. Keeping those in one document would blur the line between low-level Markdown transformation, Phase 5 orchestration, and Node/VitePress command behavior. This split keeps one plan for citation and validation primitives, one plan for renderer/composer integration, and one plan for VitePress output plus CLI build wiring.

## Pre-implementation alignment notes

These items must be treated as part of Epic 5, not discovered midway through coding:

1. `internal/composer/` does not exist yet in the Epic 5 worktree. Epic 5 should create that package rather than spreading Phase 5 logic across `cmd/wikismit/`.
2. `cmd/wikismit/generate.go` currently stops after Phase 4 and already reads `file_index.json`, `nav_plan.json`, and `shared_context.json`. Epic 5 should extend that existing flow rather than inventing a second composition command.
3. `cmd/wikismit/validate.go` and `cmd/wikismit/build.go` are still stub commands. Epic 5 should replace those stubs with real Phase 5 validation and VitePress build behavior.
4. `pkg/store` currently owns artifact schemas and JSON read/write helpers. Because `WriteValidationReport` belongs in `pkg/store`, the validation report types should live in `pkg/store` as well so the write path stays dependency-safe.
5. Baseline verification in `.worktrees/epic5` is currently clean: `go mod download` and `go test ./...` both pass before any Epic 5 edits. Treat any later failure as introduced by Epic 5 unless proven otherwise.
6. Phase 5 is deterministic per the spec. Do not add LLM calls, background processing, or new command surfaces while implementing citation injection, rendering, validation, or VitePress generation.

## Commit flow

Recommended commit checkpoints:

1. composer package scaffold + symbol map and citation injection tests
2. validation report types + validator implementation + `validate` command wiring
3. TOC generation + module/shared doc copy helpers
4. `index.md` generation + Phase 5 composer orchestration + `generate` wiring
5. VitePress config template + logo handling + config tests
6. `build` command wiring + final Epic 5 verification

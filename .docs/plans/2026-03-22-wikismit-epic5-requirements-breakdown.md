# wikismit Epic 5 Requirements Breakdown

## Goal

Split Epic 5 into smaller requirement slices before writing implementation plans.

Epic 5 target from `.docs/tasks/wikismit-epic5.md`, `.docs/spec/wikismit-tech-spec.md`, and `.docs/spec/wikismit-tasks.md` is:

- build deterministic citation injection from `file_index.json` into generated module Markdown
- validate internal cross-references across the composed `docs/` tree and write `artifacts/validation_report.json`
- assemble final `docs/modules/`, `docs/shared/`, and `docs/index.md` output from `artifacts/module_docs/`
- generate `docs/.vitepress/config.ts` from nav-plan and site-config data
- wire Phase 5 into `wikismit generate`, `wikismit validate`, and `wikismit build`

## Requirement slices

### R1 — Citation injection foundation and validation report storage

Source: `S5.1`, `S5.2.1`, spec §4 Phase 5, spec §5 artifact schemas, spec §13 testing strategy.

Includes:

- create `internal/composer/citation.go` and `internal/composer/citation_test.go`
- build a canonical symbol map from `store.FileIndex` covering functions and types
- implement exported-symbol citation injection that skips already-linked names and unknown identifiers
- create validation report types in `pkg/store`
- add `WriteValidationReport(dir string, report ValidationReport) error` and matching tests
- lock ambiguity handling and logging behavior for duplicate symbol names

Output:

- Phase 5 has a deterministic citation primitive grounded on Phase 1 artifacts
- validation-report persistence exists before validator or renderer orchestration starts

### R2 — Cross-reference validator and `validate` command wiring

Source: `S5.2`, spec §4 Phase 5, spec §10 CLI design.

Includes:

- create `internal/composer/validator.go` and `internal/composer/validator_test.go`
- scan Markdown links, filter to internal non-anchor targets, and resolve them relative to source files
- populate `store.ValidationReport` with broken-link metadata and totals
- replace the `cmd/wikismit/validate.go` stub with real validation orchestration
- write `artifacts/validation_report.json` through `pkg/store`
- keep v1 behavior non-blocking: broken links report warnings but do not turn into a non-zero exit status

Output:

- `wikismit validate` becomes a real Phase 5 validation entrypoint
- broken cross-references are inspectable both in CLI output and persisted artifact form

### R3 — Renderer primitives and Phase 5 orchestration in `generate`

Source: `S5.3`, spec §4 Phase 5, spec §16 documentation deployment.

Includes:

- create `internal/composer/renderer.go` and `internal/composer/renderer_test.go`
- implement TOC generation and heading-anchor normalization
- copy module docs into `docs/modules/` and `docs/shared/` with citation injection applied
- generate `docs/index.md` from `store.NavPlan` and `store.DepGraph`
- implement `RunComposer(cfg, plan, idx, graph)` as the deterministic Phase 5 entrypoint
- extend `cmd/wikismit/generate.go` to load `dep_graph.json`, call `RunComposer`, and continue using the existing generate surface

Output:

- Phase 5 can deterministically transform Phase 4 artifacts into a browsable Markdown docs tree
- `wikismit generate` reaches the end of Phase 5 after Phase 4 succeeds

### R4 — VitePress config generation and `build` command behavior

Source: `S5.4`, spec §16 documentation deployment, spec §10 CLI design.

Includes:

- create `internal/composer/vitepress.go` and `internal/composer/vitepress_test.go`
- generate `docs/.vitepress/config.ts` from nav-plan ordering plus `cfg.Site` settings
- copy the optional site logo into `docs/public/logo.png`
- extend `RunComposer` to write the VitePress config after Markdown output exists
- replace the `cmd/wikismit/build.go` stub with a real command that verifies prerequisites and runs `vitepress build`
- verify the build command handles missing config, missing Node.js, and first-run dependency install behavior clearly

Output:

- Phase 5 produces deployment-ready VitePress configuration automatically
- `wikismit build` becomes a thin wrapper around local VitePress site generation

## Dependency order

Implement in this order:

1. `R1` citation injection + validation report storage
2. `R2` validator + `validate` command
3. `R3` renderer + Phase 5 `generate` integration
4. `R4` VitePress config + `build` command

## Implementation document map

- `R1 + R2` → `.docs/plans/2026-03-22-wikismit-epic5-plan-01-citation-and-validation-foundation.md`
- `R3` → `.docs/plans/2026-03-22-wikismit-epic5-plan-02-renderer-and-phase5-generate-integration.md`
- `R4` → `.docs/plans/2026-03-22-wikismit-epic5-plan-03-vitepress-build-and-cli-surface.md`

## Out of scope for Epic 5

Do not implement yet:

- incremental update mode and affected-module recomputation
- citation coverage metrics and symbol-coverage quality analysis from Epic 7E/8
- Mermaid dependency diagrams and advanced visual documentation output
- multi-language parser expansion or any change to earlier phase artifact schemas beyond the new validation-report artifact

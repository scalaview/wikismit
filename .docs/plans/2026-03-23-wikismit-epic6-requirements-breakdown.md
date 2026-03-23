# wikismit Epic 6 Requirements Breakdown

## Goal

Split Epic 6 into smaller requirement slices before writing implementation plans.

Epic 6 target from `.docs/tasks/wikismit-epic6.md`, `.docs/spec/wikismit-tech-spec.md`, and `.docs/spec/wikismit-tasks.md` is:

- detect changed files from git refs or an explicit `--changed-files` override
- compute the minimal affected module set from `nav_plan.json` and `dep_graph.json`
- re-run only the required pipeline phases for changed modules while preserving full Phase 5 composition
- expose the incremental flow through a real `wikismit update` command
- ship GitHub Actions examples and setup docs for full and incremental deployment

## Requirement slices

### R1 — Git diff parsing and affected-module computation

Source: `S6.1`, `S6.2`, spec §9 Incremental Update Mode, spec §13 testing strategy.

Includes:

- create `pkg/gitdiff/diff.go` and `pkg/gitdiff/diff_test.go`
- add `github.com/go-git/go-git/v5` to `go.mod` and lock dependency behavior with tests
- implement `ChangeType`, `FileChange`, `GetChangedFiles`, and `ParseChangedFiles`
- create `internal/analyzer/affected.go` and `internal/analyzer/affected_test.go`
- map changed files to owning modules and propagate upstream dependents through a reverse dependency graph
- prove the affected-module set against `testdata/sample_repo/`

Output:

- Epic 6 has a deterministic changed-file input surface
- incremental orchestration can consume a tested affected-module set instead of guessing which modules to rerun

### R2 — Incremental pipeline orchestration primitives

Source: `S6.3.1`–`S6.3.4`, spec §9 Incremental Update Mode, spec §14 JSON artifacts over in-memory pipeline.

Includes:

- create `internal/pipeline/incremental.go` and matching tests
- implement `RunIncremental` around existing Phase 1/3/4/5 helpers
- implement changed-file reanalysis and in-place `file_index.json` updates
- add partial-run seams for shared-module preprocessing and non-shared module fan-out
- reuse existing artifact store contracts for fallback and persistence

Output:

- the repo gains a real reusable incremental pipeline entrypoint
- partial reruns are grounded in existing artifact files instead of a second shadow pipeline

### R3 — `wikismit update` CLI wiring and fallback behavior

Source: `S6.3.5`, `S6.3.6`, spec §9 Incremental Update Mode, spec §10 CLI Design.

Includes:

- replace the `cmd/wikismit/update.go` stub with a real command
- add `--base-ref`, `--head-ref`, and `--changed-files` flags
- wire the command to `RunIncremental`
- verify fallback to full `generate` when artifacts are absent
- verify CLI output, changed-file override behavior, and partial rerun call counts through `cmd/wikismit/main_test.go`

Output:

- `wikismit update` becomes the public Epic 6 surface
- incremental mode is covered end-to-end from CLI flags through artifact writes

### R4 — CI example files and deployment documentation

Source: `S6.4`, spec §16 Documentation Deployment — VitePress.

Includes:

- create `examples/github/docs-full.yml`
- create `examples/github/docs-incremental.yml`
- create `examples/github/README.md`
- document workflow validation using `actionlint` without adding a new `Makefile`
- keep the examples aligned with the actual `generate`, `update`, and `build` CLI behavior shipped by the repo

Output:

- users get copyable GitHub Actions examples for both full and incremental flows
- deployment docs stay consistent with the real CLI and current repo layout

## Dependency order

Implement in this order:

1. `R1` git diff + affected-module computation
2. `R2` incremental pipeline primitives
3. `R3` `update` CLI wiring + fallback/integration verification
4. `R4` CI examples + README

## Implementation document map

- `R1` → `.docs/plans/2026-03-23-wikismit-epic6-plan-01-gitdiff-and-affected-module-foundation.md`
- `R2 + R3` → `.docs/plans/2026-03-23-wikismit-epic6-plan-02-incremental-pipeline-and-update-cli.md`
- `R4` → `.docs/plans/2026-03-23-wikismit-epic6-plan-03-ci-example-files.md`

## Out of scope for Epic 6

Do not implement yet:

- artifact caching logic from spec §8 (`artifacts/cache/`) beyond current baseline behavior
- robust incremental hardening scenarios from Epic 7F
- new deployment targets beyond documented GitHub Pages and Cloudflare Pages examples
- changes to full-run Phase 2 planning behavior or Phase 5 output semantics unrelated to incremental reruns

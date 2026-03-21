# wikismit Epic 2 Requirements Breakdown

## Goal

Split Epic 2 into smaller requirement slices before writing implementation plans.

Epic 2 target from `.docs/tasks/wikismit-epic2.md` and `.docs/spec/wikismit-tech-spec.md` is:

- parse Go source files deterministically with tree-sitter
- produce `artifacts/file_index.json` with symbol and import metadata
- produce `artifacts/dep_graph.json` with internal dependency edges
- wire Phase 1 into `wikismit generate`
- keep output deterministic and idempotent

## Requirement slices

### R1 — Parser contracts, dependency baseline, and schema alignment

Source: `S2.1.1`, `S2.1.2`, spec §4, spec §5, spec CLI section.

Includes:

- add parser dependencies needed for Go AST parsing and golden diff testing
- define `LanguageParser` and extension registry in `internal/analyzer/parser.go`
- register the Go parser via `init()`
- align in-memory/store types before parser work starts:
  - add `TypeDecl.LineEnd` because spec examples require `line_start` and `line_end` for types
  - add `Import.ResolvedPath` as an in-memory helper field with `json:"-"` so dep graph code can use resolved file targets without changing serialized artifact shape
- note the external reference correction: use official runtime + grammar split modules, not the stale single import path shown in the Epic 2 task text

Output:

- parser contracts exist before traversal code
- store types can represent all Phase 1 data needed by the spec and task flow
- no schema surprise remains for later slices

### R2 — Go parser extraction and golden fixtures

Source: `S2.1.3` through `S2.1.7`, spec §4, spec `file_index.json` example.

Includes:

- parser creation helper using official tree-sitter Go grammar
- function extraction with signatures, line ranges, and exported detection
- type extraction with `struct` / `interface` / `alias`
- import extraction for single and grouped imports
- `content_hash` generation from file bytes
- golden fixtures under `testdata/fixtures/golang/`
- deterministic JSON comparison in tests

Output:

- a single-file Go analyzer can produce spec-shaped `FileEntry` values
- golden fixtures lock the extraction contract before repo traversal starts

### R3 — Repository traversal and sample repository fixture

Source: `S2.2`, config analysis section, spec Phase 1 input/output.

Includes:

- synthetic `testdata/sample_repo/` fixture with enough internal imports to exercise traversal and dep graph work
- `Analyzer` struct with config, registry access, logger, compiled exclude patterns, and skipped-file accounting
- traversal via `filepath.WalkDir`
- extension filtering, exclude-pattern filtering, parser dispatch, relative-path indexing
- warning-and-continue behavior on parse failures

Output:

- repo-wide `FileIndex` generation works on a controlled sample repo
- later dep graph tests can reuse the same fixture instead of inventing new cases

### R4 — Module path resolution and dependency graph construction

Source: `S2.3`, spec `dep_graph.json` example.

Includes:

- `go.mod` module path loading via `golang.org/x/mod/modfile`
- import classification into internal vs external
- resolution of internal imports onto concrete repo file paths
- adjacency-list construction with every indexed file present as a map key
- graph tests for shared/internal/external cases

Output:

- `FileEntry.Imports` carry correct `Internal` flags
- `DepGraph` is complete and deterministic for the sample repo

### R5 — Phase 1 orchestration, CLI wiring, and idempotency

Source: `S2.4`, spec CLI section, spec artifact schemas.

Includes:

- `RunPhase1(cfg *config.Config) error`
- artifact writes through `pkg/store`
- progress and skipped-file logging
- `generate` command wiring
- add missing CLI override flags required by the spec-driven smoke flow:
  - `--repo`
  - `--output`
  - `--artifacts`
- idempotency and end-to-end smoke tests

Output:

- `wikismit generate` can execute Phase 1 against a sample repo and write deterministic artifacts

## Dependency order

Implement in this order:

1. `R1` parser contracts + schema alignment
2. `R2` Go parser + fixtures
3. `R3` traversal + sample repo
4. `R4` dep graph + import resolution
5. `R5` orchestration + CLI wiring + final verification

## Implementation document map

- `R1 + R2` → `.docs/plans/2026-03-20-wikismit-epic2-plan-01-parser-and-go-extraction.md`
- `R3` → `.docs/plans/2026-03-20-wikismit-epic2-plan-02-traverser-and-sample-repo.md`
- `R4 + R5` → `.docs/plans/2026-03-20-wikismit-epic2-plan-03-dep-graph-and-phase1.md`

## Out of scope for Epic 2

Do not implement yet:

- Phase 2 module planner
- Phase 3 shared preprocessor
- Phase 4 agent fan-out
- Phase 5 composer and validation
- multi-language parsing beyond Go
- incremental update mode

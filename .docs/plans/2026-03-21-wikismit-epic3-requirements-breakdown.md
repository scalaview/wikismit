# wikismit Epic 3 Requirements Breakdown

## Goal

Split Epic 3 into smaller requirement slices before writing implementation plans.

Epic 3 target from `.docs/tasks/wikismit-epic3.md` and `.docs/spec/wikismit-tech-spec.md` is:

- build token-budget-aware repository and module skeletons from Phase 1 artifacts
- run one planner LLM call to produce validated `artifacts/nav_plan.json`
- lock shared-module ownership before any parallel documentation generation
- compute dependency-safe ordering for shared modules
- run serial shared-module summarization to produce `artifacts/shared_context.json`
- wire the `wikismit plan` command to stop after Phase 2 while keeping Phase 3 available as library orchestration

## Requirement slices

### R1 — Skeleton serialization and planner input foundation

Source: `S3.1`, spec §4 Phase 2, spec §13 testing strategy.

Includes:

- create `internal/planner/skeleton.go` and `internal/planner/skeleton_test.go`
- implement approximate token counting for budget enforcement
- serialize file-level signatures and type names with `// path.go:N` annotations
- exported-first truncation rules for both per-module and full-repo skeletons
- warning behavior when symbols are dropped because of the token budget

Output:

- deterministic skeleton text exists for both planner-wide and module-specific prompt building
- token-budget truncation rules are locked before any LLM integration work starts

### R2 — Planner prompt, LLM retry loop, and `nav_plan.json` validation

Source: `S3.2`, spec §4 Phase 2, spec `nav_plan.json` schema, spec §8 LLM integration, spec §10 CLI design.

Includes:

- create `internal/planner/planner.go`, `internal/planner/prompt.go`, and `internal/planner/planner_test.go`
- build the Phase 2 architect prompt from the full-repo skeleton
- call `llm.Client` once per attempt using `cfg.LLM.PlannerModel`
- parse planner JSON into `store.NavPlan`
- retry on malformed JSON or validation failures up to three attempts with error context appended
- validate that every indexed file appears exactly once and that owner values are valid
- set `GeneratedAt` and write the artifact via `pkg/store`

Output:

- Phase 2 can turn `file_index.json` + `dep_graph.json` into a validated `nav_plan.json`
- planner retries are deterministic and testable through the existing mock LLM client

### R3 — Plan command wiring and Phase 2 integration flow

Source: `S3.2.5`, spec §10 CLI design, spec §13 testing strategy.

Includes:

- replace the `cmd/wikismit/plan.go` stub with real Phase 1 + Phase 2 orchestration
- reuse existing Phase 1 analyzer output path instead of duplicating planner input loading rules
- print a stable success message with the written `nav_plan.json` path
- add command-level tests proving `plan` stops after writing `nav_plan.json`

Output:

- `wikismit plan` becomes the inspection-friendly entrypoint for Phase 1 + 2 only
- CLI behavior is locked before shared-preprocessor work begins

### R4 — Shared-module dependency extraction and topological ordering

Source: `S3.3`, spec §4 Phase 3.

Includes:

- create `internal/preprocessor/preprocessor.go` and `internal/preprocessor/topo_test.go`
- derive a module-level shared subgraph from `store.NavPlan` plus `store.DepGraph`
- implement Kahn’s algorithm for deterministic topological ordering
- detect and report cycles among shared modules
- return clean empty results when no shared modules exist

Output:

- shared modules can be processed in dependency order before any LLM summary generation happens
- cycle handling is explicit rather than discovered deep inside the Phase 3 run path

### R5 — Shared preprocessor prompts, summary grounding, and `shared_context.json`

Source: `S3.4`, spec §4 Phase 3, spec `shared_context.json` schema, spec §8 LLM integration.

Includes:

- create `internal/preprocessor/shared_context.go`, `internal/preprocessor/prompt.go`, and `internal/preprocessor/preprocessor_test.go`
- build one shared-module summary prompt at a time, injecting already-completed shared summaries for upstream dependencies
- parse LLM JSON responses into `store.SharedSummary`
- ground `key_functions[].ref` values against `file_index.json` and log warning fallback cases
- decide and test how `SourceRefs` is populated so the checked-in store shape stays coherent
- write completed `shared_context.json` via `pkg/store`

Output:

- Phase 3 can serially produce shared-module summaries that are ready for later Phase 4 prompt injection
- summary references are grounded against Phase 1 artifact data rather than trusting raw model output

## Dependency order

Implement in this order:

1. `R1` skeleton serialization + token budgeting
2. `R2` planner prompt + JSON retry + validation
3. `R3` `plan` command wiring + Phase 2 integration
4. `R4` shared-module subgraph + topo sort
5. `R5` shared preprocessor prompts + orchestration + artifact write

## Implementation document map

- `R1` → `.docs/plans/2026-03-21-wikismit-epic3-plan-01-skeleton-and-planner-foundation.md`
- `R2 + R3` → `.docs/plans/2026-03-21-wikismit-epic3-plan-02-phase2-nav-plan-and-cli.md`
- `R4 + R5` → `.docs/plans/2026-03-21-wikismit-epic3-plan-03-shared-preprocessor.md`

## Out of scope for Epic 3

Do not implement yet:

- Phase 4 agent fan-out
- Phase 5 composer, citation injection, and VitePress generation
- incremental update mode
- multi-language parser expansion beyond the Go-based Phase 1 artifacts already produced
- response cache behavior beyond using the existing config surface

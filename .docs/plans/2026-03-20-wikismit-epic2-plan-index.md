# wikismit Epic 2 Plan Index

Use this index instead of writing Epic 2 from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic2.md`
2. `.docs/plans/2026-03-20-wikismit-epic2-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-20-wikismit-epic2-plan-01-parser-and-go-extraction.md`
2. `.docs/plans/2026-03-20-wikismit-epic2-plan-02-traverser-and-sample-repo.md`
3. `.docs/plans/2026-03-20-wikismit-epic2-plan-03-dep-graph-and-phase1.md`

## Why this split exists

Epic 2 mixes parser work, repository traversal, schema alignment, dep-graph logic, and CLI wiring. Keeping those in one document makes TDD sequencing fuzzy and hides prerequisite alignment work. This split keeps each plan tied to one implementation lane while preserving a clean dependency order.

## Pre-implementation alignment notes

These items must be treated as part of Epic 2, not discovered midway through coding:

1. `pkg/store.TypeDecl` needs `line_end` to match the Phase 1 artifact shape.
2. `pkg/store.Import` needs a non-serialized `ResolvedPath` field so dep graph code can resolve targets without changing artifact JSON.
3. The task/spec smoke flow assumes CLI override flags like `--repo`, but the current CLI only exposes `--config` and `--verbose`.
4. Use the official tree-sitter Go modules discovered in external research:
   - `github.com/tree-sitter/go-tree-sitter`
   - `github.com/tree-sitter/tree-sitter-go/bindings/go`

## Commit flow

Recommended commit checkpoints:

1. parser contracts + schema alignment
2. Go parser bootstrap
3. Go extraction + golden fixtures
4. sample repo fixture + traverser
5. dep graph resolution
6. Phase 1 generate wiring
7. final verification fixes

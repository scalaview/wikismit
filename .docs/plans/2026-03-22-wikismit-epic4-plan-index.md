# wikismit Epic 4 Plan Index

Use this index instead of writing Epic 4 from the single task file directly.

## Read first

1. `.docs/tasks/wikismit-epic4.md`
2. `.docs/plans/2026-03-22-wikismit-epic4-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-22-wikismit-epic4-plan-01-agent-prompt-foundation.md`
2. `.docs/plans/2026-03-22-wikismit-epic4-plan-02-scheduler-and-collection.md`
3. `.docs/plans/2026-03-22-wikismit-epic4-plan-03-agent-runner-and-generate-integration.md`

## Why this split exists

Epic 4 mixes three different correctness concerns: prompt ownership enforcement, concurrent scheduling with partial-failure handling, and integration of Phase 4 into the existing generate pipeline. Keeping those in one document would blur the line between prompt construction, goroutine orchestration, and CLI/runtime behavior. This split keeps one plan for the prompt and type foundation, one for concurrency and collection semantics, and one for single-agent execution plus `generate` command integration.

## Pre-implementation alignment notes

These items must be treated as part of Epic 4, not discovered midway through coding:

1. `shared_context.json` is already grounded and filtered for direct shared dependencies upstream, so Epic 4 prompt building should consume `Module.DependsOnShared` as the declared contract rather than rediscovering dependencies from the raw graph.
2. `internal/agent/` already exists as a package path in the spec and directory layout, but the implementation files do not exist yet. Epic 4 should fill that package rather than inventing a parallel command-side orchestration layer.
3. `cmd/wikismit/generate.go` still needs to remain the single high-level entrypoint. Epic 4 should extend it after Phase 3 instead of creating a second fan-out command surface.
4. Phase 4 partial failures are explicitly allowed in v1. The scheduler and collector should preserve successful outputs, report failures, and avoid turning one module failure into a full-process hard stop.
5. The spec uses `depends_on_shared` as the prompt input contract, while the runtime concurrency limit comes from `cfg.Agent.Concurrency`. The plan should keep those two concerns separate so prompt tests do not accidentally become scheduler tests.

## Commit flow

Recommended commit checkpoints:

1. agent package types + prompt builder scaffold
2. prompt shared-context injection + ownership constraint tests
3. scheduler semaphore + results channel foundation
4. collector write path + partial-failure handling
5. concurrency limit tests + scheduler integration verification
6. single-agent execution + timing logs
7. Phase 4 summary reporting + `generate` wiring
8. full Epic 4 verification + smoke fixes

# wikismit Epic 1 Plan Index

Use this index instead of the old single-file plan.

## Read first

1. `.docs/tasks/wikismit-epic1.md`
2. `.docs/plans/2026-03-19-wikismit-epic1-requirements-breakdown.md`

## Execution order

1. `.docs/plans/2026-03-19-wikismit-epic1-plan-01-scaffold-config.md`
2. `.docs/plans/2026-03-19-wikismit-epic1-plan-02-cli-surface.md`
3. `.docs/plans/2026-03-19-wikismit-epic1-plan-03-llm-client.md`
4. `.docs/plans/2026-03-19-wikismit-epic1-plan-04-retry-mock.md`
5. `.docs/plans/2026-03-19-wikismit-epic1-plan-05-artifact-store-verification.md`

## Why this split exists

The previous Epic 1 plan was too large to execute comfortably. This split keeps each plan aligned to one requirement slice or one pair of tightly related slices.

## Commit flow

Recommended commit checkpoints:

1. scaffold
2. config
3. CLI
4. LLM core
5. retry + logging
6. mock client
7. artifact store
8. final verification fixes

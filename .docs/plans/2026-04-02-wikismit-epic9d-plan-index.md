# Epic 9D — Feature Enhancement: Plan Index

**Branch:** `epic9d-feature-enhance`
**Base:** `master` (ad2ba1e)

---

## Plan overview

| Plan | Title | Depends on | Files |
|------|-------|------------|-------|
| 01 | Extend NavPlan with Architecture Summary | — | `pkg/store/artifacts.go`, `internal/planner/prompt.go`, `internal/planner/planner.go` |
| 02 | Inject Cross-Module Context into Agent Prompts | 01 | `internal/agent/types.go`, `internal/agent/prompt.go`, `internal/pipeline/incremental.go` |
| 03 | Generate Architecture Overview Page | 01 | `internal/composer/renderer.go`, `internal/composer/vitepress.go` |

Plan 02 and 03 are independent of each other — both depend only on Plan 01.

---

## Execution order

1. Plan 01 (must be first — defines the new types)
2. Plan 02 + Plan 03 (can be done in any order after Plan 01)
3. Final verification: `go test ./... -count=1`

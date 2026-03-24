# wikismit Epic 7H Requirements Breakdown

## Goal

Split Epic 7H into small requirement slices before writing implementation plans.

Epic 7H target from `.docs/tasks/wikismit-epic7h.md`, `.docs/spec/wikismit-tech-spec.md`, and `.docs/spec/wikismit-tasks.md` is:

- reuse the existing root `--verbose` flag instead of creating a second logging switch
- plumb verbose mode through runtime config so internal packages can emit debug logs intentionally
- log high-value LLM request metadata in `internal/llm/client.go`
- log planner prompt sizing and retry-attempt metadata in `internal/planner/planner.go`
- log fallback full-generate phase entry/exit timing in `internal/pipeline/incremental.go`
- verify that verbose logging is opt-in and does not alter normal command behavior when disabled

## Requirement slices

### R1 — Verbose config plumbing and logger testability

Source: `S7H.1`, spec §10 CLI Design, spec §11 Configuration.

Includes:

- add a runtime `Verbose` field to `internal/config.Config`
- update `cmd/wikismit/main.go` so `applyCLIOverrides` copies the existing `--verbose` flag into config
- add the smallest logger seam needed to capture debug output in tests while preserving stderr text logging in production
- prove verbose defaults/off behavior through CLI and logger tests

Output:

- internal packages can make logging decisions from shared runtime config
- tests can assert debug-log behavior without relying on flaky stderr scraping

### R2 — LLM client and planner verbose diagnostics

Source: `S7H.2`, spec §8 LLM Integration, spec §12 Error Handling and Retry Strategy.

Includes:

- add debug logging around `CreateChatCompletion` in `internal/llm/client.go`
- log request metadata: model, max tokens, timeout seconds, base URL, user prompt char count, and approximate user prompt tokens
- log request timing and error classification on completion/failure
- add planner debug logging for skeleton token estimate, prompt length, planner attempt number, and planner model
- cover both packages with focused unit tests

Output:

- users can tell whether the client never sent a request, the provider stalled, or the prompt grew too large
- planner retries become visible when Phase 2 is slow or unstable

### R3 — Incremental fallback phase timing diagnostics

Source: `S7H.3`, spec §9 Incremental Update Mode.

Includes:

- add verbose phase entry/exit logs to `runFullGenerate` inside `internal/pipeline/incremental.go`
- cover `phase1`, `planner`, `preprocessor`, `agent`, and `composer`
- include elapsed timing for each phase end log
- add fallback-path tests proving the phase logs are emitted in order only when verbose mode is enabled

Output:

- slow full-generate fallback runs become diagnosable phase by phase
- incremental update troubleshooting no longer depends on guessing which phase stalled

## Dependency order

Implement in this order:

1. `R1` verbose config plumbing + logger capture seam
2. `R2` LLM client and planner debug diagnostics
3. `R3` incremental fallback phase timing diagnostics

## Implementation document map

- `R1` → `.docs/plans/2026-03-24-wikismit-epic7h-plan-01-verbose-config-and-logger-seams.md`
- `R2` → `.docs/plans/2026-03-24-wikismit-epic7h-plan-02-llm-and-planner-debug-logging.md`
- `R3` → `.docs/plans/2026-03-24-wikismit-epic7h-plan-03-incremental-fallback-phase-logging.md`

## Out of scope for Epic 7H

Do not implement yet:

- streaming terminal progress updates
- logging of full prompt text or API secrets
- new config-file keys for logging beyond the existing CLI flag
- generalized observability for every package in the repo
- changes to retry policy, cache policy, or planner semantics unrelated to debug visibility

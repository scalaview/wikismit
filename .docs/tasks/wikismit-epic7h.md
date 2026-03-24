# wikismit — Epic 7H: Verbose Debug Logging

**Status:** `todo`
**Depends on:** Epic 6
**Goal:** `wikismit --verbose` emits targeted debug logs for the LLM client, planner, and incremental full-generate fallback path so slow or oversized requests can be diagnosed without changing normal output when verbose mode is off.
**Spec refs:** §8 LLM Integration, §9 Incremental Update Mode, §10 CLI Design, §12 Error Handling and Retry Strategy

---

## S7H.1 — Verbose flag plumbing

**Status:** `todo`

**Description:**
Plumb the existing root `--verbose` CLI flag into runtime configuration so internal packages can decide whether to emit debug logs. Reuse the existing `internal/log` package and avoid introducing a second logging configuration surface.

**Acceptance criteria:**
- Running any command without `--verbose` keeps debug logs suppressed
- Running any command with `--verbose` makes the resolved config carry verbose mode to downstream packages
- Existing non-logging behavior of `generate`, `plan`, `update`, `validate`, and `build` remains unchanged

**Files to modify:**
```
cmd/wikismit/main.go
internal/config/config.go
cmd/wikismit/main_test.go
```

### Subtasks

#### S7H.1.1 — Add runtime verbose field to config

- Add a non-YAML or CLI-populated `Verbose` field to `internal/config.Config`
- Keep config-file compatibility intact; this requirement only needs CLI-driven verbose mode

#### S7H.1.2 — Apply CLI override for verbose mode

- Update `applyCLIOverrides` so the existing root `verbose` flag populates `cfg.Verbose`
- Keep repo/output/artifacts override behavior unchanged

#### S7H.1.3 — Add CLI tests for verbose config plumbing

- Add or update tests proving `--verbose` sets `cfg.Verbose`
- Verify default command execution keeps `cfg.Verbose == false`

---

## S7H.2 — LLM and planner debug logging

**Status:** `todo`

**Description:**
Add high-value debug logs before and after LLM requests in `internal/llm/client.go` and before planner calls in `internal/planner/planner.go`. The logs must expose request size, model selection, timeout/base URL, planner retries, and LLM timing/error classification without logging the entire prompt body.

**Acceptance criteria:**
- `internal/llm/client.go` logs request start/end around `CreateChatCompletion`
- LLM debug logs include: `model`, `max_tokens`, `timeout_seconds`, `base_url`, user prompt char count, estimated user prompt tokens, request start/end timing, and error type on failure
- `internal/planner/planner.go` logs: skeleton token estimate, prompt length, planner attempt number, and current model before each LLM call
- Debug logs are emitted only when verbose mode is enabled

**Files to modify:**
```
internal/llm/client.go
internal/llm/client_test.go
internal/planner/planner.go
internal/planner/planner_test.go
internal/log/log.go
```

### Subtasks

#### S7H.2.1 — Capture debug output in logger tests

- Add the smallest logger seam needed so tests can capture `slog` output without scraping real stderr
- Keep production default behavior using stderr text logs

#### S7H.2.2 — Add LLM client verbose request logs

- Log immediately before `CreateChatCompletion`
- Log immediately after completion/failure with elapsed time and error classification
- Use prompt size metadata only, not full prompt content

#### S7H.2.3 — Add planner verbose attempt logs

- Compute or reuse skeleton token estimate before prompt assembly
- Log attempt count and prompt size before each call to `client.Complete`
- Keep existing planner retry semantics intact

---

## S7H.3 — Incremental fallback phase timing logs

**Status:** `todo`

**Description:**
Add debug logging for the fallback full-generate path inside `internal/pipeline/incremental.go`, with clear entry/exit logs for each phase. This should make it obvious whether the slowdown is in Phase 1, planner, preprocessor, agent fan-out, or composer.

**Acceptance criteria:**
- Fallback full-generate path logs `phase1`, `planner`, `preprocessor`, `agent`, and `composer` start/end points
- Each phase log includes enough context to correlate start and end timing
- Normal non-verbose output of `wikismit update` stays unchanged

**Files to modify:**
```
internal/pipeline/incremental.go
internal/pipeline/incremental_test.go
```

### Subtasks

#### S7H.3.1 — Add failing fallback phase-log tests

- Write tests that run the fallback path with verbose enabled and assert phase entry/exit logs are emitted in order

#### S7H.3.2 — Add verbose phase timing logs to fallback path

- Log start/end around:
  - phase1
  - planner
  - preprocessor
  - agent
  - composer
- Include elapsed duration for each phase end log

---

## S7H.4 — Verification

**Status:** `todo`

**Description:**
Verify the verbose logging feature in focused tests first, then run full regression. The feature is complete only when debug logs appear with `--verbose`, remain quiet without it, and existing command behavior is otherwise unchanged.

**Acceptance criteria:**
- Focused tests covering config plumbing, LLM/planner logging, and incremental fallback logging pass
- `go test ./...` passes in the `epic6-debug-log` worktree
- No new diagnostics errors remain in changed files

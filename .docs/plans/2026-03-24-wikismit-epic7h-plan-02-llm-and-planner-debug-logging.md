# Epic 7H LLM and Planner Debug Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--verbose`-gated diagnostics to the LLM client and planner so oversized prompts, slow providers, and planner retry attempts are visible without logging prompt bodies.

**Architecture:** First lock the LLM metadata surface with focused client tests so request timing and error classification are stable. Then add planner attempt logs using the existing skeleton/token estimation helpers, keeping the planner’s retry behavior intact and only exposing metadata that helps explain slow Phase 2 calls.

**Tech Stack:** Go, standard `testing`, existing `internal/llm`, existing `internal/planner`, existing `internal/log`, existing mock/test seams.

---

### Task 1: Add failing LLM client debug-log tests

**Files:**
- Modify: `internal/llm/client.go`
- Modify: `internal/llm/client_test.go`

**Step 1: Write the failing tests**

Add focused coverage such as:

```go
func TestCompleteVerboseLogsRequestMetadataAndTiming(t *testing.T) {}
func TestCompleteVerboseLogsErrorTypeOnFailure(t *testing.T) {}
func TestCompleteWithoutVerboseDoesNotEmitDebugLogs(t *testing.T) {}
```

Assert the logs include model, max tokens, timeout seconds, base URL, user prompt char count, estimated user prompt tokens, and request timing/error metadata.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/llm -run 'TestCompleteVerboseLogsRequestMetadataAndTiming|TestCompleteVerboseLogsErrorTypeOnFailure|TestCompleteWithoutVerboseDoesNotEmitDebugLogs' -v
```

Expected: FAIL because `client.go` currently emits no debug logs.

**Step 3: Write minimal implementation**

Add a logger field or equivalent seam to `openAIClient`, initialize it from config verbose state, and log immediately before and after `CreateChatCompletion`. Log prompt size metadata only; do not log raw prompt content or API keys.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing planner debug-log tests

**Files:**
- Modify: `internal/planner/planner.go`
- Modify: `internal/planner/planner_test.go`

**Step 1: Write the failing tests**

Add focused coverage such as:

```go
func TestRunPlannerVerboseLogsPromptSizingAndAttemptMetadata(t *testing.T) {}
func TestRunPlannerVerboseLogsRetriesWithIncrementingAttemptNumbers(t *testing.T) {}
```

Assert that planner logs include skeleton token estimate, prompt length, planner attempt number, and planner model before each LLM call.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestRunPlannerVerboseLogsPromptSizingAndAttemptMetadata|TestRunPlannerVerboseLogsRetriesWithIncrementingAttemptNumbers' -v
```

Expected: FAIL because `planner.go` currently emits no debug logs.

**Step 3: Write minimal implementation**

Reuse the existing logging pattern in `internal/planner`, initialize a verbose-aware logger from config, and add debug logs before each planner LLM call without changing parse/retry semantics.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Verify the LLM and planner logging slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run focused package tests**

Run:

```bash
go test ./internal/llm ./internal/planner -v
```

Expected: PASS.

**Step 2: Run repo-wide sanity tests**

Run:

```bash
go test ./...
```

Expected: PASS.

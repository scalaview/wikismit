# Epic 7H Verbose Config and Logger Seams Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Plumb the existing `--verbose` CLI flag into runtime config and add the smallest logger seam needed to assert debug output in tests.

**Architecture:** Start at the CLI/config boundary so later packages can trust a shared `cfg.Verbose` value instead of reading global flag state. Then add a narrow `internal/log` seam that keeps stderr text logging in production while letting tests capture debug output deterministically.

**Tech Stack:** Go, standard `testing`, Cobra CLI tests, existing `internal/log`, existing `internal/config`.

---

### Task 1: Add failing tests for verbose config plumbing

**Files:**
- Modify: `cmd/wikismit/main_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/wikismit/main.go`

**Step 1: Write the failing tests**

Add focused coverage proving:

```go
func TestApplyCLIOverridesSetsVerboseOnConfig(t *testing.T) {}
func TestApplyCLIOverridesLeavesVerboseFalseByDefault(t *testing.T) {}
```

Keep the assertions narrow: the tests should prove only that the existing root flag propagates to config.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestApplyCLIOverridesSetsVerboseOnConfig|TestApplyCLIOverridesLeavesVerboseFalseByDefault' -v
```

Expected: FAIL because `config.Config` does not yet carry verbose state.

**Step 3: Write minimal implementation**

Add a `Verbose bool` field to `internal/config.Config` and update `applyCLIOverrides` to set it from the existing `verbose` flag.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing logger-capture tests before modifying package logging

**Files:**
- Modify: `internal/log/log.go`
- Create or modify: `internal/log/log_test.go`

**Step 1: Write the failing tests**

Add tests that prove:

```go
func TestNewVerboseLoggerEmitsDebugOutput(t *testing.T) {}
func TestNewNonVerboseLoggerSuppressesDebugOutput(t *testing.T) {}
```

The tests should capture logger output through a writer seam rather than real stderr.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/log -run 'TestNewVerboseLoggerEmitsDebugOutput|TestNewNonVerboseLoggerSuppressesDebugOutput' -v
```

Expected: FAIL because `internal/log` has no capture seam yet.

**Step 3: Write minimal implementation**

Add the smallest constructor/helper needed to build a logger against an injected writer while preserving `New(verbose)` as the production entrypoint.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Verify the verbose plumbing slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run focused package tests**

Run:

```bash
go test ./cmd/wikismit ./internal/log -v
```

Expected: PASS.

**Step 2: Run repo-wide sanity tests**

Run:

```bash
go test ./...
```

Expected: PASS.

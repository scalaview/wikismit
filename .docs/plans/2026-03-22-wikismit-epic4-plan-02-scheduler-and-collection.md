# Epic 4 Scheduler and Collection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Phase 4 goroutine scheduler and collector so module agents run under a concurrency cap and write partial-success results into `artifacts/module_docs/`.

**Architecture:** First lock the semaphore scheduling contract and peak-concurrency enforcement in isolation, then add channel collection and artifact writing behavior, and finally verify partial failure and cancellation semantics. Keep the agent runner itself injectable so scheduler tests do not depend on real prompt building.

**Tech Stack:** Go, standard `testing`, `context`, `sync`, `time`, existing `pkg/store`, filesystem helpers under `os` and `path/filepath`.

---

### Task 1: Add a failing concurrency-cap test before implementing the scheduler

**Files:**
- Create: `internal/agent/scheduler.go`
- Create: `internal/agent/scheduler_test.go`

**Step 1: Write the failing concurrency test**

Add:

```go
func TestRunSchedulerCapsConcurrentAgents(t *testing.T) {}
```

Use a fake runner that increments an `active` counter, sleeps briefly, then decrements it while tracking `peakActive`.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/agent -run TestRunSchedulerCapsConcurrentAgents -v
```

Expected: FAIL because the scheduler does not exist.

**Step 3: Write minimal implementation**

Implement a scheduler entrypoint in `internal/agent/scheduler.go` that:

- accepts a module slice and concurrency value
- uses `sem := make(chan struct{}, concurrency)` to limit goroutines
- waits for all workers before returning

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS with `peakActive <= concurrency`.

### Task 2: Add failing collection tests before writing module docs to disk

**Files:**
- Modify: `internal/agent/scheduler.go`
- Modify: `internal/agent/scheduler_test.go`

**Step 1: Write the failing collector tests**

Add:

```go
func TestCollectResultsWritesSuccessfulModuleDocs(t *testing.T) {}
func TestCollectResultsSkipsFailedModuleDocs(t *testing.T) {}
```

Cover these behaviors:

- successful `ModuleDoc` values write `{artifactsDir}/module_docs/{moduleID}.md`
- failed `ModuleDoc` values do not write files
- collector tracks failed module IDs for later summary handling

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent -run 'TestCollectResultsWritesSuccessfulModuleDocs|TestCollectResultsSkipsFailedModuleDocs' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement `collectResults` to drain `chan ModuleDoc`, write successful files, and record failures.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Lock partial-success and no-leak scheduler behavior

**Files:**
- Modify: `internal/agent/scheduler.go`
- Modify: `internal/agent/scheduler_test.go`

**Step 1: Write the failing integration-style scheduler tests**

Add:

```go
func TestRunSchedulerProcessesAllModulesWithPartialFailures(t *testing.T) {}
func TestRunSchedulerStopsCleanlyOnContextCancellation(t *testing.T) {}
```

Cover these behaviors:

- one module can fail without preventing other outputs from being written
- cancellation reaches in-flight workers and the scheduler exits without leaking goroutines

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent -run 'TestRunSchedulerProcessesAllModulesWithPartialFailures|TestRunSchedulerStopsCleanlyOnContextCancellation' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Finish the scheduler loop so it:

- launches workers with shared `context.Context`
- closes the results channel only after `wg.Wait()`
- returns partial-success summary information without losing successful outputs

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Verify the full scheduler slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run scheduler-focused tests**

Run:

```bash
go test ./internal/agent -run 'TestRunScheduler|TestCollectResults' -v
```

Expected: PASS.

**Step 2: Run broader package verification**

Run:

```bash
go test ./internal/agent ./internal/planner ./internal/preprocessor -v
```

Expected: PASS.

**Step 3: Validate artifact write shape**

Use temp directories in tests to confirm `module_docs/{moduleID}.md` is the only written Phase 4 output path.

Expected:

- successful modules have files under `module_docs/`
- failed modules have no placeholder output file

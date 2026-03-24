# Epic 7H Incremental Fallback Phase Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--verbose`-gated start/end timing logs for every phase inside the fallback full-generate path used by incremental update mode.

**Architecture:** Treat fallback diagnostics as a narrow instrumentation layer on top of the existing `runFullGenerate` pipeline in `internal/pipeline/incremental.go`. Lock the exact phase-log ordering in tests first, then add timing around each phase boundary without changing orchestration behavior.

**Tech Stack:** Go, standard `testing`, existing `internal/pipeline`, existing `internal/log`, existing incremental test seams.

---

### Task 1: Add failing fallback phase-log tests

**Files:**
- Modify: `internal/pipeline/incremental.go`
- Modify: `internal/pipeline/incremental_test.go`

**Step 1: Write the failing tests**

Add focused coverage such as:

```go
func TestRunFullGenerateVerboseLogsPhaseBoundariesInOrder(t *testing.T) {}
func TestRunFullGenerateWithoutVerboseSkipsDebugPhaseLogs(t *testing.T) {}
```

Assert start/end logging for:

- phase1
- planner
- preprocessor
- agent
- composer

and verify the log order is deterministic.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/pipeline -run 'TestRunFullGenerateVerboseLogsPhaseBoundariesInOrder|TestRunFullGenerateWithoutVerboseSkipsDebugPhaseLogs' -v
```

Expected: FAIL because fallback phase logging does not exist yet.

**Step 3: Write minimal implementation**

Add verbose debug logs around each phase in `runFullGenerate`, including elapsed duration on phase end logs. Keep all orchestration and return behavior unchanged.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Verify the incremental logging slice and full regression

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run focused pipeline tests**

Run:

```bash
go test ./internal/pipeline -v
```

Expected: PASS.

**Step 2: Run command-and-runtime regression tests**

Run:

```bash
go test ./cmd/wikismit ./internal/llm ./internal/planner ./internal/pipeline -v
```

Expected: PASS.

**Step 3: Run full repository regression**

Run:

```bash
go test ./...
```

Expected: PASS.

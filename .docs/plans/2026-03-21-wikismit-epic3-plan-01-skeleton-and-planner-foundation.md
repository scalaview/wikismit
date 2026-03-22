# Epic 3 Skeleton and Planner Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build deterministic, token-budget-aware skeleton serialization helpers that give Phase 2 and Phase 3 a stable prompt input surface.

**Architecture:** Start by creating the planner package and locking token estimation with focused tests. Then add per-module and full-repo skeleton builders in small TDD slices so later planner and preprocessor work can treat skeleton generation as a finished dependency rather than mixing prompt logic with serialization logic.

**Tech Stack:** Go, standard `testing`, existing `pkg/store`, existing `internal/log` logger.

---

### Task 1: Scaffold the planner package around token estimation

**Files:**
- Create: `internal/planner/skeleton.go`
- Create: `internal/planner/skeleton_test.go`

**Step 1: Write the failing token-estimation test**

Add:

```go
func TestEstimateTokensUsesSimpleCharacterApproximation(t *testing.T) {}
```

Assert that a known string length produces the expected `len(text) / 4` approximation.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/planner -run TestEstimateTokensUsesSimpleCharacterApproximation -v
```

Expected: FAIL because the planner package does not exist yet.

**Step 3: Write minimal implementation**

Implement:

```go
func estimateTokens(text string) int
```

Keep it deterministic and local; do not pull in a tokenizer dependency.

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Lock per-module skeleton formatting before truncation logic

**Files:**
- Modify: `internal/planner/skeleton.go`
- Modify: `internal/planner/skeleton_test.go`

**Step 1: Write failing formatting tests**

Add:

```go
func TestBuildSkeletonIncludesAnnotatedFunctionAndTypeLines(t *testing.T) {}
func TestBuildSkeletonSeparatesFilesWithHeaders(t *testing.T) {}
```

Cover these behaviors:

- each file starts with `// === {relative/path.go} ===`
- functions render as `{Signature}  // {path}:{LineStart}`
- types render as `type {Name} {Kind}  // {path}:{LineStart}`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestBuildSkeletonIncludesAnnotatedFunctionAndTypeLines|TestBuildSkeletonSeparatesFilesWithHeaders' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func BuildSkeleton(files []string, idx store.FileIndex, maxTokens int) string
```

At this stage it is acceptable to serialize all lines without truncation as long as ordering and annotations are deterministic.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add exported-first truncation rules for module skeletons

**Files:**
- Modify: `internal/planner/skeleton.go`
- Modify: `internal/planner/skeleton_test.go`

**Step 1: Write failing truncation tests**

Add:

```go
func TestBuildSkeletonDropsUnexportedSymbolsBeforeExportedOnBudgetOverflow(t *testing.T) {}
func TestBuildSkeletonStaysWithinTokenBudget(t *testing.T) {}
```

Assert that:

- exported lines remain when the budget is tight
- unexported lines are the first lines dropped
- returned skeleton text never exceeds the provided token budget approximation

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestBuildSkeletonDropsUnexportedSymbolsBeforeExportedOnBudgetOverflow|TestBuildSkeletonStaysWithinTokenBudget' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Add truncation behavior that:

- buckets exported vs unexported lines
- appends exported lines first
- stops before exceeding the budget
- logs a warning with dropped-symbol count when truncation happens

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add full-repo skeleton generation after module rules are stable

**Files:**
- Modify: `internal/planner/skeleton.go`
- Modify: `internal/planner/skeleton_test.go`

**Step 1: Write failing full-repo skeleton tests**

Add:

```go
func TestBuildFullSkeletonIncludesAllFilesWhenUnderBudget(t *testing.T) {}
func TestBuildFullSkeletonUsesSameExportedFirstTruncationRule(t *testing.T) {}
```

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestBuildFullSkeletonIncludesAllFilesWhenUnderBudget|TestBuildFullSkeletonUsesSameExportedFirstTruncationRule' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func BuildFullSkeleton(idx store.FileIndex, maxTokens int) string
```

Reuse the same deterministic ordering and truncation policy as `BuildSkeleton`.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the planner foundation slice before Phase 2 work

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run the planner package suite**

Run:

```bash
go test ./internal/planner -v
```

Expected: PASS.

**Step 2: Run repo-wide sanity tests**

Run:

```bash
go test ./...
```

Expected: note any pre-existing master failures separately, but no new failures caused by planner package changes.

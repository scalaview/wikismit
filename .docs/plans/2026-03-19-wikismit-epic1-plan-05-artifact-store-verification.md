# Epic 1 Artifact Store and Verification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the artifact store contract used by later phases and finish Epic 1 with full verification.

**Architecture:** Keep all artifact schemas and read/write helpers in `pkg/store`, with atomic writes and typed read APIs. Finish by verifying config, CLI, llm, and store together as the Epic 1 baseline.

**Tech Stack:** Go, `encoding/json`, `os`, `filepath`, standard `testing`.

---

### Task 1: Write failing store tests

**Files:**
- Create: `pkg/store/store_test.go`

**Step 1: Add round-trip and atomicity tests**

```go
func TestWriteAndReadFileIndexRoundTrip(t *testing.T) {}
func TestWriteAndReadDepGraphRoundTrip(t *testing.T) {}
func TestWriteAndReadNavPlanRoundTrip(t *testing.T) {}
func TestWriteAndReadSharedContextRoundTrip(t *testing.T) {}
func TestReadReturnsErrArtifactNotFound(t *testing.T) {}
func TestConcurrentWritesLeaveValidJSON(t *testing.T) {}
```

**Step 2: Run tests**

```bash
go test ./pkg/store -v
```

Expected: FAIL.

### Task 2: Implement artifact schemas

**Files:**
- Create: `pkg/store/artifacts.go`

**Step 1: Copy the Epic 1 schemas exactly**

Implement:

- `FileIndex`
- `FileEntry`
- `FunctionDecl`
- `TypeDecl`
- `Import`
- `DepGraph`
- `Module`
- `NavPlan`
- `SharedSummary`
- `KeyFunction`
- `SharedContext`

### Task 3: Implement atomic read/write helpers

**Files:**
- Create: `pkg/store/store.go`
- Create: `pkg/store/index.go`

**Step 1: Implement shared JSON helpers**

```go
var ErrArtifactNotFound = errors.New("artifact not found")

func writeJSON(path string, v any) error
func readJSON(path string, v any) error
```

Rules:

- mkdir parent dirs
- write to `.tmp`
- rename atomically
- indent JSON
- wrap missing files with `ErrArtifactNotFound`

**Step 2: Implement typed APIs**

Add:

- `WriteFileIndex` / `ReadFileIndex`
- `WriteDepGraph` / `ReadDepGraph`
- `WriteNavPlan` / `ReadNavPlan`
- `WriteSharedContext` / `ReadSharedContext`

**Step 3: Run store tests**

```bash
go test ./pkg/store -v
```

Expected: PASS.

**Step 4: Commit**

```bash
git add pkg/store
git commit -m "feat: add artifact store layer"
```

### Task 4: Run full Epic 1 verification

**Files:**
- Verify only

**Step 1: Focused suites**

```bash
go test ./internal/config -v
go test ./internal/llm -v
go test ./pkg/store -v
```

**Step 2: Full suite**

```bash
go test ./...
```

**Step 3: Build and smoke test**

```bash
go build -o ./wikismit ./cmd/wikismit
./wikismit --help
./wikismit generate --config ./config.yaml
./wikismit update --config ./config.yaml
./wikismit plan --config ./config.yaml
./wikismit validate --config ./config.yaml
./wikismit build --config ./config.yaml
```

Expected:

- build succeeds
- help is correct
- `generate` prints resolved config
- other commands stay stubbed but do not panic

**Step 4: Final fixup commit if needed**

```bash
git add .
git commit -m "test: finish Epic 1 verification fixes"
```

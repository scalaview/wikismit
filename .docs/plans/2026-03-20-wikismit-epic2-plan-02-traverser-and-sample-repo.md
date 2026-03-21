# Epic 2 Traverser and Sample Repo Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the synthetic sample repository and the repository-level analyzer that traverses files, applies exclude rules, dispatches parsers, and returns a complete `FileIndex`.

**Architecture:** Create the fixture repo first so traversal behavior has a stable target. Then implement the `Analyzer` in small TDD slices: construction, happy-path traversal, exclusion behavior, unknown-language skipping, and parse-error continuation.

**Tech Stack:** Go, standard `testing`, `filepath.WalkDir`, `github.com/bmatcuk/doublestar/v4`, existing `internal/log` logger.

---

### Task 1: Create the synthetic sample repository fixture

**Files:**
- Create: `testdata/sample_repo/go.mod`
- Create: `testdata/sample_repo/README.md`
- Create: `testdata/sample_repo/cmd/main.go`
- Create: `testdata/sample_repo/internal/api/handler.go`
- Create: `testdata/sample_repo/internal/auth/jwt.go`
- Create: `testdata/sample_repo/internal/auth/middleware.go`
- Create: `testdata/sample_repo/internal/db/client.go`
- Create: `testdata/sample_repo/pkg/logger/logger.go`
- Create: `testdata/sample_repo/pkg/errors/errors.go`

**Step 1: Create the repo tree exactly as the Epic 2 task describes**

Preserve the module path and import relationships from `S2.2.1` so dep graph work can reuse the same fixture later.

**Step 2: Sanity-check fixture count**

Run:

```bash
go test ./... -run TestDoesNotExist
```

Expected: no fixture-related parse or package discovery issues from the main repo itself.

### Task 2: Add failing tests for `Analyzer` construction and parser dispatch

**Files:**
- Create: `internal/analyzer/analyzer.go`
- Create: `internal/analyzer/analyzer_test.go`

**Step 1: Write failing constructor tests**

Add:

```go
func TestNewAnalyzerStoresExcludePatternsAndRegistry(t *testing.T) {}
func TestAnalyzeIndexesAllGoFilesInSampleRepo(t *testing.T) {}
```

Assert that:

- constructor retains analysis config needed at runtime
- analyzing `testdata/sample_repo` returns all seven `.go` files keyed by relative path

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestNewAnalyzer|TestAnalyzeIndexesAllGoFilesInSampleRepo' -v
```

Expected: FAIL because `Analyzer` does not exist.

**Step 3: Write minimal implementation**

Implement:

- `type Analyzer struct { ... }`
- `func NewAnalyzer(cfg config.AnalysisConfig) *Analyzer`
- minimal `Analyze(repoPath string) (store.FileIndex, error)` happy path

Prefer pointer fields inside the struct where that avoids unnecessary copying.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add failing exclusion and unknown-extension tests

**Files:**
- Modify: `internal/analyzer/analyzer_test.go`
- Modify: `internal/analyzer/analyzer.go`

**Step 1: Write failing tests**

Add:

```go
func TestAnalyzeSkipsFilesMatchingExcludePatterns(t *testing.T) {}
func TestAnalyzeSkipsUnknownExtensionsSilently(t *testing.T) {}
```

Cover these cases:

- `*_test.go` excluded
- `vendor/**` excluded
- `.py` file ignored while no Python parser exists

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestAnalyzeSkipsFilesMatchingExcludePatterns|TestAnalyzeSkipsUnknownExtensionsSilently' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Use `doublestar` against slash-normalized relative paths and skip any file whose extension is absent from the registry.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add failing parse-error continuation test

**Files:**
- Modify: `internal/analyzer/analyzer_test.go`
- Modify: `internal/analyzer/analyzer.go`

**Step 1: Write the failing continuation test**

Add:

```go
func TestAnalyzeWarnsAndContinuesOnParseError(t *testing.T) {}
```

Test flow:

- create temp repo with one valid `.go` file and one malformed `.go` file
- analyze the temp repo
- assert the valid file is still present
- assert skipped count or logged warning evidence exists

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/analyzer -run TestAnalyzeWarnsAndContinuesOnParseError -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

On parser error:

- record the skip
- log a warning with file path and error
- continue traversal

**Step 4: Run tests to confirm GREEN**

Run:

```bash
go test ./internal/analyzer -v
```

Expected: PASS.

### Task 5: Verify the traverser slice before moving to dep graph work

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run the full analyzer-focused suite**

Run:

```bash
go test ./internal/analyzer/... -v
```

Expected: PASS.

**Step 2: Run repo-wide sanity tests**

Run:

```bash
go test ./... 
```

Expected: PASS with no regressions outside analyzer packages.

# Epic 2 Dependency Graph and Phase 1 Wiring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve internal imports into concrete file dependencies, build `dep_graph.json`, wire Phase 1 into `wikismit generate`, and verify deterministic artifact output.

**Architecture:** Reuse the sample repo and traversal output from earlier plans. First lock internal/external import resolution and graph shape with focused tests, then wire `RunPhase1` and CLI overrides, and finish with idempotency plus end-to-end artifact verification.

**Tech Stack:** Go, `golang.org/x/mod/modfile`, standard `testing`, existing `pkg/store`, existing Cobra CLI.

---

### Task 1: Add failing tests for module-path loading and import resolution

**Files:**
- Create: `internal/analyzer/dep_graph.go`
- Create: `internal/analyzer/dep_graph_test.go`

**Step 1: Write failing tests**

Add:

```go
func TestReadModulePathReturnsGoModModule(t *testing.T) {}
func TestResolveInternalImportsMarksImportsAndResolvedPaths(t *testing.T) {}
```

Assert that:

- `readModulePath(testdata/sample_repo)` returns `github.com/wikismit/sample`
- imports under that prefix become `Internal: true`
- `ResolvedPath` is populated in memory for internal imports
- third-party imports stay external

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestReadModulePathReturnsGoModModule|TestResolveInternalImportsMarksImportsAndResolvedPaths' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

- `readModulePath(repoPath string) (string, error)`
- helper(s) to resolve internal import targets against repo files
- one-read-per-run module path caching on `Analyzer`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing dep graph tests before building the graph

**Files:**
- Modify: `internal/analyzer/dep_graph_test.go`
- Modify: `internal/analyzer/dep_graph.go`

**Step 1: Write failing graph tests**

Add:

```go
func TestBuildDepGraphIncludesEdgesForInternalImports(t *testing.T) {}
func TestBuildDepGraphIncludesFilesWithNoInternalImports(t *testing.T) {}
func TestBuildDepGraphOmitsThirdPartyEdges(t *testing.T) {}
```

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestBuildDepGraph' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func BuildDepGraph(idx store.FileIndex) store.DepGraph
```

Requirements:

- every indexed file becomes a key
- internal imports contribute edges via `ResolvedPath`
- output order is deterministic

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add failing tests for Phase 1 orchestration

**Files:**
- Create: `internal/analyzer/phase1.go`
- Modify: `internal/analyzer/analyzer_test.go`
- Modify: `internal/analyzer/dep_graph_test.go`

**Step 1: Write the failing orchestration tests**

Add:

```go
func TestRunPhase1WritesFileIndexAndDepGraph(t *testing.T) {}
func TestRunPhase1IsIdempotentForUnchangedRepo(t *testing.T) {}
```

Test flow:

- build a temp config pointed at `testdata/sample_repo`
- write artifacts to a temp directory
- run `RunPhase1`
- assert `file_index.json` and `dep_graph.json` exist and are non-empty
- run again and compare bytes for deterministic output

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestRunPhase1' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

- `RunPhase1(cfg *config.Config) error`
- info logging before and after traversal
- warning logging for skipped parse failures
- store writes only after successful analysis + graph construction

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add failing CLI wiring tests before changing `generate`

**Files:**
- Modify: `cmd/wikismit/main.go`
- Modify: `cmd/wikismit/generate.go`
- Modify: `cmd/wikismit/generate_test.go`

**Step 1: Write the failing CLI tests**

Add tests that prove:

- root command supports `--repo`, `--output`, and `--artifacts`
- those flags override config values before `RunPhase1`
- `generate` no longer prints the config YAML stub output

Suggested test names:

```go
func TestGenerateCommandRunsPhase1WithRepoOverride(t *testing.T) {}
func TestRootCommandExposesPhase1OverrideFlags(t *testing.T) {}
```

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestGenerateCommandRunsPhase1WithRepoOverride|TestRootCommandExposesPhase1OverrideFlags' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement config overrides in the CLI layer and replace the YAML-printing stub in `generate.go` with a Phase 1 call.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Run the end-to-end verification suite

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run focused package tests**

Run:

```bash
go test ./internal/analyzer/... ./cmd/wikismit -v
```

Expected: PASS.

**Step 2: Run repo-wide tests**

Run:

```bash
go test ./...
```

Expected: PASS.

**Step 3: Build the CLI**

Run:

```bash
go build -o ./wikismit ./cmd/wikismit
```

Expected: exit 0.

**Step 4: Run the Phase 1 smoke check**

Run:

```bash
cp config.yaml.example config.yaml
./wikismit generate --config ./config.yaml --repo ./testdata/sample_repo --artifacts ./artifacts
```

Expected:

- `artifacts/file_index.json` exists and is non-empty
- `artifacts/dep_graph.json` exists and is non-empty
- logs include Phase 1 start/completion messages

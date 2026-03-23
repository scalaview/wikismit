# Epic 6 Git Diff and Affected Module Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build deterministic changed-file detection and affected-module computation so incremental mode knows exactly which modules must be reprocessed.

**Architecture:** First lock the changed-file contracts in a new `pkg/gitdiff` package using tight unit tests around explicit file lists and git-ref diffs. Then add module ownership and upstream propagation in `internal/analyzer`, grounded on the existing `store.NavPlan` and file-level `store.DepGraph`, so later incremental orchestration can consume a tested affected-module set.

**Tech Stack:** Go, standard `testing`, existing `pkg/store`, existing `internal/analyzer`, `github.com/go-git/go-git/v5`, existing sample repo fixtures.

---

### Task 1: Add the git dependency and lock explicit changed-file parsing first

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `pkg/gitdiff/diff.go`
- Create: `pkg/gitdiff/diff_test.go`

**Step 1: Write the failing parse-only tests**

Add:

```go
func TestParseChangedFilesReturnsModifiedEntries(t *testing.T) {}
func TestParseChangedFilesTrimsWhitespaceAndSkipsEmptyValues(t *testing.T) {}
func TestParseChangedFilesEmptyInputReturnsEmptySlice(t *testing.T) {}
```

Cover these behaviors:

- comma-separated input becomes `[]FileChange`
- each parsed entry defaults to `ChangeModified`
- blank segments are ignored deterministically

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./pkg/gitdiff -run 'TestParseChangedFilesReturnsModifiedEntries|TestParseChangedFilesTrimsWhitespaceAndSkipsEmptyValues|TestParseChangedFilesEmptyInputReturnsEmptySlice' -v
```

Expected: FAIL because `pkg/gitdiff` does not exist.

**Step 3: Write minimal implementation**

Implement in `pkg/gitdiff/diff.go`:

```go
type ChangeType string

const (
	ChangeModified ChangeType = "modified"
	ChangeAdded    ChangeType = "added"
	ChangeDeleted  ChangeType = "deleted"
	ChangeRenamed  ChangeType = "renamed"
)

type FileChange struct {
	Path    string
	OldPath string
	Type    ChangeType
}

func ParseChangedFiles(input string) []FileChange
```

Keep the implementation deterministic and free of git access.

**Step 4: Add the dependency**

Run:

```bash
go get github.com/go-git/go-git/v5
go mod tidy
```

Expected: `go.mod` and `go.sum` update cleanly.

**Step 5: Run tests to confirm GREEN**

Run the same `go test` command. Expected: PASS.

### Task 2: Add failing git-diff tests before implementing repository diffing

**Files:**
- Modify: `pkg/gitdiff/diff.go`
- Modify: `pkg/gitdiff/diff_test.go`

**Step 1: Write the failing git-diff tests**

Add:

```go
func TestGetChangedFilesReturnsModifiedFileBetweenTwoCommits(t *testing.T) {}
func TestGetChangedFilesReportsAddedDeletedAndRenamedFiles(t *testing.T) {}
func TestGetChangedFilesDefaultsToHeadRangeWhenRefsAreEmpty(t *testing.T) {}
func TestGetChangedFilesReturnsAllFilesForInitialCommit(t *testing.T) {}
```

Build the test repo in a temp directory with real commits so the tests prove the contract against `go-git` behavior.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./pkg/gitdiff -run 'TestGetChangedFilesReturnsModifiedFileBetweenTwoCommits|TestGetChangedFilesReportsAddedDeletedAndRenamedFiles|TestGetChangedFilesDefaultsToHeadRangeWhenRefsAreEmpty|TestGetChangedFilesReturnsAllFilesForInitialCommit' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func GetChangedFiles(repoPath, baseRef, headRef string) ([]FileChange, error)
```

Requirements:

- open the repo with `git.PlainOpen`
- default `baseRef` to `HEAD~1` and `headRef` to `HEAD`
- resolve refs with `ResolveRevision`
- diff the two commit trees
- map from/to patch states to `ChangeModified`, `ChangeAdded`, `ChangeDeleted`, and `ChangeRenamed`
- return repo-relative paths only

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Map changed files to owning modules before transitive propagation

**Files:**
- Create: `internal/analyzer/affected.go`
- Create: `internal/analyzer/affected_test.go`

**Step 1: Write the failing ownership tests**

Add:

```go
func TestOwningModulesReturnsDirectOwnersForChangedFiles(t *testing.T) {}
func TestOwningModulesIgnoresUnknownFiles(t *testing.T) {}
```

Use a small in-memory `store.NavPlan` and `gitdiff.FileChange` set.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestOwningModulesReturnsDirectOwnersForChangedFiles|TestOwningModulesIgnoresUnknownFiles' -v
```

Expected: FAIL because the affected-module helpers do not exist.

**Step 3: Write minimal implementation**

Implement helper(s) in `internal/analyzer/affected.go`:

```go
func owningModules(changedFiles []gitdiff.FileChange, plan *store.NavPlan) []string
```

Return a deduplicated, stable module-ID list.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Build reverse-graph propagation before full affected-set computation

**Files:**
- Modify: `internal/analyzer/affected.go`
- Modify: `internal/analyzer/affected_test.go`

**Step 1: Write the failing graph tests**

Add:

```go
func TestBuildReverseGraphReversesFileEdges(t *testing.T) {}
func TestComputeAffectedReturnsLeafOwnerOnlyForIsolatedChange(t *testing.T) {}
func TestComputeAffectedPropagatesSharedModuleChangesToDependents(t *testing.T) {}
func TestComputeAffectedHandlesErrorsModuleDependenciesFromSampleRepo(t *testing.T) {}
```

Use `testdata/sample_repo/`-shaped plans and dep graphs so the expectations match the existing synthetic repo.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/analyzer -run 'TestBuildReverseGraphReversesFileEdges|TestComputeAffectedReturnsLeafOwnerOnlyForIsolatedChange|TestComputeAffectedPropagatesSharedModuleChangesToDependents|TestComputeAffectedHandlesErrorsModuleDependenciesFromSampleRepo' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func buildReverseGraph(graph store.DepGraph) store.DepGraph
func ComputeAffected(changedFiles []gitdiff.FileChange, plan *store.NavPlan, graph store.DepGraph) []store.Module
```

Requirements:

- start from direct owning modules
- traverse upstream dependents from the file-level reverse graph
- map affected files back to module IDs
- return only modules present in `plan.Modules`
- keep output stable and deduplicated for tests

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the Epic 6 foundation slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run gitdiff tests**

Run:

```bash
go test ./pkg/gitdiff -v
```

Expected: PASS.

**Step 2: Run analyzer-focused affected tests**

Run:

```bash
go test ./internal/analyzer -run 'TestOwningModules|TestBuildReverseGraph|TestComputeAffected' -v
```

Expected: PASS.

**Step 3: Run broader regression check**

Run:

```bash
go test ./internal/analyzer ./pkg/gitdiff ./cmd/wikismit -v
```

Expected: PASS with no new failures.

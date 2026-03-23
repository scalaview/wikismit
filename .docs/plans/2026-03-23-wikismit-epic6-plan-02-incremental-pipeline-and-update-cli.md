# Epic 6 Incremental Pipeline and Update CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a real incremental pipeline and replace the `wikismit update` stub so changed modules can be reprocessed without rerunning the full pipeline unnecessarily.

**Architecture:** First build a dedicated `internal/pipeline` incremental entrypoint that reuses existing Phase 1/3/4/5 package boundaries instead of duplicating pipeline logic in the CLI layer. Then add minimal partial-run seams to the analyzer/preprocessor/agent surfaces, wire the `update` command flags to that runtime, and finish with fallback and call-count verification through the existing CLI test harness.

**Tech Stack:** Go, standard `testing`, `context`, existing Cobra CLI tests, existing `pkg/store`, existing `internal/analyzer`, `internal/preprocessor`, `internal/agent`, `internal/composer`, existing `internal/llm.MockClient`.

---

### Task 1: Add failing incremental fallback tests before creating the pipeline package

**Files:**
- Create: `internal/pipeline/incremental.go`
- Create: `internal/pipeline/incremental_test.go`

**Step 1: Write the failing fallback tests**

Add:

```go
func TestRunIncrementalFallsBackToFullGenerateWhenArtifactsMissing(t *testing.T) {}
func TestRunIncrementalUsesChangedFilesOverrideWithoutOpeningGit(t *testing.T) {}
```

Cover these behaviors:

- missing `file_index.json` returns the full-generate path using `store.ErrArtifactNotFound`
- explicit changed files bypass git diff lookup entirely

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/pipeline -run 'TestRunIncrementalFallsBackToFullGenerateWhenArtifactsMissing|TestRunIncrementalUsesChangedFilesOverrideWithoutOpeningGit' -v
```

Expected: FAIL because `internal/pipeline` does not exist.

**Step 3: Write minimal implementation**

Create `internal/pipeline/incremental.go` with a small, injectable scaffold around:

```go
func RunIncremental(ctx context.Context, cfg *config.Config, client llm.Client, opts IncrementalOptions) error
```

and minimal indirections for:

- full-generate fallback
- changed-file provider
- artifact reads

Keep the first pass narrow: prove fallback and override selection before partial reruns.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Reanalyze changed files and persist updated artifacts in place

**Files:**
- Modify: `internal/pipeline/incremental.go`
- Modify: `internal/pipeline/incremental_test.go`
- Modify: `internal/analyzer/phase1.go`
- Modify: `internal/analyzer/analyzer.go`

**Step 1: Write the failing reanalysis tests**

Add:

```go
func TestReanalyzeChangedUpdatesModifiedAndAddedFiles(t *testing.T) {}
func TestReanalyzeChangedRemovesDeletedFiles(t *testing.T) {}
func TestReanalyzeChangedHandlesRenamesByDroppingOldPathAndParsingNewPath(t *testing.T) {}
```

The tests should prove in-place `store.FileIndex` mutation plus rewritten `file_index.json` output.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/pipeline -run 'TestReanalyzeChangedUpdatesModifiedAndAddedFiles|TestReanalyzeChangedRemovesDeletedFiles|TestReanalyzeChangedHandlesRenamesByDroppingOldPathAndParsingNewPath' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement helper(s) such as:

```go
func reanalyzeChanged(changes []gitdiff.FileChange, idx store.FileIndex, cfg *config.Config) (store.FileIndex, error)
```

Requirements:

- reuse existing analyzer parsing logic where possible
- update only touched entries for add/modify/rename
- remove deleted paths
- write the updated `file_index.json` back through `pkg/store`
- rebuild `dep_graph.json` from the updated index after reanalysis

If a reusable single-file analyzer seam is missing, add the smallest one to `internal/analyzer` instead of copying parser logic into the pipeline package.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Add failing partial Phase 3 and Phase 4 rerun tests before introducing subset execution

**Files:**
- Modify: `internal/preprocessor/preprocessor.go`
- Modify: `internal/preprocessor/preprocessor_test.go`
- Modify: `internal/agent/scheduler.go`
- Modify: `internal/agent/scheduler_test.go`
- Modify: `internal/pipeline/incremental.go`
- Modify: `internal/pipeline/incremental_test.go`

**Step 1: Write the failing subset-run tests**

Add:

```go
func TestRunPreprocessorForRerunsOnlyAffectedSharedModules(t *testing.T) {}
func TestRunForProcessesOnlyAffectedAgentModules(t *testing.T) {}
func TestRunIncrementalRerunsSharedDependenciesBeforeAffectedAgentModules(t *testing.T) {}
func TestRunIncrementalRunsComposerInFullAfterPartialReruns(t *testing.T) {}
```

Use the mock client and existing plan/sample-repo patterns to prove ordering and selective LLM call counts.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/preprocessor ./internal/agent ./internal/pipeline -run 'TestRunPreprocessorForRerunsOnlyAffectedSharedModules|TestRunForProcessesOnlyAffectedAgentModules|TestRunIncrementalRerunsSharedDependenciesBeforeAffectedAgentModules|TestRunIncrementalRunsComposerInFullAfterPartialReruns' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement the smallest subset seams needed:

```go
func RunPreprocessorFor(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *config.Config, client llm.Client) (store.SharedContext, error)
func RunFor(ctx context.Context, modules []store.Module, input AgentInput, client llm.Client, artifactsDir string, concurrency int) error
```

Requirements:

- rerun only shared modules in the affected set
- preserve existing summaries for unaffected shared modules
- rerun Phase 4 only for affected non-shared modules
- leave unaffected module docs untouched
- keep Phase 5 full by reusing `composer.RunComposer`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Replace the `update` stub with a real command surface

**Files:**
- Modify: `cmd/wikismit/update.go`
- Modify: `cmd/wikismit/main.go`
- Modify: `cmd/wikismit/main_test.go`
- Modify: `cmd/wikismit/helpers.go`
- Modify: `internal/pipeline/incremental.go`

**Step 1: Write the failing CLI tests**

Add:

```go
func TestUpdateCommandExposesIncrementalFlags(t *testing.T) {}
func TestUpdateCommandFallsBackToGenerateWhenArtifactsMissing(t *testing.T) {}
func TestUpdateCommandUsesChangedFilesOverrideForSingleModuleRerun(t *testing.T) {}
func TestUpdateCommandRerunsDependentsWhenSharedModuleChanges(t *testing.T) {}
```

Use the existing `executeCLI`, `writeCLIConfig`, and mock-client seams from `cmd/wikismit/main_test.go`.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestUpdateCommandExposesIncrementalFlags|TestUpdateCommandFallsBackToGenerateWhenArtifactsMissing|TestUpdateCommandUsesChangedFilesOverrideForSingleModuleRerun|TestUpdateCommandRerunsDependentsWhenSharedModuleChanges' -v
```

Expected: FAIL because `update` still returns `not implemented`.

**Step 3: Write minimal implementation**

Replace the stub with a real command that:

- adds `--base-ref`, `--head-ref`, and `--changed-files`
- builds an `IncrementalOptions` value
- creates or injects an LLM client using the existing factory pattern
- calls `pipeline.RunIncremental`
- prints stable success/warning output through `writeCommandOutput`

Keep the CLI layer thin; orchestration belongs in `internal/pipeline`.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the full incremental slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run pipeline tests**

Run:

```bash
go test ./internal/pipeline -v
```

Expected: PASS.

**Step 2: Run incremental-adjacent package tests**

Run:

```bash
go test ./internal/analyzer ./internal/preprocessor ./internal/agent ./cmd/wikismit -v
```

Expected: PASS.

**Step 3: Run full regression check**

Run:

```bash
go test ./...
```

Expected: PASS.

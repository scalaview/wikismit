# Epic 3 Shared Preprocessor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build shared-module dependency ordering and serial shared-context generation so Phase 3 can write grounded `shared_context.json` artifacts for later agent injection.

**Architecture:** First derive the shared-module subgraph and topological ordering in isolation so Phase 3 has a trustworthy execution order. Then add prompt construction, summary grounding, and the serial LLM loop in small TDD slices, finishing with artifact writing and empty-shared-module behavior.

**Tech Stack:** Go, standard `testing`, `context`, existing `pkg/store`, existing `internal/llm`, existing `internal/planner` skeleton helpers.

---

### Task 1: Build shared-module subgraph extraction before sorting logic

**Files:**
- Create: `internal/preprocessor/preprocessor.go`
- Create: `internal/preprocessor/topo_test.go`

**Step 1: Write the failing subgraph test**

Add:

```go
func TestSharedSubgraphIncludesOnlySharedModuleDependencies(t *testing.T) {}
```

Assert that only shared-to-shared edges appear in the returned module-level graph.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/preprocessor -run TestSharedSubgraphIncludesOnlySharedModuleDependencies -v
```

Expected: FAIL because the preprocessor package does not exist yet.

**Step 3: Write minimal implementation**

Implement:

```go
func sharedSubgraph(plan *store.NavPlan, graph store.DepGraph) map[string][]string
```

Map file-level dependencies onto module IDs using only modules where `Shared` is true.

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing topo-sort tests before wiring Phase 3 orchestration

**Files:**
- Modify: `internal/preprocessor/preprocessor.go`
- Modify: `internal/preprocessor/topo_test.go`

**Step 1: Write failing topo-sort tests**

Add:

```go
func TestTopoSortOrdersDependenciesBeforeDependents(t *testing.T) {}
func TestTopoSortReturnsErrorOnCycle(t *testing.T) {}
func TestTopoSortReturnsEmptyOrderForEmptyGraph(t *testing.T) {}
```

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/preprocessor -run 'TestTopoSortOrdersDependenciesBeforeDependents|TestTopoSortReturnsErrorOnCycle|TestTopoSortReturnsEmptyOrderForEmptyGraph' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func topoSort(graph map[string][]string) ([]string, error)
```

Use deterministic Kahn processing order so tests do not depend on map iteration.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Build the shared-summary prompt before running any LLM loop

**Files:**
- Create: `internal/preprocessor/prompt.go`
- Create: `internal/preprocessor/preprocessor_test.go`

**Step 1: Write the failing prompt tests**

Add:

```go
func TestBuildSharedPromptIncludesSkeletonAndJSONContract(t *testing.T) {}
func TestBuildSharedPromptInjectsAlreadySummarisedDependencies(t *testing.T) {}
```

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/preprocessor -run 'TestBuildSharedPromptIncludesSkeletonAndJSONContract|TestBuildSharedPromptInjectsAlreadySummarisedDependencies' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func buildSharedPrompt(moduleID string, skeleton string, alreadySummarised store.SharedContext) string
```

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Ground shared summary refs against Phase 1 artifacts

**Files:**
- Create: `internal/preprocessor/shared_context.go`
- Modify: `internal/preprocessor/preprocessor_test.go`

**Step 1: Write failing grounding tests**

Add:

```go
func TestGroundSharedSummaryRefsUsesFileIndexLineNumbers(t *testing.T) {}
func TestGroundSharedSummaryRefsKeepsUnknownRefAndWarns(t *testing.T) {}
```

Cover these behaviors:

- `key_functions[].ref` is rewritten from `file_index.json` when a function match exists
- hallucinated refs are preserved only as fallback and emit warning evidence
- `SourceRefs` population is explicit and consistent with the grounded result

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/preprocessor -run 'TestGroundSharedSummaryRefsUsesFileIndexLineNumbers|TestGroundSharedSummaryRefsKeepsUnknownRefAndWarns' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement the smallest helper(s) needed to:

- match key functions against files in the shared module
- rewrite refs as `{path}#L{LineStart}` using `file_index.json`
- produce consistent `SourceRefs` values for the final stored summary

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Add failing orchestration tests for serial shared-module processing

**Files:**
- Modify: `internal/preprocessor/preprocessor.go`
- Modify: `internal/preprocessor/preprocessor_test.go`

**Step 1: Write the failing orchestration tests**

Add:

```go
func TestRunPreprocessorWritesSharedContextInTopologicalOrder(t *testing.T) {}
func TestRunPreprocessorSkipsLLMCallsWhenNoSharedModulesExist(t *testing.T) {}
func TestRunPreprocessorReturnsErrorOnInvalidSharedSummaryJSON(t *testing.T) {}
```

Use the mock LLM client to prove call ordering and zero-call behavior.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/preprocessor -run 'TestRunPreprocessorWritesSharedContextInTopologicalOrder|TestRunPreprocessorSkipsLLMCallsWhenNoSharedModulesExist|TestRunPreprocessorReturnsErrorOnInvalidSharedSummaryJSON' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func RunPreprocessor(ctx context.Context, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *config.Config, llm llm.Client) (store.SharedContext, error)
```

Requirements:

- derive shared-module order from `sharedSubgraph` + `topoSort`
- return an empty context and zero calls when no shared modules exist
- build each module skeleton from planner helpers
- inject already-produced shared summaries into later prompts
- write `shared_context.json` only after the full serial run succeeds

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 6: Verify the full Phase 3 slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run preprocessor-focused tests**

Run:

```bash
go test ./internal/preprocessor -v
```

Expected: PASS.

**Step 2: Run Epic 3 package verification**

Run:

```bash
go test ./internal/planner ./internal/preprocessor ./cmd/wikismit -v
```

Expected: PASS except for any separately documented pre-existing master failures outside the touched Epic 3 scope.

**Step 3: Run a shared-context smoke check**

Run:

```bash
ART_DIR=$(mktemp -d)
OPENAI_API_KEY=phase3-smoke ./wikismit plan --config ./config.yaml.example --repo ./testdata/sample_repo --artifacts "$ART_DIR"
```

Then run the Phase 3 library entrypoint through a focused test or small harness so that:

- `shared_context.json` exists in `$ART_DIR`
- each shared module has `summary`, `key_types`, and `key_functions`
- grounded refs use `path#Lline` format from `file_index.json`

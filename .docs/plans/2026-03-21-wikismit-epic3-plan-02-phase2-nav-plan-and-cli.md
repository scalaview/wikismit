# Epic 3 Phase 2 Nav Plan and CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Turn Phase 1 artifacts into a validated `nav_plan.json`, then wire `wikismit plan` to run Phase 1 + Phase 2 and stop cleanly.

**Architecture:** Reuse the skeleton helpers from Plan 01 as the only planner prompt input path. First lock prompt construction and planner retry behavior against the mock LLM client, then validate the parsed plan against `FileIndex`, write the artifact, and only after that replace the `plan` command stub with the new orchestration flow.

**Tech Stack:** Go, standard `testing`, `context`, `encoding/json`, existing `internal/llm`, existing `pkg/store`, existing Cobra CLI.

---

### Task 1: Build the Phase 2 planner prompt before any orchestration code

**Files:**
- Create: `internal/planner/prompt.go`
- Create: `internal/planner/planner_test.go`

**Step 1: Write the failing prompt test**

Add:

```go
func TestBuildPlannerPromptIncludesRulesThresholdAndSkeleton(t *testing.T) {}
```

Assert that the prompt includes:

- the architect framing
- the shared threshold value
- the JSON-only response rule
- the provided skeleton body

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/planner -run TestBuildPlannerPromptIncludesRulesThresholdAndSkeleton -v
```

Expected: FAIL because `buildPlannerPrompt` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func buildPlannerPrompt(skeleton string, threshold int) string
```

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing planner retry tests before implementing the LLM loop

**Files:**
- Create: `internal/planner/planner.go`
- Modify: `internal/planner/planner_test.go`

**Step 1: Write failing planner-run tests**

Add:

```go
func TestRunPlannerSucceedsWithValidJSONResponse(t *testing.T) {}
func TestRunPlannerRetriesAfterJSONParseFailure(t *testing.T) {}
func TestRunPlannerFailsAfterThreeInvalidResponses(t *testing.T) {}
```

Use the existing mock LLM client to simulate valid JSON, invalid JSON, and retryable malformed responses.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestRunPlannerSucceedsWithValidJSONResponse|TestRunPlannerRetriesAfterJSONParseFailure|TestRunPlannerFailsAfterThreeInvalidResponses' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Implement:

```go
func RunPlanner(ctx context.Context, idx store.FileIndex, graph store.DepGraph, cfg *config.Config, llm llm.Client) (*store.NavPlan, error)
```

Requirements:

- build the full-repo skeleton
- call `llm.Complete`
- parse JSON into `store.NavPlan`
- retry up to three attempts with prior error context appended to the prompt

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Validate nav-plan correctness after JSON parsing

**Files:**
- Modify: `internal/planner/planner.go`
- Modify: `internal/planner/planner_test.go`

**Step 1: Write failing validation tests**

Add:

```go
func TestRunPlannerRejectsMissingFileAssignments(t *testing.T) {}
func TestRunPlannerRejectsDuplicateFileAssignments(t *testing.T) {}
func TestRunPlannerRejectsInvalidOwnerValue(t *testing.T) {}
```

Treat these as retryable validation failures, not separate success paths.

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/planner -run 'TestRunPlannerRejectsMissingFileAssignments|TestRunPlannerRejectsDuplicateFileAssignments|TestRunPlannerRejectsInvalidOwnerValue' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Add post-parse validation that proves:

- every file in `idx` appears exactly once
- no file appears in two modules
- owner is only `agent` or `shared_preprocessor`
- `GeneratedAt` is set before write-out

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Write the artifact and replace the `plan` command stub

**Files:**
- Modify: `internal/planner/planner.go`
- Modify: `cmd/wikismit/plan.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing command tests**

Add:

```go
func TestPlanCommandWritesNavPlanArtifact(t *testing.T) {}
func TestPlanCommandReportsNavPlanLocation(t *testing.T) {}
```

Cover these behaviors:

- `wikismit plan` runs Phase 1 + Phase 2 and writes `nav_plan.json`
- the command prints a stable path-oriented success message
- the command does not continue into Phase 3+

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestPlanCommandWritesNavPlanArtifact|TestPlanCommandReportsNavPlanLocation' -v
```

Expected: FAIL because `plan` is still a stub.

**Step 3: Write minimal implementation**

Implement artifact write + command wiring by:

- writing `nav_plan.json` through `pkg/store`
- replacing the stub in `cmd/wikismit/plan.go`
- reusing existing config-loading flow and Phase 1 analyzer orchestration

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the full Phase 2 slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run planner + CLI focused tests**

Run:

```bash
go test ./internal/planner ./cmd/wikismit -v
```

Expected: PASS except for any clearly pre-existing master failures unrelated to the touched files.

**Step 2: Run a Phase 2 smoke check**

Run:

```bash
ART_DIR=$(mktemp -d)
OPENAI_API_KEY=phase2-smoke ./wikismit plan --config ./config.yaml.example --repo ./testdata/sample_repo --artifacts "$ART_DIR"
```

Expected:

- `file_index.json` exists in `$ART_DIR`
- `dep_graph.json` exists in `$ART_DIR`
- `nav_plan.json` exists in `$ART_DIR`
- command output reports the nav-plan path

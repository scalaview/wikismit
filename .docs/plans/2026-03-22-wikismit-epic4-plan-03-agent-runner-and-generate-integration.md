# Epic 4 Agent Runner and Generate Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement single-agent execution, Phase 4 reporting, and `wikismit generate` integration so non-shared module docs are produced after Phase 3.

**Architecture:** First lock the behavior of one module agent in isolation using the existing mock LLM client. Then layer Phase 4 summary reporting on top of scheduler results, and only after that extend `generate` to load Phase 2+3 artifacts and invoke the scheduler for non-shared modules.

**Tech Stack:** Go, standard `testing`, `context`, `time`, existing `internal/llm`, existing `pkg/store`, existing Cobra command package.

---

### Task 1: Add failing tests for single-agent execution

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`

**Step 1: Write the failing agent-run tests**

Add:

```go
func TestRunAgentReturnsModuleDocOnSuccess(t *testing.T) {}
func TestRunAgentReturnsModuleDocErrorOnFailure(t *testing.T) {}
```

Cover these behaviors:

- successful LLM response populates `ModuleDoc.Content`
- failed LLM response populates `ModuleDoc.Err` and leaves `Content` empty

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent -run 'TestRunAgentReturnsModuleDocOnSuccess|TestRunAgentReturnsModuleDocErrorOnFailure' -v
```

Expected: FAIL because `runAgent` does not exist.

**Step 3: Write minimal implementation**

Implement a single-module runner that:

- builds the prompt with `BuildAgentPrompt`
- calls `llm.Client.Complete` using `cfg.LLM.AgentModel`
- returns `ModuleDoc{ModuleID, Content, Err}`

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 2: Add failing timing and summary-report tests

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/agent_test.go`

**Step 1: Write the failing reporting tests**

Add:

```go
func TestRunAgentLogsCompletionTiming(t *testing.T) {}
func TestFormatPhase4SummaryIncludesFailuresOnlyWhenPresent(t *testing.T) {}
```

Cover these behaviors:

- success/failure logs include module ID and elapsed time
- Phase 4 summary always reports success/total count
- failed modules section appears only when failures exist

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent -run 'TestRunAgentLogsCompletionTiming|TestFormatPhase4SummaryIncludesFailuresOnlyWhenPresent' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Add timing around the LLM call and implement a small summary-format helper used by the Phase 4 integration path.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Wire Phase 4 into `generate` after loading Phase 2 and 3 artifacts

**Files:**
- Modify: `cmd/wikismit/generate.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing generate-command tests**

Add:

```go
func TestGenerateCommandRunsPhase4ForNonSharedModules(t *testing.T) {}
func TestGenerateCommandSkipsSharedModulesDuringPhase4Fanout(t *testing.T) {}
```

Cover these behaviors:

- `generate` loads `nav_plan.json` and `shared_context.json`
- only modules with `Owner == "agent"` are scheduled for Phase 4
- Phase 4 results write module docs under `artifacts/module_docs/`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestGenerateCommandRunsPhase4ForNonSharedModules|TestGenerateCommandSkipsSharedModulesDuringPhase4Fanout' -v
```

Expected: FAIL because `generate` still stops after Phase 1.

**Step 3: Write minimal implementation**

Extend `cmd/wikismit/generate.go` to:

- run Phase 1, load `file_index.json` + `dep_graph.json`
- load `nav_plan.json` and `shared_context.json`
- build an `AgentInput` base value
- filter non-shared modules and call the scheduler

Keep the wiring minimal; do not start Phase 5 in this Epic.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Add failing partial-failure integration coverage

**Files:**
- Modify: `internal/agent/agent_test.go`
- Modify: `cmd/wikismit/main_test.go`

**Step 1: Write the failing partial-failure tests**

Add:

```go
func TestPhase4PartialFailureWritesOnlySuccessfulModuleDocs(t *testing.T) {}
func TestGenerateCommandReportsPhase4SummaryToStderr(t *testing.T) {}
```

Cover these behaviors:

- 1 failing module still allows successful module docs to be written
- failed module file is absent
- summary output reaches `stderr`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent ./cmd/wikismit -run 'TestPhase4PartialFailureWritesOnlySuccessfulModuleDocs|TestGenerateCommandReportsPhase4SummaryToStderr' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Finish the integration path so scheduler failures are summarized and successful module docs are preserved.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the full Epic 4 slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run agent + CLI verification**

Run:

```bash
go test ./internal/agent ./cmd/wikismit -v
```

Expected: PASS.

**Step 2: Run broader package verification**

Run:

```bash
go test ./internal/agent ./internal/planner ./internal/preprocessor ./cmd/wikismit -v
```

Expected: PASS.

**Step 3: Run a Phase 4 smoke check against the sample repo**

Use a focused test or small harness so that after the Phase 4 path runs:

- `artifacts/module_docs/` exists
- one `.md` file exists per non-shared module
- shared modules are not emitted into `module_docs/`
- failure summary output is empty in the all-success case

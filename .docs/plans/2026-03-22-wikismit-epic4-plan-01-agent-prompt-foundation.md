# Epic 4 Agent Prompt Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Phase 4 agent input types and prompt builder so each module agent gets the correct skeleton, direct shared context, and ownership constraints.

**Architecture:** First lock the type surface for Phase 4 so later scheduler and runner code can reuse one stable input contract. Then implement prompt construction in TDD slices: no shared dependencies, shared dependency injection, and explicit citation/ownership instructions.

**Tech Stack:** Go, standard `testing`, existing `internal/planner`, existing `pkg/store`, existing `internal/llm.MockClient`.

---

### Task 1: Define Phase 4 input and result types before any prompt construction

**Files:**
- Create: `internal/agent/types.go`
- Create: `internal/agent/prompt_test.go`

**Step 1: Write the failing type-usage test**

Add:

```go
func TestBuildAgentPromptUsesAgentInputModuleAndArtifacts(t *testing.T) {}
```

Use `AgentInput` in the test so the package does not compile until the types exist.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/agent -run TestBuildAgentPromptUsesAgentInputModuleAndArtifacts -v
```

Expected: FAIL because `internal/agent` types do not exist yet.

**Step 3: Write minimal implementation**

Define in `internal/agent/types.go`:

```go
type AgentInput struct {
    Module        store.Module
    FileIndex     store.FileIndex
    SharedContext store.SharedContext
    Config        *config.Config
}

type ModuleDoc struct {
    ModuleID string
    Content  string
    Err      error
}
```

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: compile progresses past the missing-type failure.

### Task 2: Build the base prompt for a module with no shared dependencies

**Files:**
- Create: `internal/agent/prompt.go`
- Modify: `internal/agent/prompt_test.go`

**Step 1: Write the failing base prompt test**

Add:

```go
func TestBuildAgentPromptOmitsSharedContextWhenModuleHasNoSharedDeps(t *testing.T) {}
```

Assert that the prompt contains:

- the module ID framing
- the code skeleton section
- the Markdown section instructions

And does not contain `## Shared modules` when `DependsOnShared` is empty.

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/agent -run TestBuildAgentPromptOmitsSharedContextWhenModuleHasNoSharedDeps -v
```

Expected: FAIL because `BuildAgentPrompt` does not exist.

**Step 3: Write minimal implementation**

Implement:

```go
func BuildAgentPrompt(input AgentInput) string
```

Requirements:

- build the module skeleton via `planner.BuildSkeleton(input.Module.Files, input.FileIndex, input.Config.Agent.SkeletonMaxTokens)`
- include the Phase 4 Markdown instructions from the task/spec
- omit the shared module block when there are no shared dependencies

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 3: Inject only declared shared dependencies into the prompt

**Files:**
- Modify: `internal/agent/prompt.go`
- Modify: `internal/agent/prompt_test.go`

**Step 1: Write the failing shared-context prompt test**

Add:

```go
func TestBuildAgentPromptInjectsDeclaredSharedDependenciesOnly(t *testing.T) {}
```

Cover these behaviors:

- `DependsOnShared: []string{"logger"}` injects the logger summary
- summaries not listed in `DependsOnShared` are absent
- shared context is formatted as the shared-module block with summary, key functions, and shared-doc link

**Step 2: Run test to confirm RED**

Run:

```bash
go test ./internal/agent -run TestBuildAgentPromptInjectsDeclaredSharedDependenciesOnly -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Format the shared block from `input.Module.DependsOnShared` in order, looking up each summary in `input.SharedContext`.

**Step 4: Run test to confirm GREEN**

Run the same command. Expected: PASS.

### Task 4: Lock citation and ownership instructions in the prompt contract

**Files:**
- Modify: `internal/agent/prompt.go`
- Modify: `internal/agent/prompt_test.go`

**Step 1: Write the failing instruction-contract tests**

Add:

```go
func TestBuildAgentPromptIncludesCitationFormatInstruction(t *testing.T) {}
func TestBuildAgentPromptIncludesSharedOwnershipConstraint(t *testing.T) {}
```

Assert the prompt contains:

- `[FuncName](path/to/file.go#L{line})`
- `Do NOT describe shared modules`

**Step 2: Run tests to confirm RED**

Run:

```bash
go test ./internal/agent -run 'TestBuildAgentPromptIncludesCitationFormatInstruction|TestBuildAgentPromptIncludesSharedOwnershipConstraint' -v
```

Expected: FAIL.

**Step 3: Write minimal implementation**

Add the explicit citation-format instruction and ownership constraint text from Epic 4 task/spec to `BuildAgentPrompt`.

**Step 4: Run tests to confirm GREEN**

Run the same command. Expected: PASS.

### Task 5: Verify the full prompt foundation slice

**Files:**
- Modify only if test-driven fixes are required

**Step 1: Run agent prompt tests**

Run:

```bash
go test ./internal/agent -run TestBuildAgentPrompt -v
```

Expected: PASS.

**Step 2: Run broader package verification**

Run:

```bash
go test ./internal/planner ./internal/preprocessor ./internal/agent -v
```

Expected: PASS.

**Step 3: Spot-check prompt content with a mock invocation**

Add or reuse a focused test helper so `MockClient.Calls()[0].UserMsg` can be inspected for the final prompt shape.

Expected:

- prompt contains skeleton text
- prompt contains shared block only for declared shared dependencies
- prompt contains ownership and citation instructions

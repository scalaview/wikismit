# Plan 02 — Inject Cross-Module Context into Agent Prompts

**Epic ref:** S9D.2
**Depends on:** Plan 01

---

## Goal

Extend `AgentInput` with architecture summary and neighbor info. Inject two new prompt sections when the data is available. Wire through pipeline.

---

## Implementation steps

### Step 1: Extend AgentInput

**File:** `internal/agent/types.go`

```go
type AgentInput struct {
    Module            store.Module
    FileIndex         store.FileIndex
    SharedContext     store.SharedContext
    Config            *configpkg.Config
    ArchSummary       *store.ArchSummary   // NEW
    NeighborSummaries map[string]string     // NEW: module ID -> brief description
}
```

### Step 2: Inject architecture context into prompt

**File:** `internal/agent/prompt.go`

In `BuildAgentPrompt`, before existing `## Code skeleton` section, conditionally add:

```
## System Architecture
Purpose: {ArchSummary.Purpose}
Layers: {ArchSummary.Layers joined by " > "}
Data flow: {ArchSummary.DataFlow}

## This Module's Role
Upstream dependencies: {NeighborSummaries for modules this depends on}
Downstream consumers: {NeighborSummaries for modules that depend on this}
```

Only inject when `ArchSummary != nil`.

Build neighbor info from `input.Module.DependsOnShared` + `input.Module.ReferencedBy` matched against `input.NeighborSummaries`.

### Step 3: Wire through pipeline

**File:** `internal/pipeline/incremental.go`

In `RunFullGenerate` (line ~74) and `RunIncremental` (line ~139):
- After `plan` is available, extract `plan.ArchitectureSummary`
- Build `NeighborSummaries` from plan modules: for each module, create a brief string like `"module_id (X files, shared: true/false)"`
- Pass both to `agent.AgentInput`

### Step 4: Add prompt tests

**File:** `internal/agent/prompt_test.go`

- Test: prompt includes architecture context when `ArchSummary` is present
- Test: prompt omits architecture context when `ArchSummary` is nil (backward compat)
- Test: neighbor summaries appear correctly

---

## Verification

```bash
go test ./internal/agent/ -v
go test ./internal/pipeline/ -v
```

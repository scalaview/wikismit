# Plan 01 — Extend NavPlan with Architecture Summary

**Epic ref:** S9D.1
**Depends on:** (none)

---

## Goal

Add `ArchSummary` struct and wire it into `NavPlan`. Extend Planner prompt to request `architecture_summary` from the LLM. Parse and validate the new field.

---

## Implementation steps

### Step 1: Define ArchSummary struct

**File:** `pkg/store/artifacts.go`

Add after `NavPlan`:

```go
type ArchSummary struct {
    Purpose    string   `json:"purpose"`
    Layers     []string `json:"layers,omitempty"`
    DataFlow   string   `json:"data_flow,omitempty"`
    KeyModules []string `json:"key_modules,omitempty"`
}
```

Add `ArchitectureSummary *ArchSummary` field to `NavPlan`:

```go
type NavPlan struct {
    GeneratedAt         time.Time    `json:"generated_at"`
    Modules             []Module     `json:"modules"`
    ArchitectureSummary *ArchSummary `json:"architecture_summary,omitempty"`
}
```

Use pointer so zero-value NavPlan (no `architecture_summary` key in JSON) deserializes to nil — backward compatible.

### Step 2: Add roundtrip test for ArchSummary

**File:** `pkg/store/artifacts_test.go`

Test that `NavPlan` with `ArchitectureSummary` marshals/unmarshals correctly, and that `NavPlan` without it still deserializes (backward compat).

### Step 3: Extend Planner prompt

**File:** `internal/planner/prompt.go`

Modify `buildPlannerPrompt` to append `architecture_summary` to the expected JSON schema:

```
Schema: { modules: [...], architecture_summary: { purpose, layers, data_flow, key_modules } }
```

Add example showing the new field. Keep existing module grouping instructions unchanged.

### Step 4: Add basic validation for ArchSummary

**File:** `internal/planner/planner.go`

In `validateNavPlan`, after existing validation:
- If `ArchitectureSummary` is present, `Purpose` must be non-empty
- `Layers`, `DataFlow`, `KeyModules` may be empty (graceful degradation)

### Step 5: Update Planner prompt test

**File:** `internal/planner/planner_test.go` (or `prompt_test.go` if it exists)

Verify `buildPlannerPrompt` output includes `architecture_summary` in schema instructions.

---

## Verification

```bash
go test ./pkg/store/ -v
go test ./internal/planner/ -v
```

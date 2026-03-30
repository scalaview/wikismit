# Epic 8A.1 Fresh-run Full Generate Orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `wikismit generate` a true from-scratch entrypoint by delegating directly to the shared five-phase full-generate pipeline.

**Architecture:** Keep `internal/pipeline.RunFullGenerate` as the single source of truth for the full generate flow. Turn `cmd/wikismit/runGenerate` into a thin command wrapper that only supplies `context.Background()`, config, and client, then rely on pipeline and agent tests for phase-specific behavior. Refresh command tests so they lock the CLI entrypoint and stderr behavior instead of the deleted command-internal orchestration.

**Tech Stack:** Go, Cobra CLI, `internal/pipeline`, `internal/analyzer`, `internal/planner`, `internal/preprocessor`, `internal/agent`, `internal/composer`, `pkg/store`, standard `testing`.

---

### Task 1: Lock the new generate entrypoint with failing tests

**Files:**
- Modify: `cmd/wikismit/main_test.go:278-329`
- Modify: `cmd/wikismit/main_test.go:754-850`
- Test: `cmd/wikismit/main_test.go`

- [ ] **Step 1: Add a CLI fresh-run success test**

Add a new test named `TestGenerateCommandRunsFullPipelineFromEmptyArtifacts` near the existing generate command coverage. Use `executeCLI`, `writeCLIConfig`, and the existing command-factory seam. Seed **no** artifacts in the temp artifact directory.

Use a mock client response sequence that matches the full pipeline:

```go
llm.NewMockClient(
    `{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`,
    `{"summary":"Shared logger helpers.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() *Logger","ref":"pkg/logger/logger.go#L1"}]}`,
    "# Auth doc\n",
)
```

Assert that one CLI run writes all of:

```text
artifacts/file_index.json
artifacts/dep_graph.json
artifacts/nav_plan.json
artifacts/shared_context.json
artifacts/module_docs/auth.md
output/index.md
artifacts/validation_report.json
```

- [ ] **Step 2: Add a delegation-focused RED test for the shared full-generate path**

Replace `TestGenerateCommandLoadsDepGraphForPhase5` with `TestGenerateCommandDelegatesToSharedFullGeneratePath`. Introduce a package-level seam in `cmd/wikismit/generate.go`:

```go
var runFullGenerate = pipeline.RunFullGenerate
```

Then, in the test, override that seam and assert `runGenerate(newGenerateCmd(), cfg, client)` delegates once with the same `cfg` and `client` instance:

```go
called := 0
originalRunFullGenerate := runFullGenerate
runFullGenerate = func(ctx context.Context, gotCfg *configpkg.Config, gotClient llm.Client) error {
    called++
    if gotCfg != cfg {
        t.Fatalf("cfg pointer mismatch")
    }
    if gotClient != client {
        t.Fatalf("client mismatch")
    }
    return nil
}
```

Expected after implementation: `called == 1`, and the test no longer depends on any command-side artifact reads.

- [ ] **Step 3: Update the stderr-summary test fixture to full-pipeline inputs**

Rewrite `TestGenerateCommandReportsPhase4SummaryToStderr` so the mock response sequence includes planner output and any required shared-context output **before** the phase-4 failure is injected. Keep the stderr assertions (`"Phase 4 complete:"` plus at least one failed module id), but stop relying on prewritten `nav_plan.json` / `shared_context.json` as command-owned inputs.

- [ ] **Step 4: Run focused generate/update tests to confirm RED**

Run:

```bash
go test ./cmd/wikismit -run 'TestGenerateCommandRunsFullPipelineFromEmptyArtifacts|TestGenerateCommandDelegatesToSharedFullGeneratePath|TestGenerateCommandReportsPhase4SummaryToStderr|TestUpdateCommandFallsBackToGenerateWhenArtifactsMissing' -v
```

Expected: FAIL because `runGenerate` still executes command-owned phase logic instead of delegating through the shared full-generate seam.

- [ ] **Step 5: Commit the RED test slice**

```bash
git add cmd/wikismit/main_test.go
git commit -m "test: lock generate command full-pipeline delegation"
```

### Task 2: Route `generate` through the shared full-generate helper

**Files:**
- Modify: `cmd/wikismit/generate.go:3-90`

- [ ] **Step 1: Replace the command-owned orchestration with one delegation call**

Add the command-local seam:

```go
var runFullGenerate = pipeline.RunFullGenerate
```

then shrink `runGenerate` to:

```go
func runGenerate(_ *cobra.Command, cfg *configpkg.Config, client llm.Client) error {
    return runFullGenerate(context.Background(), cfg, client)
}
```

Do not add a second orchestration helper. The seam must only point at `internal/pipeline.RunFullGenerate` so the shared implementation path remains singular.

- [ ] **Step 2: Remove now-dead command-only imports and seams**

Delete imports that only supported the old command-owned orchestration path:

```go
"errors"
"fmt"
"github.com/scalaview/wikismit/internal/agent"
"github.com/scalaview/wikismit/internal/analyzer"
"github.com/scalaview/wikismit/internal/composer"
"github.com/scalaview/wikismit/pkg/store"
```

Do **not** delete the new `runFullGenerate` seam; it is part of the testability contract for the thin command wrapper.

- [ ] **Step 3: Keep command-level client creation behavior unchanged**

Leave `agentClientFactory` and the `llm.NewClient(cfg.LLM)` fallback intact inside `newGenerateCmd()`. Epic 8A.1 changes orchestration ownership, not client creation.

- [ ] **Step 4: Run the same focused command tests to confirm GREEN**

Run the command from Task 1 Step 4 again.

Expected: PASS.

- [ ] **Step 5: Commit the delegation cleanup**

```bash
git add cmd/wikismit/generate.go cmd/wikismit/main_test.go
git commit -m "refactor: route generate through shared full pipeline"
```

### Task 3: Remove stale command-only assertions and re-verify shared behavior

**Files:**
- Modify: `cmd/wikismit/main_test.go:213-276`
- Modify: `cmd/wikismit/main_test.go:754-850`
- Modify: `internal/pipeline/incremental_test.go:157-175`
- Modify only if needed: `internal/pipeline/incremental.go:34-82`

- [ ] **Step 1: Retire tests that asserted deleted command responsibilities**

Review these tests after Task 2:

- `TestGenerateCommandRunsPhase4ForNonSharedModules`
- `TestGenerateCommandSkipsSharedModulesDuringPhase4Fanout`
- `TestGenerateCommandRunsComposerAfterPhase4`

Delete or rewrite them only if they no longer test command behavior. Do **not** duplicate phase-4 filtering assertions in command tests once `generate` is just a delegate. The behavior already belongs to pipeline and agent coverage such as:

- `internal/pipeline/incremental_test.go:318-358`
- `internal/pipeline/incremental_test.go:360-389`
- `internal/pipeline/incremental_test.go:453-522`

- [ ] **Step 2: Keep `update` fallback coverage unchanged**

Re-run and, only if needed, minimally adjust `TestUpdateCommandFallsBackToGenerateWhenArtifactsMissing` so it still proves:

1. stdout contains `"full generate"`
2. fallback writes `file_index.json`, `nav_plan.json`, `shared_context.json`, module docs, and `output/index.md`

Do not change the success message text unless an existing assertion forces it.

- [ ] **Step 3: Run focused package regression**

Run:

```bash
go test ./cmd/wikismit ./internal/pipeline -run 'TestGenerate|TestUpdate|TestRunFullGenerate|TestRunIncremental' -v
```

Expected: PASS.

- [ ] **Step 4: Run broader repository verification for this slice**

Run:

```bash
go test ./cmd/wikismit ./internal/pipeline -v
```

Expected: PASS.

- [ ] **Step 5: Commit the test rebalance and verification slice**

```bash
git add cmd/wikismit/main_test.go internal/pipeline/incremental_test.go
git commit -m "test: align generate coverage with shared pipeline ownership"
```

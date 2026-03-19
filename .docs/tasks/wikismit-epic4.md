# wikismit — Epic 4: Phase 4 — Agent Fan-out

**Status:** `todo`  
**Depends on:** Epic 3  
**Goal:** Given `nav_plan.json`, `file_index.json`, and `shared_context.json`, run one LLM agent per non-shared module concurrently (up to configured concurrency limit) and write per-module Markdown to `artifacts/module_docs/`.  
**Spec refs:** §4 Phase 4, §7 Key Interfaces (AgentInput, ModuleDoc)

---

## S4.1 — Prompt builder with shared context injection

**Status:** `todo`

**Description:**  
Implement `internal/agent/prompt.go`. For a given module, build the LLM prompt from the module's code skeleton, the relevant subset of `shared_context.json`, and ownership constraint instructions. Include explicit `file:line` citation format instructions.

**Acceptance criteria:**
- Module with `depends_on_shared: ["logger"]` → prompt contains logger's summary from `shared_context.json`
- Module with no shared dependencies → shared context block absent from prompt
- Prompt contains the ownership constraint instruction
- Prompt structure verified by snapshot test using `MockClient`

**Files to create:**
```
internal/agent/prompt.go
internal/agent/prompt_test.go
```

### Subtasks

#### S4.1.1 — Define `AgentInput` type

- Define in `internal/agent/types.go`:
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

#### S4.1.2 — Implement `BuildAgentPrompt(input AgentInput) string`

- Step 1 — skeleton: call `planner.BuildSkeleton(input.Module.Files, input.FileIndex, input.Config.Agent.SkeletonMaxTokens)`
- Step 2 — shared context block: for each module ID in `input.Module.DependsOnShared`, look up `input.SharedContext[id]` and format:
  ```
  ## Shared modules (do not re-describe — link only)
  
  ### {id}
  {summary}
  Key functions: {key_functions joined by ", "}
  Reference: [See full docs](../shared/{id}.md)
  ```
- Step 3 — assemble full prompt:
  ```
  You are a technical writer documenting the "{moduleID}" module of a software project.

  ## Code skeleton
  {skeleton}

  {shared_context_block if non-empty}

  ## Instructions
  - Write a Markdown document with sections: Overview, Key Types, Key Functions, Usage Notes.
  - For every function reference, include a source link: [FuncName](path/to/file.go#L{line}).
  - Do NOT describe shared modules listed above — link to them using the format shown.
  - Use clear, concise technical prose. Avoid repeating the function signature verbatim.
  ```

#### S4.1.3 — Prompt snapshot tests

- Test: module with `depends_on_shared: ["logger"]` → snapshot assert prompt contains `"## Shared modules"` and logger's summary text
- Test: module with empty `depends_on_shared` → snapshot assert prompt does NOT contain `"## Shared modules"`
- Test: prompt contains `"[FuncName](path/to/file.go#L{line})"` instruction
- Test: prompt contains the ownership constraint `"Do NOT describe shared modules"`
- Use `MockClient` with a pre-recorded response; assert `MockClient.Calls()[0].UserMsg` matches expected substrings

---

## S4.2 — Goroutine scheduler with concurrency control

**Status:** `todo`

**Description:**  
Implement `internal/agent/scheduler.go`. Use a semaphore channel to cap concurrent goroutines at the configured `concurrency` value. Each goroutine runs one agent call and sends the result to a buffered `chan ModuleDoc`. A collector writes results to `artifacts/module_docs/`.

**Acceptance criteria:**
- 8 modules with `concurrency: 2` → no more than 2 goroutines active simultaneously
- All 8 modules complete and produce output files
- No goroutine leak after completion
- Elapsed time per module and total logged at `INFO`

**Files to create:**
```
internal/agent/scheduler.go
internal/agent/scheduler_test.go
```

### Subtasks

#### S4.2.1 — Implement semaphore-based scheduler

```go
func Run(ctx context.Context, modules []store.Module, input AgentInput,
    llm llm.Client, artifactsDir string, concurrency int) error {

    results := make(chan ModuleDoc, len(modules))
    sem := make(chan struct{}, concurrency)
    var wg sync.WaitGroup

    for _, mod := range modules {
        wg.Add(1)
        sem <- struct{}{}
        go func(m store.Module) {
            defer wg.Done()
            defer func() { <-sem }()
            doc := runAgent(ctx, m, input, llm)
            results <- doc
        }(mod)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    return collectResults(results, artifactsDir)
}
```

#### S4.2.2 — Implement `collectResults`

- Drain the `results` channel until closed
- For each `ModuleDoc` with `Err == nil`: write `{artifactsDir}/module_docs/{moduleID}.md`
- For each `ModuleDoc` with `Err != nil`: append to a `failures []ModuleDoc` slice
- After draining: if `len(failures) > 0`, log each failure at `ERROR` and return a summary error:
  `"Phase 4 completed with {n} failures: [{moduleID1}, {moduleID2}]"`
- Partial success (some modules failed, some succeeded) does NOT return a non-zero exit code in v1

#### S4.2.3 — Concurrency enforcement test

- Implement a test using a `MockClient` that records peak concurrent call count:
  ```go
  var (
      mu          sync.Mutex
      active      int
      peakActive  int
  )
  // MockClient.Complete increments active, sleeps 10ms, decrements active
  // tracks peak
  ```
- Assert `peakActive <= concurrency` for `concurrency: 2` and 8 modules

#### S4.2.4 — Scheduler integration test

- Test: 4 modules, all succeed → 4 `.md` files written to temp `module_docs/` dir
- Test: 4 modules, 1 fails → 3 `.md` files written, failure logged, no error returned from `Run`
- Test: context cancelled before all goroutines complete → in-flight agents receive cancelled context, no goroutine leak (verified by `goleak` or `sync.WaitGroup` timeout)

---

## S4.3 — Agent execution and partial failure handling

**Status:** `todo`

**Description:**  
Implement `internal/agent/agent.go`. Each agent builds its prompt, calls the LLM, and returns a `ModuleDoc`. On failure after retries, `ModuleDoc.Err` is set; the scheduler is not interrupted. A placeholder file is not written for failed modules.

**Acceptance criteria:**
- 4 modules, 1 agent always fails → 3 valid `.md` files, 1 failure reported at the end
- Failing module's `.md` file is not written (no partial/empty files)
- Phase 4 summary is printed to `stderr` at the end
- `MockClient` test: first call errors, remaining succeed; assert 3 files written

**Files to create:**
```
internal/agent/agent.go
internal/agent/agent_test.go
```

### Subtasks

#### S4.3.1 — Implement `runAgent(ctx, module, input, llm) ModuleDoc`

- Build prompt: `prompt := BuildAgentPrompt(AgentInput{Module: module, ...})`
- Call LLM: `content, err := llm.Complete(ctx, CompletionRequest{Model: cfg.LLM.AgentModel, UserMsg: prompt, MaxTokens: cfg.LLM.MaxTokens})`
- On error: return `ModuleDoc{ModuleID: module.ID, Err: err}`
- On success: return `ModuleDoc{ModuleID: module.ID, Content: content}`

#### S4.3.2 — Per-module timing log

- Record `start := time.Now()` before the LLM call
- After completion (success or failure): `INFO "Phase 4: module {id} completed in {elapsed}"` or `ERROR "Phase 4: module {id} failed in {elapsed}: {err}"`

#### S4.3.3 — Phase 4 summary report

- After `collectResults` drains the channel, print a summary to `stderr`:
  ```
  Phase 4 complete: {success}/{total} modules documented
  Failed modules:
    - {moduleID}: {err}
  ```
- Successful count and total are always printed; failed section only appears if there are failures

#### S4.3.4 — Wire Phase 4 into `generate` command

- In `cmd/wikismit/generate.go`, after Phase 3 completes:
  - Load `nav_plan.json` and `shared_context.json` from store
  - Filter modules to non-shared only: `[m for m in plan.Modules if m.Owner == "agent"]`
  - Call `agent.Run(ctx, nonSharedModules, input, llmClient, cfg.ArtifactsDir, cfg.Agent.Concurrency)`

#### S4.3.5 — Agent unit tests

- Test: `MockClient` returns valid Markdown → `ModuleDoc.Content` contains the response, `Err` is nil
- Test: `MockClient` returns an error (after retries exhausted) → `ModuleDoc.Err` is non-nil, `Content` is empty
- Test: 4 modules where module 2's `MockClient` call errors → `collectResults` writes 3 files, returns summary error listing module 2
- Test: assert no `.md` file written for the failed module

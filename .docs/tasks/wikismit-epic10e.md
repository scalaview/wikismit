# wikismit — Epic 10e: Function Summary Agent

**Status:** `todo`
**Depends on:** Epic 10d (linked internal call metadata in `FileIndex`)
**Goal:** Implement a reusable `FunctionSummaryAgent` in `internal/agent` that walks `store.FileIndex`, summarizes functions in internal-dependency order, batches LLM requests by estimated context size, and writes summaries back to `FunctionDecl.Summary` in place.
**Spec refs:** `.docs/spec/function-summary-agent-design.md` Overview, Dependency Graph, Batch Sizing, Prompt Construction, Response Parsing, Error Handling

---

## S10e.1 — Prompt package support for function-summary requests

**Status:** `todo`

**Description:**
Extend `internal/agent/prompt` so the function-summary agent can render the existing `function_user_prompt.tmpl` template. Keep the prompt package focused on prompt-template input types; the LLM response DTOs belong in `internal/agent`, where they are parsed and applied.

**Acceptance criteria:**
- `FunctionUserPromptTmp` is parsed in `internal/agent/prompt/prompt.go` during package init
- Existing module prompt parsing behavior remains unchanged
- `internal/agent/prompt/prompt.go` continues to own prompt-input structs (`FunctionStruct`, `CalledFunctionStruct`) only
- The plan does **not** change `function_user_prompt.tmpl` contents unless a later test proves the current template is insufficient

**Files to modify:**
```
internal/agent/prompt/prompt.go
```

**Files to create:**
```
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.1.1 — Add parsed user-template handle

Add a parsed template variable alongside the existing module/function prompt variables:

```go
var (
    ModuleSystemPromptTmp   *template.Template
    ModuleUserPromptTmp     *template.Template
    FunctionSystemPromptTmp *template.Template
    FunctionUserPromptTmp   *template.Template
)
```

Initialize it in `init()`:

```go
FunctionUserPromptTmp = template.Must(template.New("function_user_prompt").Parse(FunctionUserPrompt))
```

#### S10e.1.2 — Keep prompt-package boundaries narrow

Do **not** add function-summary response DTOs to the prompt package. Keep response decoding types local to `internal/agent/function_summary.go` so the agent package can parse them without relying on unexported identifiers across packages.

#### S10e.1.3 — Add prompt package smoke coverage through agent tests

In `internal/agent/function_summary_test.go`, add a small prompt-rendering test that proves:
- `FunctionUserPromptTmp` executes successfully
- rendered prompt contains the function path, source, and injected callee summary block when provided

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummaryBuildPrompt' -v
```

Expected: PASS

---

## S10e.2 — Introduce reusable `FunctionSummaryAgent` and run-scoped state

**Status:** `todo`

**Description:**
Replace the current `RecursionGenFunctionSummary` helper-only file with a real reusable agent instance. The instance should own only long-lived collaborators (`client`, `cfg`, `logger`), while each `Run` call creates fresh per-run state.

**Acceptance criteria:**
- `FunctionSummaryAgent` exists with fields:
  - `client llm.Client`
  - `cfg *FunctionSummaryConfig`
  - `logger logpkg.Logger`
- `NewFunctionSummaryAgent(client, cfg)` returns a reusable instance
- `Run(ctx, idx)` creates fresh per-run state and does not retain `FileIndex` / graph / summary state between calls
- Existing `FunctionDecl.Summary` values are copied into run-scoped summary state before graph construction so caller prompts can reuse them without re-requesting the callee
- `Run(ctx, idx)` returns `nil` for an empty `FileIndex`
- `Run(ctx, idx)` returns `nil` when all functions already have `Summary`
- `Run(ctx, idx)` skips functions with empty `Src`
- The old `RecursionGenFunctionSummary` helper is removed or absorbed into the new implementation so there is only one function-summary path

**Files to modify:**
```
internal/agent/function_summary.go
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.2.1 — Define reusable agent/config types

Add the public types:

```go
type FunctionSummaryConfig struct {
    Model         string
    MaxTokens     int
    ContextBudget int
    MaxRetries    int
}

type FunctionSummaryAgent struct {
    client llm.Client
    cfg    *FunctionSummaryConfig
    logger logpkg.Logger
}
```

Create a private per-run state type:

```go
type runContext struct {
    idx       store.FileIndex
    graph     *depGraph
    summaries map[FuncSign]string
    failed    map[FuncSign]error
}
```

`failed` is needed to prevent permanently failed batches from being re-selected forever.

#### S10e.2.2 — Add constructor, retry wrapping, and summary seeding

Implement:

```go
func NewFunctionSummaryAgent(client llm.Client, cfg *FunctionSummaryConfig) *FunctionSummaryAgent {
    if cfg == nil {
        cfg = &FunctionSummaryConfig{}
    }

    wrappedClient := client
    if cfg.MaxRetries > 0 {
        wrappedClient = llm.NewRetryingClient(client, cfg.MaxRetries, logger)
    }

    return &FunctionSummaryAgent{
        client: wrappedClient,
        cfg:    cfg,
        logger: logger,
    }
}
```

Also add a helper that seeds run-scoped summaries from the input index before graph construction:

```go
func seedExistingSummaries(idx store.FileIndex) map[FuncSign]string {
    summaries := make(map[FuncSign]string)
    for path, entry := range idx {
        for _, fn := range entry.Functions {
            if fn.Summary == "" {
                continue
            }
            summaries[FuncSign(path+"#"+fn.Name)] = fn.Summary
        }
    }
    return summaries
}
```

With the zero-safe constructor above, `Run` should still validate any required runtime fields (`Model`, `MaxTokens`, `ContextBudget`) and return a clear error if the agent was constructed with an unusable config.

#### S10e.2.3 — Add early-return, reuse, and seeding tests

Create tests:
- `TestFunctionSummaryAgentRunReturnsNilForEmptyIndex`
- `TestFunctionSummaryAgentRunReturnsNilWhenEverythingAlreadySummarized`
- `TestFunctionSummaryAgentRunUsesFreshStatePerInvocation`
- `TestFunctionSummaryBuildPromptUsesPreexistingCalleeSummary`

For the reuse test, use one `FunctionSummaryAgent` instance on two different `FileIndex` inputs and verify the second run does not see summaries from the first run. For the seeding test, give the caller a callee whose `Summary` is already present in the input index and assert that the caller prompt includes it without issuing a new LLM request for the callee.

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummaryAgentRun(ReturnsNil|UsesFreshState)|TestFunctionSummaryBuildPromptUsesPreexistingCalleeSummary' -v
```

Expected: PASS

---

## S10e.3 — Build dependency graph with deterministic ready-set ordering

**Status:** `todo`

**Description:**
Encode the dependency-ordering rule from the spec using a Kahn-style graph. Internal dependencies are discovered from `CallRef` values where `Ownership == store.OwnershipInternal`. The graph must be deterministic so batching and test output are reproducible.

**Acceptance criteria:**
- `FuncSign` and `fnKey.Sign()` exist and are used as the graph identity
- `depGraph` tracks `inDegree`, `deps`, `reverse`, and `pending`
- Graph construction skips:
  - functions with `Summary != ""`
  - functions with `Src == ""`
- Internal edges are derived from `call.Ownership == store.OwnershipInternal`
- Internal callee sign is derived from `call.Path + "#" + call.Name`
- Calls marked internal but missing from the current pending set are ignored instead of crashing graph construction
- `ready()` returns functions sorted by sign so batch grouping is deterministic
- `resolve(sign)` removes a summarized function from `pending` and decrements dependents
- `remaining()` returns unsummarized leftovers after the main Kahn pass

**Files to modify:**
```
internal/agent/function_summary.go
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.3.1 — Add graph identity types

Add:

```go
type FuncSign string

type fnKey struct {
    path string
    name string
}

func (f *fnKey) Sign() FuncSign {
    return FuncSign(f.path + "#" + f.name)
}
```

Use `*fnKey` throughout the graph to avoid extra copying and to align with the existing pointer-first project style.

#### S10e.3.2 — Implement `newDepGraph`

Build the graph in two passes:
1. First pass: collect all pending functions that still need summary
2. Second pass: inspect `Calls` and add internal edges only for pending callees

Pseudo-shape:

```go
func newDepGraph(idx store.FileIndex) *depGraph {
    g := &depGraph{
        inDegree: make(map[FuncSign]int),
        deps:     make(map[FuncSign][]*fnKey),
        reverse:  make(map[FuncSign][]*fnKey),
        pending:  make(map[FuncSign]*fnKey),
    }
    // pass 1: seed pending
    // pass 2: add edges for internal calls that point at pending callees
    return g
}
```

Do not verify dependency-ness via `ResolvedTarget`; the internal/external filter is `Ownership`.

#### S10e.3.3 — Implement deterministic `ready`, `resolve`, and `remaining`

- `ready()` should collect all `pending` nodes with `inDegree == 0` and sort by `FuncSign`
- `resolve(sign)` should:
  - remove the sign from `pending`
  - decrement the indegree of each dependent in `reverse[sign]`
- `remaining()` should return the unresolved pending keys, also sorted by `FuncSign`

#### S10e.3.4 — Add dependency-graph unit tests

Create table-driven tests covering:
- leaf + caller chain
- shared helper called by two functions
- preexisting callee summary removing indegree
- empty `Src` exclusion
- simple cycle (`a -> b -> a`) showing both functions remain in `remaining()` after the Kahn pass

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummaryDepGraph' -v
```

Expected: PASS

---

## S10e.4 — Add token-based batching and prompt construction

**Status:** `todo`

**Description:**
Batch only the currently ready functions. Estimate token usage locally using the same formula as `llm.estimatePromptTokens`, then build prompt payloads that include already-known callee summaries.

**Acceptance criteria:**
- A local `estimateTokens(charCount int) int` helper exists in `internal/agent/function_summary.go`
- `estimateFunctionTokens` includes:
  - source length
  - prompt overhead
  - already-known internal callee summaries from `runContext.summaries`
- A concrete `batch` shape is defined so the engineer knows what moves through `buildBatches`, `buildPrompt`, and `processBatch`
- A helper exists to resolve a `*fnKey` back to the corresponding `*store.FunctionDecl`
- `buildBatches` splits the ready set according to `ContextBudget`
- Deterministic input order produces deterministic batch composition
- If a single function exceeds `ContextBudget`, it is still emitted as a one-function batch so the run does not starve forever
- `buildPrompt` uses:
  - `prompt.FunctionSystemPrompt` as the system message
  - `prompt.FunctionUserPromptTmp` for the user message
- `buildPrompt` includes only internal callees whose summaries are already available in `runContext.summaries`

**Files to modify:**
```
internal/agent/function_summary.go
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.4.1 — Implement local token estimators

Add:

```go
func estimateTokens(charCount int) int {
    if charCount <= 0 {
        return 0
    }
    return (charCount + 3) / 4
}

func estimateFunctionTokens(fn *store.FunctionDecl, summaries map[FuncSign]string) int {
    cost := estimateTokens(len(fn.Src)) + 50
    for _, call := range fn.Calls {
        if call.Ownership != store.OwnershipInternal {
            continue
        }
        sign := FuncSign(call.Path + "#" + call.Name)
        if summary, ok := summaries[sign]; ok {
            cost += estimateTokens(len(summary))
        }
    }
    return cost
}
```

Keep the formula local because `llm.estimatePromptTokens` is unexported.

#### S10e.4.2 — Define `batch` shape and function lookup

Use a small explicit batch type:

```go
type batch struct {
    keys []*fnKey
}
```

Add a helper that resolves a graph key to the concrete declaration used by token estimation and prompt building:

```go
func lookupFunction(idx store.FileIndex, key *fnKey) *store.FunctionDecl {
    entry, ok := idx[key.path]
    if !ok {
        return nil
    }
    for _, fn := range entry.Functions {
        if fn.Name == key.name {
            return fn
        }
    }
    return nil
}
```

#### S10e.4.3 — Implement `buildBatches`

Batch algorithm:
- start with sorted `ready()` output
- resolve each `*fnKey` to its `*store.FunctionDecl`
- keep appending until the next function would exceed `ContextBudget`
- if the current batch is empty and one function already exceeds budget, emit that function alone
- continue until all ready functions are assigned

#### S10e.4.4 — Implement `buildPrompt`

For each batch function, resolve the `*fnKey` via `lookupFunction`, then construct `prompt.FunctionStruct`:

```go
prompt.FunctionStruct{
    Path: fn.Path,
    Src:  fn.Src,
    CalledFunctions: []*prompt.CalledFunctionStruct{...},
}
```

Populate `CalledFunctions` by iterating `FunctionDecl.Calls` and looking up `runContext.summaries[FuncSign(call.Path+"#"+call.Name)]` for internal calls only.

Then render:

```go
var userBuf bytes.Buffer
err := prompt.FunctionUserPromptTmp.Execute(&userBuf, &prompt.FunctionSystemPromptData{Functions: payload})
```

Return:
- system message = `prompt.FunctionSystemPrompt`
- user message = `userBuf.String()`

#### S10e.4.5 — Add batching/prompt unit tests

Create tests:
- `TestFunctionSummaryBuildBatchesRespectsContextBudget`
- `TestFunctionSummaryBuildBatchesKeepsOversizedFunctionAsSingleBatch`
- `TestFunctionSummaryBuildPromptIncludesOnlyKnownInternalCalleeSummaries`

Use short synthetic sources and summary strings so expected batch boundaries are stable.

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummaryBuild(Batches|Prompt)' -v
```

Expected: PASS

---

## S10e.5 — Parse responses, apply summaries, and define failure semantics

**Status:** `todo`

**Description:**
Decode the LLM JSON safely, write successful summaries back into the `FileIndex`, and make permanent failures explicit without violating dependency ordering.

**Implementation note:** This epic intentionally tightens the design spec's coarse “skip batch, continue” wording. The implementation should finish other batches in the **current ready layer**, then return a typed `FunctionSummaryRunError` and stop before later dependency layers. This preserves dependency-order safety and prevents infinite re-selection of failed ready batches.

**Acceptance criteria:**
- Response parsing uses `llm.ParseJSON` rather than ad-hoc JSON extraction
- `applySummaries` updates both:
  - `runContext.summaries`
  - `store.FileIndex[path].Functions[i].Summary`
- Unknown response items (path/id not found in the current `FileIndex`) are ignored and logged, not treated as panics
- Partial responses apply only the returned summaries
- In a normal ready layer, any requested function missing from the parsed response is treated as a failed item for that layer and recorded in `runContext.failed`
- A permanently failed batch is recorded in `runContext.failed`, and `runContext.failed` is surfaced through the returned `FunctionSummaryRunError`
- The run must **not** unlock dependent callers after a permanent batch failure, because those callers would be missing required callee summaries
- After finishing the current ready layer, `Run` returns an aggregate error if any batch in that layer failed or returned missing items, and does not advance to later dependency layers
- The implementation must not re-queue the same failed ready batch forever

**Files to modify:**
```
internal/agent/function_summary.go
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.5.1 — Implement local response DTOs and `parseResponse`

Add response DTOs in `internal/agent/function_summary.go`:

```go
type functionSummaryResult struct {
    ID      string `json:"id"`
    Path    string `json:"path"`
    Summary string `json:"summary"`
}

type functionSummaryResponse struct {
    Functions []*functionSummaryResult `json:"functions"`
}
```

Then parse with local types:

```go
func (a *FunctionSummaryAgent) parseResponse(content string) ([]*functionSummaryResult, error) {
    var resp functionSummaryResponse
    if err := llm.ParseJSON(content, &resp); err != nil {
        return nil, err
    }
    return resp.Functions, nil
}
```

Do **not** rely on unexported identifiers across packages.

#### S10e.5.2 — Implement `applySummaries`

Pseudo-shape:

```go
func (a *FunctionSummaryAgent) applySummaries(rc *runContext, results []*functionSummaryResult) {
    for _, fn := range results {
        entry, ok := rc.idx[fn.Path]
        if !ok {
            a.logger.Warn("function summary path missing from index", "path", fn.Path, "id", fn.ID)
            continue
        }

        var matched *store.FunctionDecl
        for _, decl := range entry.Functions {
            if decl.Name == fn.ID {
                matched = decl
                break
            }
        }
        if matched == nil {
            a.logger.Warn("function summary id missing from index", "path", fn.Path, "id", fn.ID)
            continue
        }

        sign := FuncSign(fn.Path + "#" + fn.ID)
        rc.summaries[sign] = fn.Summary
        matched.Summary = fn.Summary
    }
}
```

#### S10e.5.3 — Add aggregate failure reporting

Introduce a typed error, e.g.:

```go
type FunctionSummaryRunError struct {
    Failed  map[FuncSign]error
    Blocked []FuncSign
}
```

Behavior:
- if one batch fails after retrying, record every function in that batch in `Failed`
- if one batch parses successfully but omits some requested functions, record those missing function signs in `Failed` as well
- continue other batches in the **same** ready layer so independent leaves can still succeed
- after the layer finishes, return `FunctionSummaryRunError`
- do not move into the next dependency layer once there are failures, because callers would be summarized without their callee summaries

This is the key guard against both infinite retry loops and dependency-order violations.

#### S10e.5.4 — Add response/failure tests

Create tests:
- `TestFunctionSummaryParseResponseAcceptsBareJSONAndFencedJSON`
- `TestFunctionSummaryApplySummariesUpdatesIndexAndSummaryMap`
- `TestFunctionSummaryRunReturnsAggregateErrorAfterLayerFailure`
- `TestFunctionSummaryRunDoesNotUnlockCallerAfterCalleeBatchFailure`

For the failure test, configure the mock client so:
- batch 1 returns malformed JSON or a retryable error that exhausts retries
- batch 2 in the same layer still succeeds
- the caller depending on the failed callee is **not** summarized

Also add `TestFunctionSummaryRunTreatsMissingBatchResultsAsFailure`, where the parsed response omits one requested zero-indegree function from a normal ready layer; assert that the omitted function is recorded in `FunctionSummaryRunError` and is not re-selected on a later loop.

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummary(ParseResponse|ApplySummaries|RunReturnsAggregateError|RunDoesNotUnlockCaller|RunTreatsMissingBatchResultsAsFailure)' -v
```

Expected: PASS

---

## S10e.6 — Implement end-to-end `Run` flow

**Status:** `todo`

**Description:**
Wire the graph, batching, prompt building, LLM execution, response parsing, cycle fallback, and in-place summary updates into the agent’s `Run` method.

**Acceptance criteria:**
- `Run` creates a fresh `runContext`
- `Run` seeds `runContext.summaries` from preexisting `FunctionDecl.Summary` values before graph construction
- `Run` executes Kahn layers in order until there are no ready functions left
- Each ready layer is batched using `ContextBudget`
- Successful batch results are applied immediately and resolved in the graph
- Remaining nodes after the normal Kahn loop are treated as cycle leftovers and processed as one final un-split batch to match the approved spec
- The cycle fallback uses whatever summaries are already available, but does not require unresolved cycle peers to have summaries first
- If the final cycle batch returns only a partial response, any still-unresolved cycle members are surfaced through `FunctionSummaryRunError` rather than being treated as a successful run
- Existing `Summary` values are preserved and never re-requested from the LLM
- The same `FunctionSummaryAgent` instance can be reused across multiple runs

**Files to modify:**
```
internal/agent/function_summary.go
internal/agent/function_summary_test.go
```

### Subtasks

#### S10e.6.1 — Implement `processBatch`

`processBatch` should:
1. build system/user prompt
2. call `a.client.Complete(ctx, req)`
3. parse the JSON response
4. compare the requested function signs with the returned items; treat any missing requested signs as failed for the current layer
5. apply summaries
6. resolve each successfully summarized function in the graph

Construct the request with:

```go
req := &llm.CompletionRequest{
    Model:     a.cfg.Model,
    SystemMsg: systemMsg,
    UserMsg:   userMsg,
    MaxTokens: a.cfg.MaxTokens,
}
```

#### S10e.6.2 — Implement `processLayer`

Within one ready layer:
- build batches once from the sorted ready set
- process them sequentially
- collect any permanent failures or missing-response items into `runContext.failed`
- if any batch failed or returned missing items, return an aggregate layer error after all same-layer batches finish

#### S10e.6.3 — Implement `Run`

Recommended shape:

```go
func (a *FunctionSummaryAgent) Run(ctx context.Context, idx store.FileIndex) error {
    rc := &runContext{
        idx:       idx,
        summaries: seedExistingSummaries(idx),
        failed:    make(map[FuncSign]error),
    }
    rc.graph = newDepGraph(idx)
    if len(rc.graph.pending) == 0 {
        return nil
    }

    for {
        ready := rc.graph.ready()
        if len(ready) == 0 {
            break
        }
        if err := a.processLayer(ctx, rc, ready); err != nil {
            return err
        }
    }

    remaining := rc.graph.remaining()
    if len(remaining) == 0 {
        return nil
    }
    return a.processBatch(ctx, rc, &batch{keys: remaining})
}
```

Use the final cycle pass only for actual unresolved leftovers after successful normal layers — not as a fallback after a failed ready layer. The cycle fallback intentionally bypasses normal `ContextBudget` splitting and sends one final batch, matching the approved spec. After that final batch, re-check `rc.graph.remaining()`: if any cycle members are still unresolved because the response was partial or malformed, return `FunctionSummaryRunError` instead of reporting success.

#### S10e.6.4 — Add end-to-end run tests

Create tests:
- `TestFunctionSummaryRunProcessesLeafBeforeCaller`
- `TestFunctionSummaryRunReusesExistingSummaryWithoutReRequesting`
- `TestFunctionSummaryRunProcessesCycleInFinalPass`
- `TestFunctionSummaryRunReturnsErrorWhenCycleBatchIsPartial`
- `TestFunctionSummaryAgentRetriesTransientLLMFailure`

Example mock responses:

```json
{"functions":[{"id":"helper","path":"internal/auth/helper.go","summary":"internal/auth/helper.go#helper\nSummary: Validates the token payload and returns parsed claims."}]}
```

```json
{"functions":[{"id":"Handle","path":"internal/api/handler.go","summary":"internal/api/handler.go#Handle\nSummary: Handles the request and delegates token validation to the auth helper before building the response.\nCall relationships: internal/auth/helper.go#helper"}]}
```

For retry coverage, configure `llm.NewMockClient(...).WithErrors(...)` with a retryable `*llm.LLMError{StatusCode: 500, Retryable: true}` on the first call and a valid JSON response on the second call.

Run:
```bash
go test ./internal/agent -run 'TestFunctionSummaryRun|TestFunctionSummaryAgentRetriesTransientLLMFailure' -v
```

Expected: PASS

---

## S10e.7 — Final verification and scope guard

**Status:** `todo`

**Description:**
Verify the new function-summary agent is correct, deterministic, and scoped only to `internal/agent` prompt/summary work.

**Acceptance criteria:**
- `go test ./internal/agent ./internal/llm -v` passes
- No changes are required in `internal/analyzer`, `internal/pipeline`, or `cmd/` for this epic
- Diff is limited to:
  - `internal/agent/function_summary.go`
  - `internal/agent/function_summary_test.go`
  - `internal/agent/prompt/prompt.go`
- `FunctionDecl.Summary` values are updated in place and are visible to the caller after `Run`
- Same input order produces the same batching order in tests (deterministic behavior)

### Subtasks

#### S10e.7.1 — Run targeted package tests

Run:
```bash
go test ./internal/agent ./internal/llm -v
```

Expected: PASS

#### S10e.7.2 — Run full repository smoke test

Run:
```bash
go test ./... -v
```

Expected: PASS or only pre-existing unrelated failures. If there are unrelated failures, document them before merging.

#### S10e.7.3 — Scope review before merge

Before merge, confirm:
- no pipeline wiring was added in this epic
- no analyzer/linker behavior was changed in this epic
- no module-doc (`runAgent`) behavior regressed
- no global mutable summary cache was introduced on the reusable agent instance

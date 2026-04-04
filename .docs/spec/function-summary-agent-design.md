# Function Summary Agent Design

## Overview

A long-lived agent that generates behavioral summaries for all functions in a `store.FileIndex` using LLM. It processes functions in dependency order (callees before callers) via Kahn's algorithm, batches them by token estimation to respect context limits, and writes summaries back into the FileIndex.

## Public API

```go
type FunctionSummaryAgent struct {
    client llm.Client
    cfg    *FunctionSummaryConfig
    logger logpkg.Logger
}

func NewFunctionSummaryAgent(client llm.Client, cfg *FunctionSummaryConfig) *FunctionSummaryAgent
func (a *FunctionSummaryAgent) Run(ctx context.Context, idx store.FileIndex) error
```

`FunctionSummaryAgent` is a reusable instance. Each `Run` call processes one `FileIndex`, modifying it in-place (updating `FunctionDecl.Summary` fields).

## Configuration

```go
type FunctionSummaryConfig struct {
    Model         string // LLM model to use
    MaxTokens     int    // max output tokens per LLM call
    ContextBudget int    // max estimated input tokens per batch
    MaxRetries    int    // passed to NewRetryingClient
}
```

Retry behavior is delegated to `llm.NewRetryingClient`, which provides exponential backoff with jitter and retryable error classification.

## Data Flow

```
FileIndex -> buildDepGraph -> Kahn's ready set -> batch by token estimation
    -> for each batch: build prompt -> call LLM -> parse JSON -> apply summaries
    -> resolve in graph -> next ready set -> repeat
    -> handle cycle leftovers (final batch)
```

## Dependency Graph

### Types

```go
type FuncSign string

type fnKey struct {
    path string
    name string
}

func (f *fnKey) Sign() FuncSign

type depGraph struct {
    inDegree map[FuncSign]int       // unresolved internal dep count
    deps     map[FuncSign][]*fnKey  // fn -> its internal callees
    reverse  map[FuncSign][]*fnKey  // fn -> dependents
    pending  map[FuncSign]*fnKey    // all unresolved functions
}
```

### Graph Construction (`newDepGraph`)

1. Iterate all `FunctionDecl` in `FileIndex`
2. Skip functions where `Summary != ""` (pre-resolved)
3. Skip functions where `Src == ""` (nothing to summarize)
4. For each `CallRef` where `Ownership == OwnershipInternal`:
   - Look up callee in FileIndex by `CallRef.Path + CallRef.Name`
   - If callee exists and needs summary -> record edge, increment inDegree
   - If callee already has summary or doesn't exist -> ignore (resolved/external)

### Kahn's Algorithm

- `ready()` returns all functions with `inDegree == 0`
- After processing a batch, `resolve(sign)` decrements inDegree of dependents
- `remaining()` returns unresolved functions after Kahn's completes (cycles)

### Cycle Handling

Functions remaining after Kahn's algorithm are in cyclic dependencies. Process them all in one final batch, ignoring unresolved callee summaries.

## Per-Call State

```go
type runContext struct {
    idx       store.FileIndex
    graph     *depGraph
    summaries map[FuncSign]string // populated as batches complete
}
```

Created fresh by each `Run` call. Passed through the method chain.

## Batch Sizing

Token estimation uses a local `estimateTokens` function (same formula as `llm.estimatePromptTokens`: `(charCount + 3) / 4`). The `llm` version is unexported, so we define our own.

Each function's estimated cost:

```go
func estimateTokens(charCount int) int {
    if charCount <= 0 { return 0 }
    return (charCount + 3) / 4
}

func estimateFunctionTokens(fn *store.FunctionDecl, summaries map[FuncSign]string) int {
    cost := estimateTokens(len(fn.Src)) + 50
    for _, call := range fn.Calls {
        if call.Ownership == store.OwnershipInternal {
            sign := FuncSign(call.Path + "#" + call.Name)
            if s, ok := summaries[sign]; ok {
                cost += estimateTokens(len(s))
            }
        }
    }
    return cost
}
```

Functions accumulate into a batch until adding the next function would exceed `ContextBudget`. Functions with empty `Src` are skipped (no source to summarize).

## Prompt Construction

Uses existing templates from `internal/agent/prompt/`:

- System message: `FunctionSystemPrompt` (static text)
- User message: `FunctionUserPromptTmp` (Go template with `{{range .Functions}}`)

For each function in the batch, build `FunctionStruct`:
- `Path` from `FunctionDecl.Path`
- `Src` from `FunctionDecl.Src`
- `CalledFunctions` from `FunctionDecl.Calls`, with `Summary` populated from the `summaries` map

**Note:** `FunctionUserPromptTmp` does not exist yet in `prompt.go`. It must be added.

## Response Parsing

LLM returns JSON matching this structure:

```go
type Function struct {
    ID      string `json:"id"`
    Path    string `json:"path"`
    Summary string `json:"summary"`
}

type functionSummaryResponse struct {
    Functions []*Function `json:"functions"`
}
```

Parse using `llm.ParseJSON[functionSummaryResponse](content, &resp)` — a generic helper that strips markdown fences before unmarshaling.

## Applying Summaries

`applySummaries` writes parsed results back to both the `runContext.summaries` map and the `FileIndex`:

```go
func (a *FunctionSummaryAgent) applySummaries(rc *runContext, results []*prompt.Function) {
    for _, fn := range results {
        sign := FuncSign(fn.Path + "#" + fn.ID)
        // Update summaries map for prompt construction
        rc.summaries[sign] = fn.Summary

        // Update FileIndex in-place
        entry, ok := rc.idx[fn.Path]
        if !ok { continue }
        for _, decl := range entry.Functions {
            if decl.Name == fn.ID {
                decl.Summary = fn.Summary
                break
            }
        }
    }
}
```

## Edge Cases

- **Empty FileIndex**: `Run` returns `nil` immediately (no work to do).
- **Function with empty Src**: Skipped during graph construction (nothing to summarize).
- **All functions already have summaries**: Graph is empty, `Run` returns `nil`.
- **LLM returns fewer summaries than requested**: Apply whatever was returned, unresolved functions remain in graph for next iteration.

## Error Handling

LLM retry is handled by `llm.NewRetryingClient` wrapper. Agent-level handling:

| Failure | Action |
|---------|--------|
| LLM API error (after retries) | Log error, skip batch, continue |
| Malformed JSON response | Log error, skip batch, continue |
| Context cancelled | Propagate `ctx.Err()` immediately |
| All batches done | Return summary with success/skip counts |

## Agent Methods

```go
// Long-lived agent methods
func (a *FunctionSummaryAgent) Run(ctx context.Context, idx store.FileIndex) error
func (a *FunctionSummaryAgent) processLayer(ctx context.Context, rc *runContext, ready []*fnKey) error
func (a *FunctionSummaryAgent) processBatch(ctx context.Context, rc *runContext, b *batch) error
func (a *FunctionSummaryAgent) buildPrompt(rc *runContext, b *batch) (string, string, error)
func (a *FunctionSummaryAgent) parseResponse(content string) ([]*prompt.Function, error)
func (a *FunctionSummaryAgent) applySummaries(rc *runContext, results []*prompt.Function)

// depGraph methods
func newDepGraph(idx store.FileIndex, summaries map[FuncSign]string) *depGraph
func (g *depGraph) ready() []*fnKey
func (g *depGraph) resolve(sign FuncSign)
func (g *depGraph) remaining() []*fnKey
```

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/function_summary.go` | Rewrite - agent struct, depGraph, batch logic, main loop |
| `internal/agent/prompt/prompt.go` | Add `FunctionUserPromptTmp` template variable, `Function` and `functionSummaryResponse` types |

## Main Loop (Run method)

```go
func (a *FunctionSummaryAgent) Run(ctx context.Context, idx store.FileIndex) error {
    rc := &runContext{
        idx:       idx,
        summaries: make(map[FuncSign]string),
    }
    rc.graph = newDepGraph(idx, rc.summaries)

    for {
        ready := rc.graph.ready()
        if len(ready) == 0 { break }
        if err := a.processLayer(ctx, rc, ready); err != nil { return err }
    }

    if remaining := rc.graph.remaining(); len(remaining) > 0 {
        if err := a.processLayer(ctx, rc, remaining); err != nil { return err }
    }
    return nil
}
```

## Usage

```go
cfg := &FunctionSummaryConfig{
    Model:         "gpt-4o",
    MaxTokens:     4096,
    ContextBudget: 100000,
    MaxRetries:    3,
}
agent := NewFunctionSummaryAgent(retryClient, cfg)

// Reuse across different FileIndexes
agent.Run(ctx, fileIndex1)
agent.Run(ctx, fileIndex2)
```

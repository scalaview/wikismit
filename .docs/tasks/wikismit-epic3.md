# wikismit — Epic 3: Phase 2 + 3 — Planner + Shared Preprocessor

**Status:** `todo`  
**Depends on:** Epic 2  
**Goal:** Given `file_index.json` and `dep_graph.json`, produce `nav_plan.json` with module ownership locked, then `shared_context.json` with LLM-generated summaries for shared modules ready for agent injection.  
**Spec refs:** §4 Phase 2, §4 Phase 3, §5 Artifact Schemas

---

## S3.1 — Skeleton serialiser

**Status:** `todo`

**Description:**  
Implement `internal/planner/skeleton.go`. Generate a compact, token-budget-aware text representation of a module's code structure from `FileIndex`. Contains function signatures and type names only — no bodies. Truncates by dropping unexported symbols first when the budget is exceeded.

**Acceptance criteria:**
- Skeleton for the `auth` module contains all exported signatures with `// file.go:N` annotations
- Output never exceeds `skeleton_max_tokens` (verified by token count, not character count)
- Unexported symbols dropped before exported ones during truncation
- Truncation is logged at `WARN` with the module ID and dropped symbol count

**Files to create:**
```
internal/planner/skeleton.go
internal/planner/skeleton_test.go
```

### Subtasks

#### S3.1.1 — Token count estimation

- Implement `estimateTokens(text string) int`:
  - Simple approximation: `len(text) / 4` (1 token ≈ 4 characters)
  - This is sufficient for skeleton budget enforcement; exact tokenisation not required
- Accept `maxTokens int` as a parameter; skeleton builder will call this to check budget

#### S3.1.2 — Implement `BuildSkeleton(files []string, idx store.FileIndex, maxTokens int) string`

- For each file in the module's `files` list:
  - Write a header: `// === {relative/path.go} ===`
  - For each `FunctionDecl`: write `{Signature}  // {path}:{LineStart}`
  - For each `TypeDecl`: write `type {Name} {Kind}  // {path}:{LineStart}`
- Collect all lines into two buckets: exported (uppercase) and unexported
- Build the skeleton starting with exported lines; append unexported lines while under token budget
- If adding an unexported line would exceed the budget, stop and log `WARN`
- Return the assembled skeleton string

#### S3.1.3 — Full-repo skeleton for Phase 2

- Implement `BuildFullSkeleton(idx store.FileIndex, maxTokens int) string`:
  - Concatenate per-file skeletons for all files in the index
  - Same truncation logic: exported first, then unexported, stop when budget exceeded
  - Used by the planner (Phase 2) to give the LLM a view of the entire repo

#### S3.1.4 — Skeleton unit tests

- Test: module with 2 exported funcs + 1 unexported type → all 3 appear in skeleton when under budget
- Test: module skeleton exceeds budget → unexported symbols are absent, exported symbols are all present
- Test: `estimateTokens` output for a known string matches expected approximation
- Test: annotations `// path.go:N` appear on every line

---

## S3.2 — Phase 2: module planner LLM call

**Status:** `todo`

**Description:**  
Implement `internal/planner/planner.go`. Build the planner prompt, call the LLM once, parse the JSON response into a `NavPlan`. Retry with error context on JSON parse failure. Write via `pkg/store`. Wire the `wikismit plan` subcommand.

**Acceptance criteria:**
- Given `testdata/sample_repo/` artifacts, `nav_plan.json` has every file in exactly one module
- Modules with 3+ importers are `shared: true`, `owner: "shared_preprocessor"`
- JSON parse failure triggers a retry with error appended; `MockClient` test simulates bad then good response
- `wikismit plan` runs Phase 1 + Phase 2 only and exits, printing path to `nav_plan.json`

**Files to create:**
```
internal/planner/planner.go
internal/planner/prompt.go
internal/planner/planner_test.go
```

### Subtasks

#### S3.2.1 — Compute shared module candidates

- Implement `computeInDegree(graph store.DepGraph) map[string]int`:
  - For each edge `A → B`, increment `inDegree[B]`
  - Return the complete in-degree map (files with no inbound edges have value 0)
- Implement `sharedCandidates(idx store.FileIndex, graph store.DepGraph, threshold int) map[string]bool`:
  - A file's owning module is shared if any of its files have `inDegree >= threshold`
  - Returns a set of file paths belonging to shared modules

#### S3.2.2 — Build planner prompt

- Implement `buildPlannerPrompt(skeleton string, threshold int) string` in `internal/planner/prompt.go`:
  ```
  You are a software architect. Given the repository skeleton below, group files
  into logical documentation modules. Identify shared utilities imported by {threshold}+
  modules and mark them as shared.

  Rules:
  - Every file must appear in exactly one module
  - Shared modules must have: "shared": true, "owner": "shared_preprocessor"
  - Non-shared modules must have: "shared": false, "owner": "agent"
  - depends_on_shared lists only module IDs, not file paths
  - Respond ONLY with valid JSON. No preamble, no markdown fences.

  Schema:
  {"modules": [{"id": string, "files": [string], "shared": bool, "owner": string,
   "depends_on_shared": [string], "referenced_by": [string]}]}

  Repository skeleton:
  {skeleton}
  ```

#### S3.2.3 — LLM call and JSON parsing with retry

- Implement `RunPlanner(ctx context.Context, idx store.FileIndex, graph store.DepGraph, cfg *config.Config, llm llm.Client) (*store.NavPlan, error)`:
  1. Build full skeleton via `BuildFullSkeleton`
  2. Build prompt via `buildPlannerPrompt`
  3. Call `llm.Complete` with `CompletionRequest{Model: cfg.LLM.PlannerModel, ...}`
  4. Attempt `json.Unmarshal` into `store.NavPlan`
  5. On failure: append error to prompt (`"Previous response failed JSON parse: {err}. Try again."`) and retry
  6. After 3 failures: return error wrapping all parse errors

#### S3.2.4 — NavPlan validation

- After successful JSON parse, validate the `NavPlan`:
  - Every file path in `idx` appears in exactly one module's `files` list
  - No file path appears in two modules
  - Each `owner` value is either `"agent"` or `"shared_preprocessor"`
- On validation failure, treat as a parse failure and retry with the validation error appended to the prompt

#### S3.2.5 — Write artifact and wire `plan` command

- Call `store.WriteNavPlan(cfg.ArtifactsDir, navPlan)` after successful validation
- In `cmd/wikismit/plan.go`:
  1. Call `RunPhase1` (or load existing artifacts if present)
  2. Call `RunPlanner`
  3. Print `"nav_plan.json written to {artifactsDir}"` and exit 0
- Wire `plan` command into the Cobra root command

#### S3.2.6 — Planner unit tests

- Test: `MockClient` returns valid `NavPlan` JSON → `nav_plan.json` written, all files present
- Test: `MockClient` returns invalid JSON on call 1, valid on call 2 → success after retry, 2 total calls
- Test: `MockClient` returns invalid JSON 3 times → error returned, no artifact written
- Test: module with 4 importers → `shared: true` in output
- Test: module with 2 importers → `shared: false` in output (below threshold of 3)

---

## S3.3 — Shared module topological sort

**Status:** `todo`

**Description:**  
Extract shared modules from `nav_plan.json`, build their dependency subgraph, and produce a processing order where each shared module is processed before the shared modules that depend on it.

**Acceptance criteria:**
- If `pkg/errors` is imported by `pkg/logger`, `errors` precedes `logger` in sort output
- Cycles in the shared subgraph produce a clear error and halt Phase 3
- No shared modules → empty output, Phase 3 skipped without error

**Files to create:**
```
internal/preprocessor/preprocessor.go
internal/preprocessor/topo_test.go
```

### Subtasks

#### S3.3.1 — Extract shared module subgraph

- Implement `sharedSubgraph(plan *store.NavPlan, graph store.DepGraph) map[string][]string`:
  - Filter `plan.Modules` to those with `Shared: true`
  - For each shared module's files, find edges in `graph` that point to files belonging to another shared module
  - Return a module-level adjacency list: `map[moduleID][]dependentModuleID`

#### S3.3.2 — Kahn's algorithm topological sort

- Implement `topoSort(graph map[string][]string) ([]string, error)`:
  - Standard Kahn's algorithm using in-degree counts and a processing queue
  - If the output list length < number of nodes → cycle detected → return `fmt.Errorf("cycle detected among shared modules: %v", remainingNodes)`
  - Return sorted module ID list (dependencies first)

#### S3.3.3 — Topological sort unit tests

- Test: linear chain `errors → logger → middleware` → sort output is `[errors, logger, middleware]`
- Test: two independent shared modules → both appear in output (order between them is stable but either is valid)
- Test: introduce a cycle (`A → B → A`) → error returned
- Test: empty shared module list → empty output, no error

---

## S3.4 — Phase 3: shared module serial LLM calls

**Status:** `todo`

**Description:**  
Complete `internal/preprocessor/preprocessor.go`. Iterate over shared modules in topological order, build a summary prompt for each (injecting already-completed summaries for that module's own shared dependencies), call the LLM, parse the result, and write `shared_context.json`.

**Acceptance criteria:**
- `shared_context.json` produced with `summary`, `key_types`, `key_functions`, `source_refs` for each shared module
- Prompt for `pkg/middleware` includes `pkg/logger`'s summary from the same run
- `source_refs` values match lines in `file_index.json`

**Files to create:**
```
internal/preprocessor/shared_context.go
internal/preprocessor/prompt.go
internal/preprocessor/preprocessor_test.go
```

### Subtasks

#### S3.4.1 — Build shared module summary prompt

- Implement `buildSharedPrompt(moduleID string, skeleton string, alreadySummarised store.SharedContext) string` in `internal/preprocessor/prompt.go`:
  ```
  You are documenting the shared module "{moduleID}".

  Code skeleton:
  {skeleton}

  {if len(alreadySummarised) > 0:}
  The following shared modules are used by this module.
  Use their summaries for context only — do not describe them:
  {for each dep in alreadySummarised:}
  - {dep.id}: {dep.summary}
  {end}

  Respond ONLY with valid JSON:
  {
    "summary": "2-4 sentence description of purpose and usage pattern",
    "key_types": ["TypeName1", "TypeName2"],
    "key_functions": [
      {"name": "FuncName", "signature": "func ...", "ref": "path/file.go#L18"}
    ]
  }
  ```

#### S3.4.2 — Derive `source_refs` from `file_index.json`

- After parsing the LLM JSON response, for each entry in `key_functions`:
  - Look up `ref` in `store.FileIndex` by matching `FunctionDecl.Name` within the module's files
  - If found, rewrite `ref` as `"{path}#L{LineStart}"` from `file_index.json` (ground truth overrides LLM output)
  - If not found in `file_index`, keep the LLM-provided ref and log `WARN "hallucinated ref: {ref}"`

#### S3.4.3 — Implement `RunPreprocessor`

- Implement `RunPreprocessor(ctx context.Context, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, cfg *config.Config, llm llm.Client) (store.SharedContext, error)`:
  1. Compute topological order of shared modules (S3.3)
  2. If no shared modules → return empty `SharedContext`, no LLM calls
  3. For each module in order:
     - Build skeleton for this module's files
     - Build prompt with already-accumulated `SharedContext`
     - Call LLM, parse JSON response into `store.SharedSummary`
     - Derive `source_refs` (S3.4.2)
     - Append to accumulating `SharedContext`
  4. Write completed `SharedContext` via `store.WriteSharedContext`
  5. Return completed `SharedContext`

#### S3.4.4 — Preprocessor unit tests

- Test: `testdata/sample_repo/` with `logger` and `errors` as shared modules → both appear in `shared_context.json`
- Test: prompt for `logger` (which depends on `errors`) includes `errors` summary in the context block
- Test: `key_functions[].ref` values are cross-checked against `file_index.json`; LLM-hallucinated refs produce `WARN` log
- Test: LLM returns invalid JSON → error propagated, `shared_context.json` not written
- Test: no shared modules in `nav_plan.json` → `RunPreprocessor` returns empty context, 0 LLM calls (verified via `MockClient.CallCount()`)

# wikismit — Task Breakdown

**Version:** 0.1.0  
**Status:** Draft  
**Last updated:** 2026-03-18  
**Ref:** wikismit-tech-spec.md

---

## Overview

This document breaks down the wikismit implementation into Stories organised by Epic. Each Story is a self-contained unit of work with a clear completion criterion — something that can be verified without reading the code.

**Execution strategy:** Vertical slice first. Epic 1–6 cut through the full pipeline using Go only and happy-path logic, producing a working end-to-end tool as early as possible. Epic 7 systematically fills in what was deliberately deferred. Epic 8 adds quality assurance on top of the working tool.

**Story status values:** `todo` · `in-progress` · `done` · `blocked`

---

## Epic Map

| Epic | Description | Depends on | Stories |
|---|---|---|---|
| Epic 1 | Project scaffold + LLM client | — | 5 |
| Epic 2 | Phase 1 — AST analysis (Go) | Epic 1 | 4 |
| Epic 3 | Phase 2+3 — Planner + shared preprocessor | Epic 2 | 4 |
| Epic 4 | Phase 4 — Agent fan-out | Epic 3 | 3 |
| Epic 5 | Phase 5 — Doc composer + VitePress | Epic 4 | 4 |
| Epic 6 | Incremental update + CI examples | Epic 5 | 4 |
| Epic 7A | Multi-language AST extension | Epic 2 | 8 |
| Epic 7B | LLM client hardening | Epic 1 | 4 |
| Epic 7C | Planner stability | Epic 3 | 4 |
| Epic 7D | Agent quality | Epic 4 | 4 |
| Epic 7E | Composer completeness | Epic 5 | 4 |
| Epic 7F | Incremental update robustness | Epic 6 | 4 |
| Epic 7G | Full-pipeline integration tests | Epic 7A–7F | 1 |
| Epic 8 | Documentation quality evaluation | Epic 5 | 5 |

---

## Epic 1 — Project Scaffold + LLM Client

**Goal:** A runnable CLI binary that can read config and make a real LLM call. All subsequent Epics build on this foundation.

**Spec refs:** §6 Directory Structure, §8 LLM Integration, §10 CLI Design, §11 Configuration

---

### S1.1 — Project skeleton and CLI entry point

**Status:** `todo`

**Description:**  
Initialise the Go module, directory structure, and the Cobra CLI. Implement config loading from `config.yaml` with env var override for `api_key_env`. The binary must respond to `--help` and load config without panicking.

**Acceptance criteria:**
- `go build ./cmd/wikismit` produces a binary with no errors
- `wikismit --help` lists all subcommands (`generate`, `update`, `plan`, `validate`, `build`)
- `wikismit generate --config ./config.yaml` reads and prints the resolved config (dry-run mode, no actual work)
- Missing required config fields produce a clear error message, not a panic
- `config.yaml` structure matches §11 of the tech spec

**Files to create:**
```
cmd/wikismit/main.go
internal/config/config.go
config.yaml          (template with all fields and comments)
go.mod
.gitignore
```

---

### S1.2 — LLM client: basic completion

**Status:** `todo`

**Description:**  
Implement `internal/llm/client.go` wrapping `go-openai`. The `Client` interface exposes a single `Complete(ctx, CompletionRequest) (string, error)` method. Base URL, model, and timeout must be configurable. The client must work with any OpenAI-compatible endpoint.

**Acceptance criteria:**
- `Client` interface is defined and satisfied by `openAIClient`
- A manual smoke test (`go run`) can send a message to a real OpenAI endpoint and print the reply
- Base URL is read from config; switching to `http://localhost:11434/v1` (Ollama) requires only a config change
- API key is read from the env var named in `api_key_env`, never hardcoded

**Files to create:**
```
internal/llm/client.go
internal/llm/types.go
```

---

### S1.3 — LLM client: retry with exponential backoff

**Status:** `todo`

**Description:**  
Implement `internal/llm/retry.go`. Wrap the base client with retry logic: exponential backoff with jitter for `429`, `500`, `503`, and network timeouts. Fail immediately on `400` and `401`. Max 3 retries, initial backoff 2s, max backoff 30s.

**Acceptance criteria:**
- A `429` response triggers a retry after ~2s, ~4s, ~8s (with jitter), then fails
- A `401` response fails immediately without retry
- Retry attempts are logged at `DEBUG` level with the attempt number and wait duration
- All retry behaviour is exercised by unit tests using a mock HTTP server, no real API calls required

**Files to create:**
```
internal/llm/retry.go
internal/llm/retry_test.go
```

---

### S1.4 — Mock LLM client

**Status:** `todo`

**Description:**  
Implement a `MockClient` in `internal/llm` that satisfies the `Client` interface and returns pre-configured responses. It must support per-call response sequences (first call returns X, second returns Y) and call recording for assertion in tests.

**Acceptance criteria:**
- `MockClient` can be configured with a slice of responses: `[]string{"resp1", "resp2"}`
- `MockClient.Calls()` returns a slice of all `CompletionRequest` values received, for assertion
- Configuring fewer responses than calls returns an error on the extra calls
- All subsequent Epics use `MockClient` in their unit tests; no test requires a real API key

**Files to create:**
```
internal/llm/mock.go
internal/llm/mock_test.go
```

---

### S1.5 — Artifact store layer

**Status:** `todo`

**Description:**  
Implement `pkg/store` as the single read/write interface for all JSON artifacts (`file_index.json`, `dep_graph.json`, `nav_plan.json`, `shared_context.json`). All pipeline phases must use this package for artifact IO; no phase should handle file serialisation directly.

**Acceptance criteria:**
- `store.WriteFileIndex(dir, index)` and `store.ReadFileIndex(dir)` round-trip correctly (write then read returns identical struct)
- Same pattern for `DepGraph`, `NavPlan`, `SharedContext`
- Writing is atomic: write to a temp file then rename, so a partial write never leaves a corrupt artifact
- `pkg/store` has 100% unit test coverage for read/write/round-trip

**Files to create:**
```
pkg/store/index.go
pkg/store/artifacts.go
pkg/store/store_test.go
```

---

## Epic 2 — Phase 1: AST Analysis (Go)

**Goal:** Given a Go repository path, produce `file_index.json` and `dep_graph.json` with correct symbol extraction and `file:line` references preserved.

**Spec refs:** §4 Phase 1, §5 Artifact Schemas, §7 Key Interfaces

---

### S2.1 — tree-sitter Go parser: symbol extraction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/golang.go`. Use `go-tree-sitter` to parse a single `.go` file and return a `FileEntry` containing all exported and unexported function declarations (with `line_start`/`line_end`), type declarations (`struct`, `interface`), and import paths. Compute `content_hash` as `sha256` of the file bytes.

**Acceptance criteria:**
- Given `testdata/fixtures/golang/simple.go` (a file with 2 funcs, 1 struct, 2 imports), the returned `FileEntry` matches `testdata/fixtures/golang/simple.golden.json` exactly
- `line_start` and `line_end` are 1-indexed and correct for each symbol
- `exported` is `true` only for identifiers starting with an uppercase letter
- `content_hash` changes when the file content changes, and is stable across repeated calls on the same content

**Files to create:**
```
internal/analyzer/lang/golang.go
internal/analyzer/lang/golang_test.go
testdata/fixtures/golang/simple.go
testdata/fixtures/golang/simple.golden.json
```

---

### S2.2 — Repository file traverser

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/analyzer.go`. Recursively walk the repository path, identify source files by extension, apply `exclude_patterns` from config (glob matching), and call the appropriate language parser for each file. Return a complete `FileIndex`.

**Acceptance criteria:**
- Given `testdata/sample_repo/` (a small synthetic Go project), all `.go` files are parsed and present in the returned `FileIndex`
- Files matching `exclude_patterns` (e.g. `*_test.go`, `vendor/**`) are not present in the output
- Unrecognised file extensions are silently skipped (no error)
- A file that fails to parse logs a warning and is skipped; traversal continues

**Files to create:**
```
internal/analyzer/analyzer.go
internal/analyzer/analyzer_test.go
testdata/sample_repo/          (synthetic Go project, used by all pipeline tests)
```

---

### S2.3 — Dependency graph construction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/dep_graph.go`. From the `FileIndex` produced by S2.2, build a directed adjacency list representing internal import dependencies. An internal import is one whose path starts with the repository's module path (from `go.mod`). External (third-party) imports are recorded in `FileEntry.Imports` with `internal: false` but are not added as graph edges.

**Acceptance criteria:**
- Given `testdata/sample_repo/`, if `internal/auth/jwt.go` imports `pkg/logger`, the dep_graph contains edge `internal/auth/jwt.go → pkg/logger/logger.go`
- Third-party imports (e.g. `github.com/golang-jwt/jwt`) appear in `FileEntry.Imports` with `internal: false` but not as graph edges
- The graph is a valid DAG for the sample repo (no cycles expected in the synthetic fixture)

**Files to create:**
```
internal/analyzer/dep_graph.go
internal/analyzer/dep_graph_test.go
```

---

### S2.4 — Phase 1 orchestration and artifact write

**Status:** `todo`

**Description:**  
Wire S2.1–S2.3 together in Phase 1 orchestration. Implement the `wikismit generate` command's Phase 1 step: run the analyzer, then write `file_index.json` and `dep_graph.json` to the artifacts directory via `pkg/store`. Phase 1 must complete before Phase 2 begins (no concurrency between phases).

**Acceptance criteria:**
- Running `wikismit generate --repo ./testdata/sample_repo` produces `artifacts/file_index.json` and `artifacts/dep_graph.json`
- Both files are valid JSON matching their schemas in §5 of the tech spec
- Running Phase 1 twice on an unchanged repo produces byte-identical artifact files (idempotent)
- Elapsed time and file count are logged at `INFO` level on completion

**Files to create:**
```
internal/analyzer/phase1.go
```

---

## Epic 3 — Phase 2 + 3: Planner + Shared Preprocessor

**Goal:** Given `file_index.json` and `dep_graph.json`, produce `nav_plan.json` (module ownership locked) and `shared_context.json` (shared module summaries ready for agent injection).

**Spec refs:** §4 Phase 2, §4 Phase 3, §5 Artifact Schemas

---

### S3.1 — Skeleton serialiser

**Status:** `todo`

**Description:**  
Implement `internal/planner/skeleton.go`. Generate a token-budget-aware text representation of a module's code structure from `FileIndex`. The skeleton contains function signatures and type names only — no function bodies. If the skeleton exceeds `skeleton_max_tokens`, truncate by prioritising exported symbols over unexported ones.

**Acceptance criteria:**
- Given `testdata/sample_repo/` `FileIndex`, the skeleton for the `auth` module contains all exported function signatures and type names
- Skeleton never exceeds `skeleton_max_tokens` (verified by token count estimation, not character count)
- When truncation occurs, unexported symbols are dropped before exported ones
- Skeleton output includes `file:line` annotations for each symbol in the format `// file.go:24`

**Files to create:**
```
internal/planner/skeleton.go
internal/planner/skeleton_test.go
```

---

### S3.2 — Phase 2: module planner LLM call

**Status:** `todo`

**Description:**  
Implement `internal/planner/planner.go`. Build the planner prompt from the full repository skeleton, call the LLM once, and parse the JSON response into a `NavPlan`. If the response is not valid JSON, retry with a prompt that includes the parse error. Fail after 3 unsuccessful attempts. Write the result via `pkg/store`.

**Acceptance criteria:**
- Given `testdata/sample_repo/` artifacts, `nav_plan.json` is produced with every file assigned to exactly one module
- Modules with 3+ importing files are assigned `shared: true` and `owner: "shared_preprocessor"`
- JSON parse failure triggers a retry with the error message appended to the prompt; the `MockClient` test simulates one bad response followed by a valid one
- The `wikismit plan` subcommand runs Phase 1 + Phase 2 only and exits, printing the path to `nav_plan.json`

**Files to create:**
```
internal/planner/planner.go
internal/planner/planner_test.go
```

---

### S3.3 — Shared module topological sort

**Status:** `todo`

**Description:**  
Implement topological sorting of the shared module subgraph in `internal/preprocessor/preprocessor.go`. Extract all modules with `owner: "shared_preprocessor"` from `nav_plan.json`, build their dependency subgraph from `dep_graph.json`, and produce a processing order where a shared module is always processed before the shared modules that depend on it.

**Acceptance criteria:**
- If `pkg/errors` is imported by `pkg/logger`, and both are shared, `pkg/errors` appears before `pkg/logger` in the sort output
- Cycles in the shared subgraph (should not exist in valid Go, but may in other languages) produce a clear error and halt Phase 3
- A repo with no shared modules produces an empty sort output and skips Phase 3 without error

**Files to create:**
```
internal/preprocessor/preprocessor.go
internal/preprocessor/topo_test.go
```

---

### S3.4 — Phase 3: shared module serial LLM calls

**Status:** `todo`

**Description:**  
Complete `internal/preprocessor/preprocessor.go` and implement `internal/preprocessor/shared_context.go`. Iterate over shared modules in topological order, build a summary prompt for each, call the LLM, and parse the response into a `SharedContext` entry. Each prompt may reference previously generated summaries for that module's own shared dependencies. Write the completed `SharedContext` via `pkg/store`.

**Acceptance criteria:**
- Given `testdata/sample_repo/` with at least one shared module, `shared_context.json` is produced with a non-empty `summary`, `key_types`, `key_functions`, and `source_refs` for each shared module
- The prompt for `pkg/middleware` (which depends on `pkg/logger`) includes `pkg/logger`'s summary from the same run
- `source_refs` values are in `file.go#L{line}` format and reference lines that actually exist in `file_index.json`

**Files to create:**
```
internal/preprocessor/shared_context.go
internal/preprocessor/preprocessor_test.go
```

---

## Epic 4 — Phase 4: Agent Fan-out

**Goal:** Given `nav_plan.json`, `file_index.json`, and `shared_context.json`, run one LLM agent per non-shared module concurrently and write per-module Markdown to `artifacts/module_docs/`.

**Spec refs:** §4 Phase 4, §7 Key Interfaces (AgentInput, ModuleDoc)

---

### S4.1 — Prompt builder with shared context injection

**Status:** `todo`

**Description:**  
Implement `internal/agent/prompt.go`. For a given module, build the LLM prompt from: (1) the module's code skeleton, (2) the relevant subset of `shared_context.json` for its declared `depends_on_shared` list, and (3) the ownership constraint instruction ("do not re-describe shared modules, link only"). The prompt must include explicit `file:line` format instructions for citations.

**Acceptance criteria:**
- For a module with `depends_on_shared: ["logger"]`, the prompt contains `pkg/logger`'s summary from `shared_context.json`
- For a module with no shared dependencies, the shared context block is absent from the prompt
- The prompt contains the instruction prohibiting re-description of shared modules
- Prompt structure is verified by snapshot test using `MockClient`; the test asserts key substrings are present

**Files to create:**
```
internal/agent/prompt.go
internal/agent/prompt_test.go
```

---

### S4.2 — Goroutine scheduler with concurrency control

**Status:** `todo`

**Description:**  
Implement `internal/agent/scheduler.go`. Use a semaphore channel to cap concurrent goroutines at the configured `concurrency` value (default 4). Each goroutine runs one `agent.Run()` call and sends the result to a buffered `chan ModuleDoc`. A collector goroutine drains the channel and writes results to `artifacts/module_docs/{id}.md`.

**Acceptance criteria:**
- Given 8 modules and `concurrency: 2`, no more than 2 goroutines are active simultaneously (verified by a counter in the mock agent)
- All 8 modules complete and produce output files even when some take longer than others
- The scheduler waits for all goroutines before returning; no goroutine leak
- Elapsed time per module and total elapsed time are logged at `INFO` level

**Files to create:**
```
internal/agent/scheduler.go
internal/agent/scheduler_test.go
```

---

### S4.3 — Agent execution and partial failure handling

**Status:** `todo`

**Description:**  
Implement `internal/agent/agent.go`. Each agent builds its prompt (S4.1), calls the LLM, and returns a `ModuleDoc`. If the LLM call fails after retries, `ModuleDoc.Err` is set and the result is still sent to the channel — the scheduler must not cancel other goroutines. After all goroutines complete, the orchestrator reports a summary of failures without failing the process with a non-zero exit code in v1.

**Acceptance criteria:**
- Given 4 modules where 1 agent's LLM call always fails, the other 3 modules produce valid `.md` files
- The failure summary is printed to `stderr` at the end of Phase 4
- The failing module's `.md` file is not written (no partial/empty files)
- `MockClient` test: first call errors, remaining calls succeed; assert 3 files written, 1 failure reported

**Files to create:**
```
internal/agent/agent.go
internal/agent/agent_test.go
```

---

## Epic 5 — Phase 5: Doc Composer + VitePress Output

**Goal:** Given `artifacts/module_docs/`, produce a complete `docs/` directory with injected citations, validated cross-references, TOC, and a `docs/.vitepress/config.ts` ready for `vitepress build`.

**Spec refs:** §4 Phase 5, §16 Documentation Deployment

---

### S5.1 — Citation injector

**Status:** `todo`

**Description:**  
Implement `internal/composer/citation.go`. Scan each `module_docs/*.md` file for function and type name references (inside backticks). For each name found, look it up in `file_index.json`. If a `file:line` link is absent for that name, inject it as a Markdown link in the format `[Name](path/to/file.go#L{line})`. Names not found in `file_index.json` are left unchanged.

**Acceptance criteria:**
- `` `GenerateToken` `` in a module doc becomes `[GenerateToken](internal/auth/jwt.go#L24)` when `GenerateToken` is at line 24 in `file_index.json`
- A name that is already a Markdown link is not double-linked
- A name not present in `file_index.json` is left as-is (no error, no modification)
- Table-driven unit tests cover: name found, name already linked, name not in index, name appears multiple times

**Files to create:**
```
internal/composer/citation.go
internal/composer/citation_test.go
```

---

### S5.2 — Cross-reference validator

**Status:** `todo`

**Description:**  
Implement `internal/composer/validator.go`. Scan all output Markdown files for links pointing to `../shared/{id}.md` or `../modules/{id}.md`. Verify each target file exists in the `docs/` output directory. Collect all broken links into a `ValidationReport` and log them as warnings. The `wikismit validate` subcommand runs Phase 5 validation only and prints the report.

**Acceptance criteria:**
- A link to `../shared/logger.md` that does not exist in `docs/shared/` appears in the `ValidationReport`
- A link to a file that does exist produces no warning
- `wikismit validate` exits with code 0 even when warnings are present (non-blocking in v1)
- The validation report is also written to `artifacts/validation_report.json`

**Files to create:**
```
internal/composer/validator.go
internal/composer/validator_test.go
```

---

### S5.3 — Markdown renderer and TOC generation

**Status:** `todo`

**Description:**  
Implement `internal/composer/renderer.go`. Assemble the final `docs/` directory from `artifacts/module_docs/`, inject a table of contents at the top of each file, and write `docs/index.md` (landing page with module tree derived from `nav_plan.json`). Run citation injection (S5.1) and cross-reference validation (S5.2) as part of this step.

**Acceptance criteria:**
- After running Phase 5 on `testdata/sample_repo/` artifacts, `docs/` contains `index.md`, `modules/*.md`, and `shared/*.md`
- Every output file has a `## Contents` section with anchor links at the top
- `docs/index.md` lists all modules and shared modules with links, ordered by dependency depth (leaf modules first)
- File count and output directory path are logged at `INFO` level on completion

**Files to create:**
```
internal/composer/renderer.go
internal/composer/renderer_test.go
```

---

### S5.4 — VitePress config generator

**Status:** `todo`

**Description:**  
Implement `internal/composer/vitepress.go`. Generate `docs/.vitepress/config.ts` from `nav_plan.json` and site configuration fields (`site.title`, `site.repo_url`, `site.logo`) from `config.yaml`. Sidebar groups: "Modules" (non-shared) and "Shared" (shared modules). Sidebar order matches the dependency-depth order from `dep_graph.json`.

**Acceptance criteria:**
- Generated `config.ts` is valid TypeScript that can be consumed by `vitepress build` without modification
- Sidebar contains one entry per module, with `link` values matching the actual output file paths
- `site.title`, `site.repo_url` from `config.yaml` are reflected in the generated config
- If `site.*` fields are absent from config, sensible defaults are used (repo directory name as title)

**Files to create:**
```
internal/composer/vitepress.go
internal/composer/vitepress_test.go
```

---

## Epic 6 — Incremental Update + CI Examples

**Goal:** `wikismit update` re-runs only the pipeline phases needed for changed files, producing output identical to a full run for the changed modules.

**Spec refs:** §9 Incremental Update Mode, §16 CI integration examples

---

### S6.1 — Git diff parser

**Status:** `todo`

**Description:**  
Implement `pkg/gitdiff/diff.go` using `go-git`. Given a repository path and optional `--base-ref` / `--head-ref` flags (defaulting to `HEAD~1` and `HEAD`), return the set of modified file paths. Also support `--changed-files` flag to bypass git entirely (for custom CI triggers).

**Acceptance criteria:**
- Given a repo with a known commit that modified `internal/auth/jwt.go`, the function returns `{"internal/auth/jwt.go"}`
- `--changed-files=a.go,b.go` bypasses git and returns `{"a.go", "b.go"}`
- A clean repo (no diff) returns an empty set without error
- Paths are returned relative to the repo root

**Files to create:**
```
pkg/gitdiff/diff.go
pkg/gitdiff/diff_test.go
```

---

### S6.2 — Affected module computation

**Status:** `todo`

**Description:**  
Implement affected module resolution in `internal/analyzer/dep_graph.go`. Given the set of changed files and the `dep_graph.json`, compute: (1) the modules that directly own the changed files, and (2) all upstream modules that transitively depend on those modules. The union is the set of modules that must be re-run in Phase 3/4.

**Acceptance criteria:**
- Changing `pkg/logger/logger.go` marks `logger` (shared) and all modules that depend on it as affected
- Changing a file in a leaf module marks only that module as affected
- Affected set is a subset of all modules in `nav_plan.json`; no module outside the set is re-run

**Files to create:**
```
internal/analyzer/affected.go
internal/analyzer/affected_test.go
```

---

### S6.3 — Incremental pipeline orchestration

**Status:** `todo`

**Description:**  
Implement the `wikismit update` command orchestration. Load existing artifacts from disk, compute the affected module set (S6.2), re-run Phase 3 serially for any affected shared modules (in topological order), re-run Phase 4 for affected non-shared modules, then run Phase 5 in full. Unchanged modules reuse their existing `module_docs/*.md` files.

**Acceptance criteria:**
- On a repo where only 1 of 8 modules changed, exactly 1 LLM call is made in Phase 4 (verified via `MockClient.Calls()`)
- If an affected module is shared, Phase 3 re-runs for it before Phase 4 runs for its dependents
- Phase 5 always runs in full (cross-reference validation must reflect the current state of all docs)
- If no artifacts exist yet, `wikismit update` falls back to `wikismit generate` with a warning

**Files to create:**
```
cmd/wikismit/update.go
```

---

### S6.4 — CI example files

**Status:** `todo`

**Description:**  
Write the example GitHub Actions workflow files and README in `examples/github/`. The workflows should use the final CLI interface from S1.1 and S6.3. The README must be self-contained: a new user should be able to follow it to get docs deployed to GitHub Pages without reading anything else.

**Acceptance criteria:**
- `docs-full.yml`: triggers on push to `main`, runs `wikismit generate`, runs `vitepress build`, deploys `dist/` to GitHub Pages
- `docs-incremental.yml`: two-job pattern (generate + deploy), uses `actions/cache` for `artifacts/`, runs `wikismit update`
- `README.md` covers: required secrets, enabling GitHub Pages, Cloudflare Pages alternative (swap last 2 steps)
- Both YAML files are valid and pass `actionlint` (or equivalent syntax check)

**Files to create:**
```
examples/github/docs-full.yml
examples/github/docs-incremental.yml
examples/github/README.md
```

---

## Epic 7A — Multi-language AST Extension

**Goal:** Extend the Phase 1 analyzer to support Python, TypeScript, Rust, and Java with the same `FileEntry` quality as the Go parser.

**Spec refs:** §4 Phase 1 (supported languages table)

---

### S7A.1 — `LanguageParser` interface abstraction

**Status:** `todo`

**Description:**  
Refactor `internal/analyzer` to define a `LanguageParser` interface with a single method `ExtractSymbols(path string, src []byte) (FileEntry, error)`. Move the existing Go implementation to satisfy this interface. The file traverser (S2.2) dispatches to the registered parser for each file extension.

**Acceptance criteria:**
- Existing Go parser tests pass without modification after the refactor
- New language parsers can be registered by adding one line to a registry map; no changes to the traverser required
- An unregistered file extension is skipped silently (existing behaviour preserved)

**Files to create / modify:**
```
internal/analyzer/parser.go       (new interface + registry)
internal/analyzer/lang/golang.go  (implement interface)
```

---

### S7A.2 — Python parser: symbol extraction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/python.go`. Extract `def` (functions and methods), `class` definitions, and `import`/`from...import` statements. Python has no explicit type annotations required; `exported` is `true` for names not starting with `_`.

**Acceptance criteria:**
- Given `testdata/fixtures/python/simple.py`, output matches `simple.golden.json`
- Class methods are recorded as functions with `class_name.method_name` naming
- `__init__`, `__str__` and other dunder methods are included with `exported: false`

**Files to create:**
```
internal/analyzer/lang/python.go
internal/analyzer/lang/python_test.go
testdata/fixtures/python/simple.py
testdata/fixtures/python/simple.golden.json
```

---

### S7A.3 — Python import path resolution

**Status:** `todo`

**Description:**  
Resolve Python imports to internal file paths. Absolute imports (`import mypackage.auth`) are resolved against the repo root. Relative imports (`from . import utils`, `from ..core import base`) are resolved relative to the importing file's package. Third-party imports are marked `internal: false`.

**Acceptance criteria:**
- `from . import utils` in `mypackage/auth/jwt.py` resolves to `mypackage/auth/utils.py` if it exists
- `import mypackage.core` resolves to `mypackage/core/__init__.py` or `mypackage/core.py`
- `import requests` (third-party) is marked `internal: false` and not added as a dep_graph edge

**Files to create / modify:**
```
internal/analyzer/lang/python.go
internal/analyzer/lang/python_test.go
```

---

### S7A.4 — TypeScript parser: symbol extraction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/typescript.go`. Extract `function`, `class`, `interface`, `type` declarations, and `export` modifiers. Handle both `.ts` and `.tsx` extensions. `exported` is `true` for declarations with the `export` keyword.

**Acceptance criteria:**
- Given `testdata/fixtures/typescript/simple.ts`, output matches `simple.golden.json`
- `export default function` and `export const fn = () =>` arrow functions are both captured
- JSX in `.tsx` files does not cause parse errors

**Files to create:**
```
internal/analyzer/lang/typescript.go
internal/analyzer/lang/typescript_test.go
testdata/fixtures/typescript/simple.ts
testdata/fixtures/typescript/simple.golden.json
```

---

### S7A.5 — TypeScript import path resolution

**Status:** `todo`

**Description:**  
Resolve TypeScript `import from` paths to internal file paths. Handle relative paths (`./utils`, `../auth`), `index.ts` resolution, and `tsconfig.json` path aliases (`@/utils` → `src/utils`). Third-party imports (`react`, `lodash`) are marked `internal: false`.

**Acceptance criteria:**
- `import { Logger } from '../utils/logger'` resolves to the correct `.ts` file
- `import { fn } from '@/core/base'` resolves correctly when `tsconfig.json` defines `@` → `src/`
- If no `tsconfig.json` is present, path alias resolution is skipped without error

**Files to create / modify:**
```
internal/analyzer/lang/typescript.go
internal/analyzer/lang/typescript_test.go
```

---

### S7A.6 — Rust parser: symbol extraction

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/rust.go`. Extract `pub fn`, `pub struct`, `pub trait`, `pub enum`, and `impl` blocks. Distinguish `pub`, `pub(crate)`, and private visibility. Map `mod` declarations to internal file paths.

**Acceptance criteria:**
- Given `testdata/fixtures/rust/simple.rs`, output matches `simple.golden.json`
- `pub(crate) fn` is recorded with `exported: false` (not public outside the crate)
- `impl Trait for Type` methods are recorded under `Type::method_name`

**Files to create:**
```
internal/analyzer/lang/rust.go
internal/analyzer/lang/rust_test.go
testdata/fixtures/rust/simple.rs
testdata/fixtures/rust/simple.golden.json
```

---

### S7A.7 — Java parser: symbol extraction and import resolution

**Status:** `todo`

**Description:**  
Implement `internal/analyzer/lang/java.go`. Extract `class`, `interface`, `enum`, and `method` declarations. Resolve `import com.example.auth.AuthService` to internal file paths by mapping the package path to a directory structure under the repo root.

**Acceptance criteria:**
- Given `testdata/fixtures/java/simple.java`, output matches `simple.golden.json`
- `import com.example.auth.AuthService` resolves to `src/main/java/com/example/auth/AuthService.java` if it exists
- `import java.util.List` (stdlib) is marked `internal: false`

**Files to create:**
```
internal/analyzer/lang/java.go
internal/analyzer/lang/java_test.go
testdata/fixtures/java/simple.java
testdata/fixtures/java/simple.golden.json
```

---

### S7A.8 — Multi-language golden file test suite

**Status:** `todo`

**Description:**  
Create a comprehensive golden file test for all supported languages, plus an extended `testdata/sample_repo/` fixture that includes Python and TypeScript files alongside Go. Run the full Phase 1 pipeline against the multi-language fixture and assert the combined `file_index.json` output matches a committed golden file.

**Acceptance criteria:**
- `testdata/fixtures/` contains at least one fixture per language (Go, Python, TypeScript, Rust, Java)
- All golden file tests pass in CI without a real API key
- The multi-language `sample_repo/` produces a `file_index.json` with entries for all languages
- `go test ./internal/analyzer/...` is the single command that runs all language tests

---

## Epic 7B — LLM Client Hardening

**Goal:** The LLM client is reliable enough for production use: cached, token-budget-aware, and verified against multiple real endpoints.

**Spec refs:** §8 LLM Integration (caching, model assignment, token budget)

---

### S7B.1 — LLM response cache

**Status:** `todo`

**Description:**  
Implement response caching in `internal/llm/client.go`. Cache key is `sha256(model + prompt)`. On a cache hit, return the stored response without making a network call. Write cache entries to `artifacts/cache/` as individual JSON files. Cache is bypassed when `--no-cache` flag is set.

**Acceptance criteria:**
- Second call with identical model + prompt returns cached response; `MockClient` records only 1 call
- Cache files are written to `artifacts/cache/{hash}.json`
- `--no-cache` flag causes every call to go to the API regardless of cache state
- Cache files from a previous run survive a process restart and are used on the next run

**Files to create / modify:**
```
internal/llm/cache.go
internal/llm/client.go  (wrap with cache layer)
```

---

### S7B.2 — Cache invalidation on content change

**Status:** `todo`

**Description:**  
Tie LLM cache invalidation to `content_hash` changes in `file_index.json`. When a module's source files change (detected by comparing `content_hash` values between runs), invalidate the cache entries for all prompts derived from those files. This ensures stale docs are never served from cache after a code change.

**Acceptance criteria:**
- Modifying a source file updates its `content_hash`; the next `wikismit update` call re-calls the LLM for that module
- Unmodified modules still hit the cache
- Cache invalidation logic is tested with a two-run sequence: run 1 populates cache, file is modified, run 2 bypasses cache for the changed module only

---

### S7B.3 — Token budget enforcement

**Status:** `todo`

**Description:**  
Before making an LLM call, estimate the prompt token count using a character-based approximation (1 token ≈ 4 characters). If the estimated token count plus `max_tokens` exceeds the model's context limit (configurable per model in `config.yaml`), truncate the skeleton section of the prompt until it fits. Log a warning when truncation occurs.

**Acceptance criteria:**
- A module with 200 files does not trigger a `400 context_length_exceeded` error
- When truncation occurs, a `WARN` log line identifies the module and the number of tokens dropped
- Truncation always preserves the system instructions and shared context; only the code skeleton is truncated

**Files to create / modify:**
```
internal/llm/budget.go
internal/planner/skeleton.go  (expose token count method)
```

---

### S7B.4 — Multi-endpoint integration validation

**Status:** `todo`

**Description:**  
Write a manual integration test (gated behind `//go:build integration`) that sends one Phase 2 planner call to each of three endpoints: OpenAI, Anthropic via proxy, and Ollama. Verify that the response can be parsed into a valid `NavPlan`. Document the required environment variables in `examples/github/README.md`.

**Acceptance criteria:**
- Test passes against all three endpoints when the appropriate env vars are set
- Test is skipped (not failed) when env vars are absent
- README documents which env vars are needed for each provider

---

## Epic 7C — Planner Stability

**Goal:** Phase 2's `nav_plan.json` output is structurally correct on the first attempt, even for large or unusual repositories.

**Spec refs:** §4 Phase 2, §12 Error Handling (JSON parse failures)

---

### S7C.1 — Structured output for Phase 2

**Status:** `todo`

**Description:**  
Replace the current text-parsing + JSON extraction approach in Phase 2 with OpenAI function calling / JSON mode. Define the `NavPlan` schema as an OpenAI tool schema and request the LLM to call it. This eliminates JSON parse failures caused by the LLM adding preamble text or markdown fences around the JSON.

**Acceptance criteria:**
- Phase 2 JSON parse failure rate drops to 0 in unit tests using `MockClient` with a variety of outputs
- The function calling schema matches the `NavPlan` struct in `pkg/store/artifacts.go`
- Falls back to text-mode parsing if the model does not support function calling (Ollama), with a `WARN` log

---

### S7C.2 — README and package comment injection

**Status:** `todo`

**Description:**  
Extend the skeleton builder (S3.1) to prepend the content of `README.md` and package-level doc comments to the planner prompt. This gives the LLM semantic context about the project's design intent, improving module grouping quality for repositories where directory structure does not match conceptual boundaries.

**Acceptance criteria:**
- If `README.md` exists at the repo root, the first 500 tokens are prepended to the planner prompt
- Package-level `// Package auth provides...` comments are extracted and included in the skeleton
- Total prompt still respects the token budget (README content is the first thing truncated if needed)

---

### S7C.3 — Monorepo support

**Status:** `todo`

**Description:**  
Detect monorepo layouts by finding multiple `go.mod` or `package.json` files. Generate a separate `NavPlan` for each sub-project and store them as `nav_plan_{subproject}.json`. Phase 4 agents are scoped to their sub-project's plan; cross-sub-project imports are treated as external.

**Acceptance criteria:**
- A repo containing `backend/go.mod` and `frontend/package.json` produces two separate nav plans
- An agent for the `backend` sub-project does not receive skeleton content from `frontend/`
- `docs/` output has a subdirectory per sub-project: `docs/backend/`, `docs/frontend/`

---

### S7C.4 — Planner edge case handling

**Status:** `todo`

**Description:**  
Handle degenerate input cases in the planner: a single-file repository, a repository where all files qualify as shared, and a repository with zero Go/supported files. Each case must produce a useful warning rather than a panic or a silent empty output.

**Acceptance criteria:**
- Single-file repo: produces a `nav_plan.json` with one module containing one file, no shared modules
- All-shared repo: all modules have `owner: "shared_preprocessor"`, Phase 4 produces no output, Phase 5 produces only `docs/shared/`
- No supported files: `wikismit generate` exits with a clear error message before making any LLM calls

---

## Epic 7D — Agent Quality

**Goal:** Phase 4 agents produce consistently well-structured documentation and do not exceed API rate limits on large repositories.

**Spec refs:** §4 Phase 4, §12 Error Handling

---

### S7D.1 — Oversized module truncation

**Status:** `todo`

**Description:**  
Handle modules with too many files to fit in one prompt. When a module's skeleton exceeds `skeleton_max_tokens`, split it into a primary prompt (exported symbols only) and record the omitted symbols in a `truncation_log`. The agent generates docs for exported symbols; the truncation log is included as a "not documented" section at the end of the output file.

**Acceptance criteria:**
- A module with 50 files does not trigger a context overflow error
- The output `.md` file has a trailing section listing the unexported symbols that were omitted
- No exported symbol is silently omitted; only unexported symbols are dropped in truncation

---

### S7D.2 — Tiered prompt templates

**Status:** `todo`

**Description:**  
Differentiate prompt templates by module importance. "Core" modules (in-degree ≥ 5 in `dep_graph.json`) receive a detailed prompt requesting usage examples, design rationale, and common pitfalls. "Leaf" modules (in-degree 0) receive a concise prompt requesting a brief summary and function list only.

**Acceptance criteria:**
- Core modules' prompts include the instruction to provide usage examples
- Leaf modules' prompts do not include the usage example instruction
- In-degree threshold is configurable in `config.yaml` (`agent.core_module_threshold`, default 5)

---

### S7D.3 — LLM output validation and retry

**Status:** `todo`

**Description:**  
After receiving an LLM response, validate that it contains at least one H2 Markdown heading (`## `). If validation fails (empty response, plain code block, JSON), retry once with an appended instruction: "Your previous response was not valid Markdown. Please respond with a structured Markdown document." If the retry also fails validation, write a placeholder file and log an error.

**Acceptance criteria:**
- An empty LLM response triggers one retry
- A response starting with `{` (JSON) triggers one retry
- A response with a valid H2 heading is accepted without retry
- The placeholder file content is: `# {module_id}\n\n> Documentation generation failed. Please re-run wikismit generate.\n`

---

### S7D.4 — Per-minute rate limit throttling

**Status:** `todo`

**Description:**  
Track cumulative estimated token usage across Phase 4 goroutines. When the projected usage for the next minute exceeds 80% of the configured `rate_limit_tpm` (tokens per minute, from `config.yaml`), introduce a sleep before launching the next goroutine. This prevents `429` errors on large repositories without relying on retry as the primary rate control.

**Acceptance criteria:**
- A simulated run of 40 modules each using 5,000 tokens against a 100,000 TPM limit does not produce any `429` errors in unit tests
- Sleep durations are logged at `DEBUG` level
- `rate_limit_tpm: 0` disables throttling entirely (opt-out for users with high-tier API accounts)

---

## Epic 7E — Composer Completeness

**Goal:** The generated `docs/` site is self-contained, navigable, and visually accurate for the repository's structure.

**Spec refs:** §4 Phase 5, §16 VitePress deployment

---

### S7E.1 — Per-module Mermaid dependency diagram

**Status:** `todo`

**Description:**  
In `internal/composer/renderer.go`, generate a `graph TD` Mermaid diagram for each module showing its direct imports and direct importers. Inject the diagram at the bottom of each module's output file under a `## Dependencies` heading.

**Acceptance criteria:**
- Every `docs/modules/{id}.md` file has a `## Dependencies` section with a fenced ` ```mermaid ` block
- The diagram contains edges only for direct (one-hop) dependencies, not transitive ones
- Shared modules are styled differently in the diagram (e.g. `:::shared` class)

---

### S7E.2 — Global dependency diagram on index page

**Status:** `todo`

**Description:**  
Generate a project-level `graph TD` Mermaid diagram in `docs/index.md` showing all modules and their inter-dependencies. Shared modules use a visually distinct node style. For repositories with more than 20 modules, generate a simplified top-level diagram showing only shared modules and the clusters that depend on them.

**Acceptance criteria:**
- `docs/index.md` contains a fenced Mermaid block with all modules represented
- For repos with ≤ 20 modules, all edges are shown
- For repos with > 20 modules, only shared module edges are shown, with a note indicating the simplified view

---

### S7E.3 — Citation coverage metric

**Status:** `todo`

**Description:**  
After citation injection (S5.1), compute per-module citation coverage: `(functions with file:line link) / (total exported functions in file_index)`. Write coverage per module to `artifacts/validation_report.json`. Add a `--min-citation-coverage` flag to `wikismit validate` that exits with code 1 when any module falls below the threshold.

**Acceptance criteria:**
- `validation_report.json` contains a `citation_coverage` map: `{module_id: float}`
- `wikismit validate --min-citation-coverage=0.8` exits 1 when any module is below 80%
- Coverage of 1.0 is achievable on `testdata/sample_repo/` (all exported functions are referenced in the generated docs)

---

### S7E.4 — VitePress site customisation

**Status:** `todo`

**Description:**  
Extend the VitePress config generator (S5.4) to support additional site customisation fields from `config.yaml`: `site.logo` (path to an image file copied to `docs/public/`), `site.repo_url` (adds an "Edit on GitHub" link per page), and `site.nav` (additional top-level nav items as key-value pairs).

**Acceptance criteria:**
- `site.repo_url` set in `config.yaml` appears as `editLink.pattern` in the generated `config.ts`
- `site.logo` path is copied to `docs/public/logo.png` and referenced in `config.ts`
- `site.nav` items appear in the top navigation bar alongside the default "Modules" and "Shared" entries

---

## Epic 7F — Incremental Update Robustness

**Goal:** `wikismit update` handles all real-world file change scenarios without producing stale or inconsistent documentation.

**Spec refs:** §9 Incremental Update Mode

---

### S7F.1 — File rename handling

**Status:** `todo`

**Description:**  
Detect file renames in `pkg/gitdiff/diff.go` (git reports these as `R{score}` diff entries). When a rename is detected, update `file_index.json` to move the entry from the old path to the new path, delete `artifacts/module_docs/{old_module}.md` if the file's module changes, and add the new path to the affected module set.

**Acceptance criteria:**
- Renaming `pkg/util.go` to `pkg/helper.go` results in `file_index.json` having `pkg/helper.go` and no `pkg/util.go`
- The old module doc is removed from `docs/`; the new doc is generated
- If rename is within the same module, only one LLM call is made for that module

---

### S7F.2 — File deletion handling

**Status:** `todo`

**Description:**  
When a file is deleted, remove its entry from `file_index.json`, remove the corresponding module doc from `artifacts/module_docs/` if the module is now empty, and run cross-reference validation (S5.2) to surface broken links in documents that referenced the deleted module.

**Acceptance criteria:**
- After deleting `internal/auth/jwt.go`, `file_index.json` has no entry for that path
- If `auth` module is now empty, `docs/modules/auth.md` is deleted
- Any doc that linked to `auth` produces a broken link warning in `validation_report.json`

---

### S7F.3 — NavPlan structural change detection

**Status:** `todo`

**Description:**  
Before running an incremental update, compare the new `nav_plan.json` candidate (derived from the updated `file_index`) against the existing one on disk. If module boundaries have changed (modules added, removed, or files reassigned), trigger a full re-run of Phase 2 onwards rather than an incremental update, with a `WARN` log explaining why.

**Acceptance criteria:**
- Adding a new file that creates a new module triggers a full re-run from Phase 2
- Changing a file within an existing module (no structural change) does not trigger a full re-run
- The warning message names the specific structural change detected (e.g. "new module detected: payments")

---

### S7F.4 — Incremental update end-to-end test

**Status:** `todo`

**Description:**  
Create a two-snapshot test fixture: `testdata/sample_repo_v1/` and `testdata/sample_repo_v2/` where v2 has one modified file, one renamed file, and one new file. Run `wikismit generate` on v1, then `wikismit update` simulating the v1→v2 diff, then `wikismit generate` fresh on v2. Assert that the `docs/` output of the update run is identical to the fresh generate output.

**Acceptance criteria:**
- `docs/` from incremental update matches `docs/` from full generate on v2, file-for-file
- LLM call count for the incremental run is less than for the full generate run
- Test uses `MockClient` and requires no real API key

---

## Epic 7G — Full-pipeline Integration Test

**Goal:** A single test command that exercises the entire pipeline against a realistic multi-language repository and gates on golden output.

**Spec refs:** §13 Testing Strategy

---

### S7G.1 — End-to-end golden file integration test

**Status:** `todo`

**Description:**  
Create `testdata/sample_repo/` as a synthetic multi-language Go+Python+TypeScript repository with a realistic structure (3–4 modules, 1–2 shared modules, cross-language imports at the boundary). Run the full pipeline (`generate` → `validate`) using `MockClient` with pre-recorded responses. Diff the output `docs/` against a committed golden directory. Gate this test in CI behind `//go:build integration`.

**Acceptance criteria:**
- `go test -tags=integration ./...` runs the full pipeline against `testdata/sample_repo/`
- Output `docs/` matches `testdata/golden_docs/` exactly (byte-identical Markdown files)
- Test fails with a clear diff when any output file deviates from golden
- `validation_report.json` from the run has zero broken links and citation coverage ≥ 0.9

---

## Epic 8 — Documentation Quality Evaluation

**Goal:** `wikismit` can report on the accuracy and completeness of its own output, and detect quality regressions when prompts change.

---

### S8.1 — Symbol coverage report

**Status:** `todo`

**Description:**  
Implement symbol coverage analysis in `internal/composer/validator.go`. Compare the set of exported functions and types in `file_index.json` against the set of symbol names appearing in the generated `docs/`. Report missing symbols (exported but not mentioned in docs) per module in `artifacts/validation_report.json`.

**Acceptance criteria:**
- `validation_report.json` contains `missing_symbols: [{module, symbol}]` for every exported symbol absent from the corresponding module doc
- `wikismit validate` prints a summary: "X of Y exported symbols documented"
- A `--min-symbol-coverage` flag exits with code 1 when any module falls below the threshold

---

### S8.2 — Hallucination detector

**Status:** `todo`

**Description:**  
Scan all `docs/*.md` files for symbol references inside backticks. For each backtick-wrapped name that matches a plausible identifier pattern (starts with uppercase, contains no spaces), check whether it exists in `file_index.json`. Names not found are flagged as potential hallucinations and written to `artifacts/validation_report.json` under `hallucinated_symbols`.

**Acceptance criteria:**
- `` `NonExistentFunc` `` in a module doc appears in `hallucinated_symbols` if it is not in `file_index.json`
- Common non-symbol terms accidentally wrapped in backticks (e.g. `` `true` ``, `` `nil` ``, `` `error` ``) are excluded via an allowlist
- Report includes the doc file path and line number of each hallucination

---

### S8.3 — Shared module re-description detector

**Status:** `todo`

**Description:**  
For each shared module, check that no non-owner module doc contains a paragraph that substantively describes it (as opposed to linking to it). Use heuristic matching: if a non-owner doc contains 3+ sentences mentioning the shared module's name or key function names from `shared_context.json`, flag it as a potential ownership violation.

**Acceptance criteria:**
- A module doc that contains 4 sentences describing `pkg/logger` internals (when `logger` is a shared module owned elsewhere) is flagged
- A module doc that contains only `See [logger](../shared/logger.md)` is not flagged
- Flagged violations appear in `validation_report.json` under `ownership_violations`

---

### S8.4 — `wikismit eval`: LLM-as-judge scoring

**Status:** `todo`

**Description:**  
Implement the `wikismit eval` subcommand. For each module, send the module doc + its source code skeleton to the LLM with a structured prompt asking for scores (1–5) on three dimensions: accuracy (does the doc match the code?), completeness (are key symbols covered?), and clarity (is it easy to understand?). Write scores to `artifacts/eval_report.json`.

**Acceptance criteria:**
- `wikismit eval` produces `artifacts/eval_report.json` with scores for every module
- Each entry contains: `module_id`, `accuracy`, `completeness`, `clarity`, `notes` (LLM's one-sentence justification per dimension)
- `wikismit eval --module=auth` scores only the `auth` module (for quick spot-checks)
- Eval uses the same `planner_model` (cheaper model) since scoring requires less capability than generation

---

### S8.5 — Eval baseline and regression detection

**Status:** `todo`

**Description:**  
Extend `wikismit eval` with a baseline comparison mode. On first run, save `eval_report.json` as `artifacts/eval_baseline.json`. On subsequent runs with `--compare-baseline`, compute the delta per module per dimension. Modules where any score drops by more than 1 point are reported as regressions. This is the primary tool for detecting prompt quality regressions.

**Acceptance criteria:**
- `wikismit eval --save-baseline` writes current scores to `eval_baseline.json`
- `wikismit eval --compare-baseline` prints a regression table: module, dimension, baseline score, current score, delta
- A regression of > 1 point on any dimension exits with code 1 (blockable in CI)
- `eval_baseline.json` is committed to the repository; `eval_report.json` is gitignored

---

## Appendix: Story Count Summary

| Epic | Stories | Notes |
|---|---|---|
| Epic 1 | 5 | Foundation — must be done first |
| Epic 2 | 4 | Go only |
| Epic 3 | 4 | |
| Epic 4 | 3 | |
| Epic 5 | 4 | |
| Epic 6 | 4 | |
| **Subtotal v1 vertical slice** | **24** | |
| Epic 7A | 8 | Largest single Epic |
| Epic 7B | 4 | |
| Epic 7C | 4 | |
| Epic 7D | 4 | |
| Epic 7E | 4 | |
| Epic 7F | 4 | |
| Epic 7G | 1 | |
| Epic 8 | 5 | |
| **Subtotal hardening + quality** | **34** | |
| **Total** | **58** | |

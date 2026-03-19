# wikismit — Technical Specification

**Version:** 0.1.0-draft
**Status:** Draft
**Last updated:** 2026-03-18

---

## Table of Contents

1. [Overview](#1-overview)
2. [Goals and Non-goals](#2-goals-and-non-goals)
3. [Architecture Overview](#3-architecture-overview)
4. [Pipeline Phases](#4-pipeline-phases)
   - [Phase 1 — AST Analysis](#phase-1--ast-analysis)
   - [Phase 2 — Module Planner](#phase-2--module-planner)
   - [Phase 3 — Shared Preprocessor](#phase-3--shared-preprocessor)
   - [Phase 4 — Agent Fan-out](#phase-4--agent-fan-out)
   - [Phase 5 — Doc Composer](#phase-5--doc-composer)
5. [Artifact Schemas](#5-artifact-schemas)
6. [Directory Structure](#6-directory-structure)
7. [Key Interfaces](#7-key-interfaces)
8. [LLM Integration](#8-llm-integration)
9. [Incremental Update Mode](#9-incremental-update-mode)
10. [CLI Design](#10-cli-design)
11. [Configuration](#11-configuration)
12. [Error Handling and Retry Strategy](#12-error-handling-and-retry-strategy)
13. [Testing Strategy](#13-testing-strategy)
14. [Design Decisions and Trade-offs](#14-design-decisions-and-trade-offs)
15. [Out of Scope (v1)](#15-out-of-scope-v1)
16. [Documentation Deployment — VitePress](#16-documentation-deployment--vitepress)

---

## 1. Overview

**wikismit** is a CLI tool that automatically generates structured, traceable wiki documentation for software repositories. It statically analyses source code via AST parsing, then uses a multi-agent LLM pipeline to produce Markdown documentation with precise `file:line` citations.

The tool is designed to run as a single binary inside CI pipelines without external service dependencies. It supports full generation and incremental updates triggered by git diff.

### Design philosophy

- **Traceability first.** Every documented symbol must carry a `file:line` reference back to source. This is the primary failure point of existing tools (deepwiki-rs, CodeWiki) and is addressed by keeping `file_index.json` separate from LLM context.
- **Shared module consistency.** All four prior implementations (deepwiki-rs, CodeWiki, DeepWiki Open, Mintlify) allow multiple agents to independently describe shared utilities, producing conflicting documentation. This tool resolves ownership at planning time and enforces it at prompt construction time.
- **CI-first.** Single binary, ~30s cold start, no Python runtime, no Qdrant service, no Docker required.
- **Deterministic structure, LLM-generated prose.** AST drives module boundaries and dependency graphs. LLM is only used for prose generation and module-level semantic planning — never for structural decisions.

---

## 2. Goals and Non-goals

### Goals

- Generate wiki-quality Markdown documentation for multi-language repositories.
- Preserve `file:line` citations in all generated docs.
- Resolve shared module ownership before parallel doc generation.
- Support incremental updates: only re-generate documentation for modules affected by a git diff.
- Run as a self-contained binary in GitHub Actions and other CI systems.
- Support any OpenAI-compatible LLM API (OpenAI, Anthropic via proxy, Ollama, Azure OpenAI).
- Produce VitePress-compatible output for static deployment to GitHub Pages or Cloudflare Pages.

### Non-goals (v1)

- Interactive question-answering / RAG chat interface (no vector embedding pipeline).
- Support for binary or non-text source files.
- Real-time streaming output to the terminal (LLM calls are batched per agent).
- Proprietary or closed artifact formats.

---

## 3. Architecture Overview

The pipeline is strictly sequential across five phases. Each phase reads from and writes to a shared `artifacts/` directory. Phases communicate only through JSON files — there is no in-memory state passed between phases.

```
Source Repo
    │
    ▼
Phase 1 — AST Analysis          ──► file_index.json
    │                           ──► dep_graph.json
    ▼
Phase 2 — Module Planner        ──► nav_plan.json
    │         (1 LLM call)
    ▼
Phase 3 — Shared Preprocessor   ──► shared_context.json
    │         (N serial LLM calls, one per shared module)
    ▼
Phase 4 — Agent Fan-out         ──► module_docs/*.md
    │         (M parallel LLM calls via goroutines)
    ▼
Phase 5 — Doc Composer          ──► docs/*.md
    │         (citation inject, cross-ref validation, TOC)
    │                           ──► docs/.vitepress/config.ts
    ▼
VitePress Build                 ──► docs/.vitepress/dist/
    │         (vitepress build)
    ▼
GitHub Pages / Cloudflare Pages
```

**Core invariant:** Phase N+1 never starts until Phase N has fully written its artifact(s) to disk.

---

## 4. Pipeline Phases

### Phase 1 — AST Analysis

**Package:** `internal/analyzer`

**Input:** Repository path (from config or CLI flag)

**Output:** `artifacts/file_index.json`, `artifacts/dep_graph.json`

**What it does:**

Uses `go-tree-sitter` bindings to parse each source file and extract:

- All function/method declarations with their signatures and line ranges.
- All type declarations (structs, interfaces, enums, classes).
- All import statements, resolved to internal paths where possible.

From the per-file data it constructs:

- `file_index.json` — a flat map from file path to its extracted symbols, with `line_start`/`line_end` preserved.
- `dep_graph.json` — a directed adjacency list of import dependencies between internal files. External (third-party) imports are recorded but not traversed.

**Supported languages (v1):**

| Language | tree-sitter grammar | Notes |
|---|---|---|
| Go | `go-tree-sitter` built-in | Primary target |
| Python | `tree-sitter-python` | |
| TypeScript/JavaScript | `tree-sitter-javascript`, `tree-sitter-typescript` | |
| Rust | `tree-sitter-rust` | |
| Java | `tree-sitter-java` | |

**LLM usage:** None. Phase 1 is entirely deterministic.

**Incremental behaviour:** If `--incremental` flag is set, only files in the changed set (from `gitdiff.go`) are re-parsed. The existing `file_index.json` is loaded and updated in-place.

---

### Phase 2 — Module Planner

**Package:** `internal/planner`

**Input:** `artifacts/file_index.json`, `artifacts/dep_graph.json`

**Output:** `artifacts/nav_plan.json`

**What it does:**

Constructs a compressed skeleton from `file_index.json` (function signatures only, no bodies) and sends a single LLM call to produce a module grouping plan.

The LLM is asked to:
1. Group related files into logical modules (bounded by business domain, not directory).
2. Identify which modules are _shared_ (imported by 3 or more other modules).
3. Assign an `owner` field: `"agent"` for private modules, `"shared_preprocessor"` for shared ones.
4. Output valid JSON matching the `nav_plan.json` schema.

**Why one LLM call, not zero?** Static import analysis alone cannot determine semantic module boundaries — a `pkg/utils` directory may contain unrelated helpers that should be documented in separate sections. The LLM provides the semantic grouping layer; the AST provides the structural constraints.

**Why not let each agent decide its own scope?** Overlapping scope is the root cause of inconsistent shared-module documentation in all prior implementations. Ownership must be locked before parallelism begins.

**Prompt design:**

```
You are a software architect. Given this repository skeleton, group the files
into logical documentation modules. Identify shared utilities used by 3+ modules.

Rules:
- Every file must appear in exactly one module.
- Shared modules (pkg/logger, pkg/errors, etc.) must have owner: "shared_preprocessor".
- Respond ONLY with valid JSON matching the schema below. No preamble.

Schema: { modules: [{ id, files[], shared, owner, depends_on_shared[] }] }

Skeleton:
{skeleton}
```

**LLM usage:** 1 call. Uses the cheapest capable model (e.g. `gpt-4o-mini` or `claude-haiku`).

---

### Phase 3 — Shared Preprocessor

**Package:** `internal/preprocessor`

**Input:** `artifacts/nav_plan.json`, `artifacts/file_index.json`

**Output:** `artifacts/shared_context.json`

**What it does:**

Iterates serially over every module with `"owner": "shared_preprocessor"` in `nav_plan.json` and generates a concise summary for each.

Each summary includes:
- A 2-4 sentence description of the module's purpose and primary usage pattern.
- The key exported types and functions (names only, no prose descriptions).
- Source references (`file:line`) for the key symbols.

**Why serial?** Shared modules may themselves import other shared modules. Serial processing in dependency order ensures that when `pkg/middleware` is summarised, `pkg/logger`'s summary is already available to inject into the prompt.

**Ordering:** Topological sort of the shared module subgraph from `dep_graph.json`.

**LLM usage:** One call per shared module.

---

### Phase 4 — Agent Fan-out

**Package:** `internal/agent`

**Input:** `artifacts/nav_plan.json`, `artifacts/file_index.json`, `artifacts/shared_context.json`

**Output:** `artifacts/module_docs/{module_id}.md` (one file per module)

**What it does:**

`scheduler.go` reads all non-shared modules from `nav_plan.json` and launches one goroutine per module up to a configurable concurrency limit (`--concurrency`, default 4). Each goroutine runs `agent.go`, which:

1. Builds a prompt from the module's code skeleton + the relevant subset of `shared_context.json`.
2. Calls the LLM and collects the full response.
3. Sends the result to a buffered `chan ModuleDoc`.

A collector goroutine drains the channel and writes each result to `artifacts/module_docs/`.

**Prompt construction (enforcing the shared module contract):**

```
You are a technical writer documenting the {module_id} module.

Files in this module:
{skeleton_of_module_files}

The following shared modules are used by this module. Do NOT re-describe them —
reference them by link only using the format [ModuleName](../shared/{id}.md).

{shared_summaries_for_deps}

Write a Markdown document covering:
1. Purpose and responsibility of this module.
2. Key types and their roles.
3. Key functions — one paragraph each, with source reference [FuncName](path/file.go#L{line}).
4. Usage examples where the skeleton reveals clear call patterns.

Do not describe implementation details of shared modules. Link only.
```

**Concurrency model:**

```go
type ModuleDoc struct {
    ModuleID string
    Content  string
    Err      error
}

results := make(chan ModuleDoc, len(modules))

sem := make(chan struct{}, concurrency)
var wg sync.WaitGroup

for _, mod := range modules {
    wg.Add(1)
    sem <- struct{}{}
    go func(m Module) {
        defer wg.Done()
        defer func() { <-sem }()
        doc, err := agent.Run(m, sharedCtx, fileIndex, llmClient)
        results <- ModuleDoc{ModuleID: m.ID, Content: doc, Err: err}
    }(mod)
}

go func() { wg.Wait(); close(results) }()
```

**LLM usage:** One call per non-shared module. This is the dominant cost centre.

---

### Phase 5 — Doc Composer

**Package:** `internal/composer`

**Input:** `artifacts/module_docs/`, `artifacts/file_index.json`, `artifacts/nav_plan.json`

**Output:** `docs/` directory (final Markdown + VitePress config)

**What it does:**

1. **Citation injection** (`citation.go`): Scans each `module_docs/*.md` for function references, looks up the canonical `file:line` from `file_index.json`, and replaces bare function names with anchored links where they're missing.

2. **Cross-reference validation** (`validator.go`): Checks that every `[link](../shared/{id}.md)` in module docs points to a file that exists in `docs/shared/`. Logs warnings for broken references; does not fail the build in v1.

3. **Renderer** (`renderer.go`): Assembles the final `docs/` directory:
   - `docs/index.md` — top-level landing page with project overview.
   - `docs/modules/{id}.md` — one file per module, from `module_docs/`.
   - `docs/shared/{id}.md` — one file per shared module.
   - Optionally injects Mermaid dependency diagrams from `dep_graph.json`.

4. **TOC generation**: Each output file gets a table of contents injected at the top.

5. **VitePress config generation** (`vitepress.go`): Generates `docs/.vitepress/config.ts` by mapping `nav_plan.json` to VitePress sidebar and nav structure. This is the only step that has knowledge of the deployment target; all other steps are deployment-agnostic Markdown.

**LLM usage:** None. Phase 5 is entirely deterministic.

---

## 5. Artifact Schemas

All artifacts are written to `artifacts/` (configurable). They are gitignored by default.

### `file_index.json`

```json
{
  "internal/auth/jwt.go": {
    "language": "go",
    "content_hash": "sha256:a3f...",
    "functions": [
      {
        "name": "GenerateToken",
        "signature": "func GenerateToken(user User, secret string) (string, error)",
        "line_start": 24,
        "line_end": 45,
        "exported": true
      }
    ],
    "types": [
      {
        "name": "Claims",
        "kind": "struct",
        "line_start": 12,
        "line_end": 22,
        "exported": true
      }
    ],
    "imports": [
      { "path": "github.com/golang-jwt/jwt", "internal": false },
      { "path": "internal/models", "internal": true }
    ]
  }
}
```

`content_hash` is used by incremental mode to detect which files have actually changed, independently of git diff.

### `dep_graph.json`

```json
{
  "internal/auth/jwt.go": ["internal/models/user.go", "pkg/logger/logger.go"],
  "internal/auth/middleware.go": ["internal/auth/jwt.go", "pkg/errors/errors.go"],
  "pkg/logger/logger.go": []
}
```

### `nav_plan.json`

```json
{
  "generated_at": "2026-03-18T10:00:00Z",
  "modules": [
    {
      "id": "auth",
      "files": ["internal/auth/jwt.go", "internal/auth/middleware.go"],
      "shared": false,
      "owner": "agent",
      "depends_on_shared": ["logger", "errors"]
    },
    {
      "id": "logger",
      "files": ["pkg/logger/logger.go"],
      "shared": true,
      "owner": "shared_preprocessor",
      "referenced_by": ["auth", "api", "db"]
    }
  ]
}
```

### `shared_context.json`

```json
{
  "logger": {
    "summary": "Structured logger wrapping zerolog. Provides contextual logging with field chaining. Use Logger.With(fields).Info(msg) for contextual entries.",
    "key_types": ["Logger", "Level", "Config"],
    "key_functions": [
      { "name": "New", "signature": "func New(cfg Config) Logger", "ref": "pkg/logger/logger.go#L18" },
      { "name": "Logger.Info", "signature": "func (l Logger) Info(msg string, fields ...Field)", "ref": "pkg/logger/logger.go#L42" }
    ]
  }
}
```

---

## 6. Directory Structure

```
wikismit/
├── cmd/
│   └── wikismit/
│       └── main.go                  # CLI entrypoint
│
├── internal/
│   ├── analyzer/
│   │   ├── analyzer.go              # Orchestrates per-file parsing
│   │   ├── parser.go                # tree-sitter wrapper
│   │   ├── dep_graph.go             # Dependency graph construction
│   │   ├── file_index.go            # Symbol index builder
│   │   └── lang/
│   │       ├── golang.go
│   │       ├── python.go
│   │       ├── typescript.go
│   │       ├── rust.go
│   │       └── java.go
│   │
│   ├── planner/
│   │   ├── planner.go               # Phase 2 orchestration
│   │   └── skeleton.go              # Skeleton serialiser (token budget aware)
│   │
│   ├── preprocessor/
│   │   ├── preprocessor.go          # Phase 3 orchestration
│   │   └── shared_context.go        # shared_context.json writer
│   │
│   ├── agent/
│   │   ├── scheduler.go             # Goroutine pool + channel
│   │   ├── agent.go                 # Single-module doc generator
│   │   └── prompt.go                # Prompt builder (injects shared summaries)
│   │
│   ├── composer/
│   │   ├── composer.go              # Phase 5 orchestration
│   │   ├── validator.go             # Cross-reference checker
│   │   ├── renderer.go              # Final Markdown assembly
│   │   ├── citation.go              # file:line citation injector
│   │   └── vitepress.go             # VitePress config.ts generator
│   │
│   └── llm/
│       ├── client.go                # go-openai wrapper
│       ├── streaming.go             # Streaming response accumulator
│       └── retry.go                 # Exponential backoff with jitter
│
├── pkg/
│   ├── gitdiff/
│   │   └── diff.go                  # git diff → affected module set
│   └── store/
│       ├── index.go                 # file_index / dep_graph read-write
│       └── artifacts.go             # nav_plan / shared_context read-write
│
├── artifacts/                       # Runtime artifacts (gitignored)
│   ├── file_index.json
│   ├── dep_graph.json
│   ├── nav_plan.json
│   ├── shared_context.json
│   ├── module_docs/
│   └── cache/                       # Optional: keyed by content_hash + prompt_hash
│
├── docs/                            # Generated documentation output
│   ├── index.md                     # Landing page
│   ├── modules/                     # Per-module docs
│   ├── shared/                      # Shared module docs
│   └── .vitepress/
│       ├── config.ts                # Auto-generated from nav_plan.json
│       └── dist/                    # VitePress build output (gitignored)
│
├── examples/
│   └── github/
│       ├── docs-full.yml            # GitHub Actions: full generation + deploy
│       ├── docs-incremental.yml     # GitHub Actions: incremental update + deploy
│       └── README.md                # Setup instructions for GitHub Pages
│
├── config.yaml
├── .gitignore                       # includes artifacts/
└── go.mod
```

---

## 7. Key Interfaces

```go
// internal/analyzer
type FileIndex map[string]*FileEntry

type FileEntry struct {
    Language    string
    ContentHash string
    Functions   []FunctionDecl
    Types       []TypeDecl
    Imports     []Import
}

type FunctionDecl struct {
    Name      string
    Signature string
    LineStart  int
    LineEnd    int
    Exported  bool
}

// internal/planner
type Module struct {
    ID              string
    Files           []string
    Shared          bool
    Owner           string // "agent" | "shared_preprocessor"
    DependsOnShared []string
    ReferencedBy    []string
}

type NavPlan struct {
    GeneratedAt time.Time
    Modules     []Module
}

// internal/agent
type AgentInput struct {
    Module        *Module
    FileIndex     *FileIndex
    SharedContext *SharedContext
}

type ModuleDoc struct {
    ModuleID string
    Content  string
    Err      error
}

// internal/llm
type Client interface {
    Complete(ctx context.Context, req *CompletionRequest) (string, error)
    CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan string, error)
}

type CompletionRequest struct {
    Model       string
    SystemMsg   string
    UserMsg     string
    MaxTokens   int
    Temperature float32
}
```

---

## 8. LLM Integration

### Client

`internal/llm/client.go` wraps `github.com/sashabaranov/go-openai`. The `Client` interface is satisfied by a thin struct that holds the API key and base URL, enabling injection of test doubles.

```go
type openAIClient struct {
    c       *openai.Client
    model   string
    timeout time.Duration
}
```

Any OpenAI-compatible endpoint is supported by setting `base_url` in `config.yaml`:

| Provider | Base URL |
|---|---|
| OpenAI | `https://api.openai.com/v1` (default) |
| Anthropic (proxy) | `https://api.anthropic.com/v1` |
| Ollama (local) | `http://localhost:11434/v1` |
| Azure OpenAI | `https://{resource}.openai.azure.com/openai/deployments/{deploy}` |

### Model assignment per phase

| Phase | Suggested model | Rationale |
|---|---|---|
| Phase 2 (planning) | `gpt-4o-mini` / `claude-haiku` | Structured JSON output, low cost |
| Phase 3 (shared summaries) | `gpt-4o-mini` / `claude-haiku` | Short targeted summaries |
| Phase 4 (module docs) | `gpt-4o` / `claude-sonnet` | Long-form quality matters here |

Both `planner_model` and `agent_model` are separately configurable.

### LLM call budget estimate

For a 100,000 LOC Go repository with ~40 modules and ~5 shared modules:

| Phase | Calls | Approx tokens (in+out) | Estimated cost (gpt-4o) |
|---|---|---|---|
| Phase 2 | 1 | ~8k | ~$0.05 |
| Phase 3 | 5 | ~3k each = 15k | ~$0.09 |
| Phase 4 | 35 | ~6k each = 210k | ~$1.40 |
| **Total** | **41** | **~233k** | **~$1.54** |

Incremental update (single PR, 3 modules changed): ~$0.12.

### Caching

Responses are cached in `artifacts/cache/` keyed by `sha256(prompt + model)`. If the source files contributing to a module have not changed (`content_hash` match in `file_index.json`), the cached response is returned without an LLM call.

---

## 9. Incremental Update Mode

Triggered by `wikismit update` (vs `wikismit generate` for full runs).

### Algorithm

```
1. Load existing artifacts/file_index.json
2. Run Phase 1 on the changed file set only (from git diff or --changed-files flag)
3. For each changed file, update file_index.json entries and recompute dep_graph edges
4. Compute affected_modules = direct_owners(changed_files) ∪ upstream_dependents(changed_files)
5. If any shared module is in affected_modules → re-run Phase 3 for that module first (serial)
6. Re-run Phase 4 for all non-shared modules in affected_modules
7. Re-run Phase 5 (always full — cross-reference validation is cheap)
```

### Changed file input

```bash
# Option A: from git
wikismit update --repo=.

# Option B: explicit list (for custom CI triggers)
wikismit update --changed-files=internal/auth/jwt.go,pkg/logger/logger.go
```

`pkg/gitdiff/diff.go` uses `go-git` to compute `HEAD~1..HEAD` diff by default. The ref range is configurable via `--base-ref` and `--head-ref`.

---

## 10. CLI Design

```
wikismit <command> [flags]

Commands:
  generate    Full documentation generation from scratch
  update      Incremental update based on git diff
  plan        Run Phase 1+2 only, output nav_plan.json (for inspection)
  validate    Run Phase 5 validation only, report broken cross-references
  build       Run vitepress build on the generated docs/ directory

Global flags:
  --config string        Path to config.yaml (default: ./config.yaml)
  --repo string          Repository root path (default: .)
  --output string        Output directory for docs/ (default: ./docs)
  --artifacts string     Artifacts directory (default: ./artifacts)
  --concurrency int      Max parallel agents in Phase 4 (default: 4)
  --verbose              Enable verbose logging

generate flags:
  --model-planner string    LLM model for Phase 2 (default: gpt-4o-mini)
  --model-agent string      LLM model for Phase 4 (default: gpt-4o)
  --no-cache                Disable LLM response cache

update flags:
  --base-ref string         Base git ref for diff (default: HEAD~1)
  --head-ref string         Head git ref for diff (default: HEAD)
  --changed-files string    Comma-separated list of changed files (bypasses git diff)
```

---

## 11. Configuration

`config.yaml`:

```yaml
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"

llm:
  base_url: "https://api.openai.com/v1"
  api_key_env: "OPENAI_API_KEY"       # reads from env var
  planner_model: "gpt-4o-mini"
  agent_model: "gpt-4o"
  max_tokens: 4096
  temperature: 0.2
  timeout_seconds: 120

analysis:
  languages: ["go", "python", "typescript", "rust", "java"]
  exclude_patterns:
    - "*_test.go"
    - "vendor/**"
    - "node_modules/**"
    - "**/*.pb.go"
  shared_module_threshold: 3          # min importers for a module to be "shared"

agent:
  concurrency: 4                          # max parallel agents in Phase 4
  skeleton_max_tokens: 3000           # token budget for code skeleton per module

cache:
  enabled: true
  dir: "./artifacts/cache"
```

---

## 12. Error Handling and Retry Strategy

### LLM errors

`internal/llm/retry.go` implements exponential backoff with jitter:

- Max retries: 3
- Initial backoff: 2s
- Max backoff: 30s
- Retried errors: `429 TooManyRequests`, `500`, `503`, network timeouts
- Non-retried errors: `400 BadRequest`, `401 Unauthorized`

### Phase 4 partial failures

If an individual agent goroutine fails after retries, the error is recorded in `ModuleDoc.Err` and sent to the results channel. The scheduler does not cancel other goroutines. After all goroutines complete, the composer reports which modules failed and continues with the rest.

This means a documentation run can complete with partial output rather than failing entirely — important for large repositories where one flaky LLM call should not invalidate 35 other successful ones.

### JSON parse failures in Phase 2

If the LLM returns malformed JSON for `nav_plan.json`, Phase 2 retries with a tightened prompt that includes the parse error. If the third attempt fails, the run aborts with a clear error message.

---

## 13. Testing Strategy

### Unit tests

- `internal/analyzer`: golden-file tests per language. Each test fixture is a small synthetic source file; the expected `FileEntry` is stored as a `.golden.json` file beside the fixture.
- `internal/planner/skeleton.go`: property tests verifying the skeleton never exceeds the configured token budget.
- `internal/agent/prompt.go`: snapshot tests verifying the prompt structure when shared context is and is not present.
- `internal/composer/citation.go`: table-driven tests for the citation injection regex.

### Integration tests

- A `testdata/sample_repo/` directory contains a synthetic multi-language repository with known structure. The full pipeline is run against it and the output is diffed against a committed golden output.
- Integration tests are gated behind a `//go:build integration` tag and require `OPENAI_API_KEY` to be set. They are not run in standard `go test ./...`.

### LLM mock

`internal/llm` exports a `MockClient` that returns configurable pre-canned responses. All unit and integration tests (except the golden E2E test) use this mock.

---

## 14. Design Decisions and Trade-offs

### No vector embedding (no RAG)

Prior implementations (notably DeepWiki Open) use vector embeddings to retrieve relevant code chunks. This was explicitly excluded because:

1. The AST dependency graph is a more precise retrieval mechanism for _structured_ queries ("what files are in the auth module").
2. Embedding the entire codebase adds infrastructure (Qdrant or similar), cold-start cost, and incremental re-embedding complexity with no measurable quality benefit for documentation generation.
3. RAG answers "semantically similar" queries, not "structurally dependent" queries. For doc generation, call-graph proximity matters more than semantic similarity — `Logger.Info()` and `Auth.GenerateToken()` have low semantic similarity but a direct dependency that must appear in the generated docs.

### LLM for module grouping, AST for everything else

LLM is used _only_ where static analysis is insufficient:

- Module boundary decisions are semantically ambiguous — `pkg/utils/strings.go` and `pkg/utils/time.go` may belong to the same module or different ones depending on project convention. LLM handles this once in Phase 2.
- All structural decisions (what files exist, what symbols they export, how they depend on each other) are from the AST.

This is in contrast to CodeWiki, which uses LLM for module _clustering_ (a structural decision) with `eval()` to parse the output — both an architectural error and a security risk.

### Serial Phase 3, parallel Phase 4

Shared modules may depend on other shared modules. Serial processing in topological order ensures that each shared module summary can reference the summaries of its own dependencies, producing more accurate and consistent cross-linked documentation.

Non-shared modules are independent by definition (they do not import other non-shared modules without going through a shared module), making them safe to process in parallel.

### JSON artifacts over in-memory pipeline

Passing state through JSON files on disk costs a small amount of I/O but provides:

- Easy inspection and debugging (`cat artifacts/nav_plan.json`).
- Natural checkpoint resume (restart from Phase 3 without re-running Phase 1+2).
- Compatibility with external tooling (a pre-existing `nav_plan.json` can be provided to skip Phase 2 entirely).
- A clear contract surface between phases — no implicit shared state.

---

## 15. Out of Scope (v1)

The following are explicitly deferred to future versions:

- **Diagram generation beyond Mermaid.** Mermaid is generated from `dep_graph.json` deterministically. Richer visual diagrams require a separate rendering step.
- **Private repository SaaS mode.** v1 is a local CLI only. A server mode that hosts documentation would require authentication, multi-tenancy, and a serving layer.
- **PR diff preview comment.** Generating a documentation change summary as a GitHub PR comment requires GitHub API integration beyond the scope of the CLI.
- **Language server plugin.** In-editor hover documentation from the generated index is a separate integration concern.

---

## 16. Documentation Deployment — VitePress

### Why VitePress

VitePress is a static documentation framework built on Vite and Vue. For this tool it is the preferred build layer because:

- Sidebar navigation is driven by a `config.ts` file that `vitepress.go` generates automatically from `nav_plan.json` — no manual configuration required.
- Built-in full-text search (MiniSearch, local index, no external service) covers all browse and keyword lookup use cases without any additional step.
- Mermaid diagrams are supported natively via a VitePress plugin, matching the output of Phase 5's renderer.
- `vitepress build` produces a standard static HTML/CSS/JS directory with no server-side runtime requirement.

### Output is plain static HTML

`vitepress build` produces `docs/.vitepress/dist/` — a directory of standard HTML, CSS, and JS files with no server-side component. This output can be served by any static file host:

```
docs/.vitepress/dist/
├── index.html
├── modules/
│   ├── auth.html
│   └── api.html
├── shared/
│   └── logger.html
└── assets/          # hashed JS/CSS bundles
```

The user is free to copy this directory to any host they prefer — GitHub Pages, Cloudflare Pages, Nginx, S3, an internal file server, or simply open `index.html` locally. There is no lock-in to any specific platform.

### The `wikismit build` command

`wikismit build` is a thin wrapper around `vitepress build` for users who do not want to install Node.js separately:

```bash
# Generate docs and build the static site in one step
wikismit generate && wikismit build

# Output is ready at docs/.vitepress/dist/
# Copy it wherever you want
```

Node.js 20+ must be available in `PATH`. VitePress is installed locally under `docs/node_modules/` on first run and cached for subsequent runs.

### Generated `config.ts` structure

`internal/composer/vitepress.go` writes `docs/.vitepress/config.ts` at the end of Phase 5:

```typescript
// docs/.vitepress/config.ts  (auto-generated — do not edit manually)
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'myproject docs',
  description: 'Auto-generated by wikismit',
  themeConfig: {
    search: { provider: 'local' },
    sidebar: [
      {
        text: 'Modules',
        items: [
          { text: 'auth',   link: '/modules/auth' },
          { text: 'api',    link: '/modules/api' },
          { text: 'db',     link: '/modules/db' },
        ]
      },
      {
        text: 'Shared',
        items: [
          { text: 'logger', link: '/shared/logger' },
          { text: 'errors', link: '/shared/errors' },
        ]
      }
    ]
  }
})
```

The `title` field is derived from the repository name. The sidebar order follows module dependency depth from `dep_graph.json` (leaf modules first).

### CI integration examples

GitHub Actions workflows for automating doc generation and deployment are provided as reference examples in `examples/github/`:

| File | Purpose |
|---|---|
| `examples/github/docs-full.yml` | Full generation on every push to `main`, deploy `dist/` to GitHub Pages |
| `examples/github/docs-incremental.yml` | Incremental update with `artifacts/` cache, two-job pattern (generate + deploy) |
| `examples/github/README.md` | Step-by-step setup instructions, required secrets, Cloudflare Pages variant |

These are opt-in starting points. Users who prefer a different CI system, a different deployment target, or a manual workflow can ignore them entirely and work directly with the `dist/` output.

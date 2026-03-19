# wikismit Epic 1 Implementation Plan

> **Status:** Superseded by the split Epic 1 planning set below.
>
> Use these files instead:
> - `.docs/plans/2026-03-19-wikismit-epic1-requirements-breakdown.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-index.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-01-scaffold-config.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-02-cli-surface.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-03-llm-client.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-04-retry-mock.md`
> - `.docs/plans/2026-03-19-wikismit-epic1-plan-05-artifact-store-verification.md`

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Epic 1 foundation for `wikismit`: a runnable Go CLI that loads validated config, can perform a real LLM completion through an OpenAI-compatible endpoint, supports retry logic and mocking, and provides the artifact store layer used by later epics.

**Architecture:** Implement Epic 1 in five slices that match the task doc: project scaffold + CLI, config loading/validation, LLM client, retry/logging, mock client, and artifact store. Keep the CLI thin, keep `internal/config` and `internal/llm` independently testable, and require every slice to land with tests before moving on.

**Tech Stack:** Go, Cobra, `gopkg.in/yaml.v3`, `github.com/sashabaranov/go-openai`, `log/slog`, `net/http/httptest`, standard `testing` package.

---

## Preconditions and references

Before starting implementation, read these docs once:

- `.docs/tasks/wikismit-epic1.md`
- `.docs/spec/wikismit-tech-spec.md:402-479` (directory structure)
- `.docs/spec/wikismit-tech-spec.md:551-600` (LLM integration)
- `.docs/spec/wikismit-tech-spec.md:633-724` (CLI, config, retry)
- `.docs/spec/wikismit-tech-spec.md:728-744` (testing strategy)

Assumptions for this plan:

- The repo currently has no Go source code yet.
- `.gitignore` currently only ignores `.omc`; Epic 1 must add runtime artifact ignores.
- This plan follows Epic 1 only. It stubs future phases instead of implementing them.

Suggested verification commands to reuse throughout the plan:

```bash
go test ./...
go test ./internal/config -v
go test ./internal/llm -v
go test ./pkg/store -v
go build ./cmd/wikismit
./wikismit --help
```

---

### Task 1: Initialize the Go module and repository skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/wikismit/.gitkeep`
- Create: `internal/analyzer/.gitkeep`
- Create: `internal/analyzer/lang/.gitkeep`
- Create: `internal/planner/.gitkeep`
- Create: `internal/preprocessor/.gitkeep`
- Create: `internal/agent/.gitkeep`
- Create: `internal/composer/.gitkeep`
- Create: `internal/llm/.gitkeep`
- Create: `internal/config/.gitkeep`
- Create: `internal/log/.gitkeep`
- Create: `pkg/gitdiff/.gitkeep`
- Create: `pkg/store/.gitkeep`
- Create: `artifacts/module_docs/.gitkeep`
- Create: `artifacts/cache/.gitkeep`
- Create: `docs/modules/.gitkeep`
- Create: `docs/shared/.gitkeep`
- Create: `docs/.vitepress/.gitkeep`
- Create: `examples/github/.gitkeep`
- Modify: `.gitignore`

**Step 1: Create the module and directories**

Run:

```bash
go mod init github.com/scalaview/wikismit
mkdir -p cmd/wikismit internal/{analyzer/lang,planner,preprocessor,agent,composer,llm,config,log} pkg/{gitdiff,store} artifacts/{module_docs,cache} docs/{modules,shared,.vitepress} examples/github
touch cmd/wikismit/.gitkeep internal/analyzer/.gitkeep internal/analyzer/lang/.gitkeep internal/planner/.gitkeep internal/preprocessor/.gitkeep internal/agent/.gitkeep internal/composer/.gitkeep internal/llm/.gitkeep internal/config/.gitkeep internal/log/.gitkeep pkg/gitdiff/.gitkeep pkg/store/.gitkeep artifacts/module_docs/.gitkeep artifacts/cache/.gitkeep docs/modules/.gitkeep docs/shared/.gitkeep docs/.vitepress/.gitkeep examples/github/.gitkeep
```

Expected: `go.mod` exists and the directory layout matches the tech spec's top-level shape.

**Step 2: Update `.gitignore` for generated/runtime paths**

Add these ignore rules:

```gitignore
.omc
artifacts/*.json
artifacts/cache/
artifacts/module_docs/*.md
docs/.vitepress/dist/
```

Expected: runtime artifacts and built site output are ignored; placeholder `.gitkeep` files remain trackable.

**Step 3: Add initial dependencies**

Run:

```bash
go get github.com/spf13/cobra gopkg.in/yaml.v3
go mod tidy
```

Expected: `go.mod` and `go.sum` resolve cleanly with no build errors.

**Step 4: Verify the scaffold**

Run:

```bash
go mod tidy
```

Expected: exits 0 with no missing packages.

**Step 5: Commit**

```bash
git add go.mod go.sum .gitignore cmd internal pkg artifacts docs examples
git commit -m "chore: scaffold Epic 1 project layout"
```

---

### Task 2: Define config structures and defaults

**Files:**
- Create: `internal/config/config.go`
- Create: `config.yaml`

**Step 1: Write the failing config tests first skeleton**

Create an initial test file with placeholders for the default-loading cases that will be completed in Task 4:

```go
package config

import "testing"

func TestLoadConfigAppliesDefaults(t *testing.T) {
	// fill in after LoadConfig exists
}
```

Expected: the package now has a test target, even though it does not compile yet.

**Step 2: Define the full config model in `internal/config/config.go`**

Implement:

- `type Config struct`
- `type LLMConfig struct`
- `type AnalysisConfig struct`
- `type AgentConfig struct`
- `type CacheConfig struct`
- `type SiteConfig struct`

Fields must match Epic 1 and spec §11 exactly:

```go
type Config struct {
	RepoPath     string         `yaml:"repo_path"`
	OutputDir    string         `yaml:"output_dir"`
	ArtifactsDir string         `yaml:"artifacts_dir"`
	LLM          LLMConfig      `yaml:"llm"`
	Analysis     AnalysisConfig `yaml:"analysis"`
	Agent        AgentConfig    `yaml:"agent"`
	Cache        CacheConfig    `yaml:"cache"`
	Site         SiteConfig     `yaml:"site"`
}
```

Also add an unexported field to `LLMConfig`:

```go
resolvedAPIKey string
```

**Step 3: Implement defaulting helpers**

Implement functions like:

```go
func defaultConfig() Config
func applyDefaults(cfg *Config)
```

Default values must include:

- `RepoPath: "."`
- `OutputDir: "./docs"`
- `ArtifactsDir: "./artifacts"`
- `LLM.BaseURL: "https://api.openai.com/v1"`
- `LLM.PlannerModel: "gpt-4o-mini"`
- `LLM.AgentModel: "gpt-4o"`
- `LLM.MaxTokens: 4096`
- `LLM.Temperature: 0.2`
- `LLM.TimeoutSeconds: 120`
- `Agent.Concurrency: 4`
- `Agent.SkeletonMaxTokens: 3000`
- `Cache.Enabled: true`
- `Cache.Dir: "./artifacts/cache"`
- `Analysis.SharedModuleThreshold: 3`

**Step 4: Create the sample `config.yaml`**

Write a real config file matching the spec shape:

```yaml
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"

llm:
  base_url: "https://api.openai.com/v1"
  api_key_env: "OPENAI_API_KEY"
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
  shared_module_threshold: 3

agent:
  concurrency: 4
  skeleton_max_tokens: 3000

cache:
  enabled: true
  dir: "./artifacts/cache"

site:
  title: ""
  repo_url: ""
  logo: ""
```

**Step 5: Commit**

```bash
git add internal/config/config.go config.yaml
git commit -m "feat: define Epic 1 config schema and defaults"
```

---

### Task 3: Implement config loading, API key resolution, and validation

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests for config load and validation**

Add these tests:

```go
func TestLoadConfigLoadsValidYAML(t *testing.T) {}
func TestLoadConfigReturnsErrorWhenAPIKeyEnvMissing(t *testing.T) {}
func TestLoadConfigReadsResolvedAPIKey(t *testing.T) {}
func TestValidateRejectsBadConcurrency(t *testing.T) {}
func TestValidateRejectsMissingRepoPath(t *testing.T) {}
```

Use `t.TempDir()` to create a fake repo directory and temporary YAML files.

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/config -v
```

Expected: FAIL because `LoadConfig`, `Validate`, and `APIKey` are not fully implemented yet.

**Step 3: Implement `LoadConfig(path string) (*Config, error)`**

Requirements:

- read YAML from disk
- unmarshal into `Config`
- apply defaults after unmarshal
- resolve env var named by `cfg.LLM.APIKeyEnv`
- store result in `cfg.LLM.resolvedAPIKey`
- return error `env var OPENAI_API_KEY is not set` when the env var name exists but value is empty

Add:

```go
func (c LLMConfig) APIKey() string
```

**Step 4: Implement validation**

Implement:

```go
func (c *Config) Validate() error
```

Validation rules:

- `RepoPath` exists and is a directory
- `LLM.APIKeyEnv` is non-empty
- `Agent.Concurrency` is in `[1, 32]`
- `Analysis.SharedModuleThreshold >= 1`

Use `errors.Join(...)` to accumulate multiple failures.

**Step 5: Run tests until green**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config loading and validation"
```

---

### Task 4: Wire the root Cobra CLI and stub commands

**Files:**
- Create: `cmd/wikismit/main.go`
- Create: `cmd/wikismit/generate.go`
- Create: `cmd/wikismit/update.go`
- Create: `cmd/wikismit/plan.go`
- Create: `cmd/wikismit/validate.go`
- Create: `cmd/wikismit/build.go`

**Step 1: Write the failing CLI smoke expectation**

Document the smoke checks in a temporary checklist comment at the top of `cmd/wikismit/main.go`:

```go
// Smoke checks after wiring:
// 1. go build ./cmd/wikismit
// 2. ./wikismit --help
// 3. ./wikismit generate --config ./config.yaml
```

**Step 2: Implement the root command**

Create `main.go` with:

- root Cobra command named `wikismit`
- persistent `--config` flag defaulting to `./config.yaml`
- persistent `--verbose` bool flag
- package-level variables `configPath string` and `verbose bool`
- `Execute()` pattern from Cobra

**Step 3: Implement common config bootstrap helper**

In one command file or shared helper inside `cmd/wikismit`, add a function like:

```go
func loadAndValidateConfig() (*config.Config, error)
```

Each subcommand should call this before any action.

**Step 4: Implement stub subcommands**

Create commands for:

- `generate`
- `update`
- `plan`
- `validate`
- `build`

Behavior for now:

- load config
- validate config
- on error: print error to `stderr` and exit non-zero
- on success: `generate` prints the resolved config in dry-run form; other commands print `not implemented`

For `generate`, format output with `%+v` or JSON so the operator can see resolved values.

**Step 5: Build and smoke test**

Run:

```bash
go build -o ./wikismit ./cmd/wikismit
./wikismit --help
./wikismit generate --config ./config.yaml
```

Expected:

- build succeeds
- help lists `generate`, `update`, `plan`, `validate`, `build`
- `generate` loads config and prints resolved values without panicking

**Step 6: Commit**

```bash
git add cmd/wikismit
git commit -m "feat: add Epic 1 CLI scaffold"
```

---

### Task 5: Define LLM request/response interfaces

**Files:**
- Create: `internal/llm/types.go`

**Step 1: Write the types exactly as Epic 1 requires**

Implement:

```go
package llm

import "context"

type CompletionRequest struct {
	Model       string
	SystemMsg   string
	UserMsg     string
	MaxTokens   int
	Temperature float32
}

type Client interface {
	Complete(ctx context.Context, req CompletionRequest) (string, error)
}
```

Do not add streaming yet.

**Step 2: Verify package compiles**

Run:

```bash
go test ./internal/llm -run TestNonExistent -count=0
```

Expected: package compiles or reports “no test files” but does not fail because of missing types.

**Step 3: Commit**

```bash
git add internal/llm/types.go
git commit -m "feat: define llm client interfaces"
```

---

### Task 6: Implement the basic OpenAI-compatible client

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/client_test.go`
- Modify: `go.mod`

**Step 1: Write failing client tests with `httptest`**

Add tests:

```go
func TestClientCompleteReturnsResponseContent(t *testing.T) {}
func TestClientCompleteMaps401ToNonRetryableLLMError(t *testing.T) {}
func TestClientCompleteMaps500ToRetryableLLMError(t *testing.T) {}
func TestClientCompleteMapsTimeoutToRetryableLLMError(t *testing.T) {}
```

Use `httptest.NewServer` to return OpenAI-compatible JSON like:

```json
{
  "choices": [
    {"message": {"content": "hello from test"}}
  ]
}
```

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/llm -run TestClient -v
```

Expected: FAIL because client and error types do not exist yet.

**Step 3: Add `go-openai` and implement `LLMError`**

Run:

```bash
go get github.com/sashabaranov/go-openai
go mod tidy
```

Implement in `client.go`:

```go
type LLMError struct {
	StatusCode int
	Message    string
	Retryable  bool
}
```

Add `Error() string` so it satisfies `error`.

**Step 4: Implement `openAIClient` and `NewClient`**

Requirements:

- hold `*openai.Client`
- hold `defaultModel string`
- hold `timeout time.Duration`
- use `openai.DefaultConfig(cfg.APIKey())`
- override `BaseURL`
- create client with `openai.NewClientWithConfig`

**Step 5: Implement `Complete`**

Requirements:

- derive a timeout context from the incoming context
- choose request model: `req.Model` if present, otherwise default model
- call `CreateChatCompletion`
- send `SystemMsg` and `UserMsg` as chat messages
- return `choices[0].Message.Content`
- normalize API and timeout failures into `LLMError`

**Step 6: Run tests until green**

Run:

```bash
go test ./internal/llm -run TestClient -v
```

Expected: PASS.

**Step 7: Add an optional manual smoke helper**

Do not wire this into CLI yet. Add a small test-only note in comments showing how to instantiate the client with `config.LLMConfig` and call `Complete` against a real endpoint.

**Step 8: Commit**

```bash
git add go.mod go.sum internal/llm/client.go internal/llm/client_test.go
git commit -m "feat: add OpenAI-compatible llm client"
```

---

### Task 7: Add retrying client wrapper

**Files:**
- Create: `internal/llm/retry.go`
- Create: `internal/llm/retry_test.go`
- Create: `internal/log/log.go`

**Step 1: Write failing retry tests**

Add tests:

```go
func TestRetryingClientRetriesOnceThenSucceeds(t *testing.T) {}
func TestRetryingClientStopsAfterMaxRetries(t *testing.T) {}
func TestRetryingClientDoesNotRetry401(t *testing.T) {}
func TestRetryingClientStopsWhenContextCancelled(t *testing.T) {}
```

For testability, design the retry wrapper so timer creation or sleep duration can be observed without waiting real 30-second intervals.

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/llm -run TestRetryingClient -v
```

Expected: FAIL because retry wrapper does not exist yet.

**Step 3: Implement minimal logger abstraction**

In `internal/log/log.go`, define:

```go
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}
```

Also add a small `slog`-backed implementation and constructor like:

```go
func New(verbose bool) Logger
```

**Step 4: Implement backoff and retry wrapper**

In `retry.go`, implement:

- `type retryingClient struct`
- `func NewRetryingClient(inner Client, maxRetries int, logger log.Logger) Client`
- `func backoffDuration(attempt int) time.Duration`

Rules:

- base 2s, doubling each attempt
- cap at 30s
- jitter ±20%
- only retry `LLMError{Retryable: true}` and timeout-like failures
- use `time.NewTimer` so waits can be interrupted by context cancellation

**Step 5: Run retry tests**

Run:

```bash
go test ./internal/llm -run TestRetryingClient -v
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/llm/retry.go internal/llm/retry_test.go internal/log/log.go
git commit -m "feat: add retrying llm client"
```

---

### Task 8: Implement the mock LLM client used by later epics

**Files:**
- Create: `internal/llm/mock.go`
- Create: `internal/llm/mock_test.go`

**Step 1: Write failing mock tests**

Add tests:

```go
func TestMockClientReturnsResponsesInOrder(t *testing.T) {}
func TestMockClientErrorsWhenResponsesExhausted(t *testing.T) {}
func TestMockClientReturnsConfiguredErrorOnNthCall(t *testing.T) {}
func TestMockClientRecordsCalls(t *testing.T) {}
```

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/llm -run TestMockClient -v
```

Expected: FAIL because mock client does not exist yet.

**Step 3: Implement `MockClient`**

Implement exactly these members:

```go
type MockClient struct {
	responses []string
	errors    []error
	calls     []CompletionRequest
	mu        sync.Mutex
}
```

And:

- `NewMockClient(responses ...string) *MockClient`
- `WithErrors(errs ...error) *MockClient`
- `Complete(ctx context.Context, req CompletionRequest) (string, error)`
- `Calls() []CompletionRequest`
- `CallCount() int`

If you want assertion helpers, add them only if they keep the package simple.

**Step 4: Run tests until green**

Run:

```bash
go test ./internal/llm -run TestMockClient -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/llm/mock.go internal/llm/mock_test.go
git commit -m "feat: add mock llm client"
```

---

### Task 9: Implement artifact types and atomic JSON write/read helpers

**Files:**
- Create: `pkg/store/store.go`
- Create: `pkg/store/index.go`
- Create: `pkg/store/artifacts.go`
- Create: `pkg/store/store_test.go`

**Step 1: Write failing store tests**

Add tests:

```go
func TestWriteAndReadFileIndexRoundTrip(t *testing.T) {}
func TestWriteAndReadDepGraphRoundTrip(t *testing.T) {}
func TestWriteAndReadNavPlanRoundTrip(t *testing.T) {}
func TestWriteAndReadSharedContextRoundTrip(t *testing.T) {}
func TestReadReturnsErrArtifactNotFound(t *testing.T) {}
func TestConcurrentWritesLeaveValidJSON(t *testing.T) {}
```

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./pkg/store -v
```

Expected: FAIL because artifact types and read/write helpers do not exist yet.

**Step 3: Implement artifact structs in `artifacts.go`**

Copy the Epic 1 schema exactly for:

- `FileIndex`
- `FileEntry`
- `FunctionDecl`
- `TypeDecl`
- `Import`
- `DepGraph`
- `Module`
- `NavPlan`
- `SharedSummary`
- `KeyFunction`
- `SharedContext`

**Step 4: Implement shared JSON helpers in `store.go`**

Implement:

```go
var ErrArtifactNotFound = errors.New("artifact not found")

func writeJSON(path string, v any) error
func readJSON(path string, v any) error
```

Rules:

- create parent dirs
- write to `*.tmp`
- rename atomically
- indent JSON for readability
- wrap missing-file errors as `ErrArtifactNotFound`

**Step 5: Implement typed read/write functions in `index.go`**

Implement:

- `WriteFileIndex`
- `ReadFileIndex`
- `WriteDepGraph`
- `ReadDepGraph`
- `WriteNavPlan`
- `ReadNavPlan`
- `WriteSharedContext`
- `ReadSharedContext`

All should construct file paths from the artifact directory argument.

**Step 6: Run tests until green**

Run:

```bash
go test ./pkg/store -v
```

Expected: PASS.

**Step 7: Commit**

```bash
git add pkg/store
git commit -m "feat: add artifact store layer"
```

---

### Task 10: Perform Epic 1 full verification

**Files:**
- Verify only; no intended file creation

**Step 1: Run the focused test suites**

Run:

```bash
go test ./internal/config -v
go test ./internal/llm -v
go test ./pkg/store -v
```

Expected: all PASS.

**Step 2: Run the full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

**Step 3: Build the CLI**

Run:

```bash
go build -o ./wikismit ./cmd/wikismit
```

Expected: PASS, binary created.

**Step 4: Smoke test the CLI contract**

Run:

```bash
./wikismit --help
./wikismit generate --config ./config.yaml
./wikismit update --config ./config.yaml
./wikismit plan --config ./config.yaml
./wikismit validate --config ./config.yaml
./wikismit build --config ./config.yaml
```

Expected:

- `--help` lists all required subcommands
- `generate` loads config and prints resolved config
- the other commands print `not implemented` after successful config bootstrap
- no panics

**Step 5: Check final status**

Run:

```bash
git status --short
```

Expected: clean working tree if all commits were made during the plan.

**Step 6: Final commit if needed**

If verification uncovered any required fixups not yet committed:

```bash
git add .
git commit -m "test: finish Epic 1 verification fixes"
```

---

## Notes for the implementing engineer

- Keep `generate`, `update`, `plan`, `validate`, and `build` intentionally thin in Epic 1. Only `generate` should do a dry-run config print; the others should stop after config bootstrap.
- Do not implement streaming, planner logic, analyzer logic, git diff logic, or composer logic in this plan. Epic 1 only establishes the foundation they will rely on.
- Prefer `errors.Join` over a new dependency for multi-error validation unless a concrete need appears.
- Keep tests package-local and table-driven where it improves readability.
- If the `go-openai` API has changed, keep the plan’s contract the same: one OpenAI-compatible completion path, base URL override, timeout support, and normalized error handling.

## Suggested commit sequence

1. `chore: scaffold Epic 1 project layout`
2. `feat: define Epic 1 config schema and defaults`
3. `feat: add config loading and validation`
4. `feat: add Epic 1 CLI scaffold`
5. `feat: define llm client interfaces`
6. `feat: add OpenAI-compatible llm client`
7. `feat: add retrying llm client`
8. `feat: add mock llm client`
9. `feat: add artifact store layer`
10. `test: finish Epic 1 verification fixes`

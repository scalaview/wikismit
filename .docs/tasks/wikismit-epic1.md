# wikismit — Epic 1: Project Scaffold + LLM Client

**Status:** `todo`
**Depends on:** —
**Goal:** A runnable CLI binary that can read config and make a real LLM call. All subsequent Epics build on this foundation.
**Spec refs:** §6 Directory Structure, §8 LLM Integration, §10 CLI Design, §11 Configuration

---

## S1.1 — Project skeleton and CLI entry point

**Status:** `todo`

**Description:**
Initialise the Go module, directory structure, and Cobra CLI. Implement config loading from `config.yaml` with env var override for `api_key_env`. The binary must respond to `--help` and load config without panicking.

**Acceptance criteria:**
- `go build ./cmd/wikismit` produces a binary with no errors
- `wikismit --help` lists all subcommands: `generate`, `update`, `plan`, `validate`, `build`
- `wikismit generate --config ./config.yaml` reads and prints the resolved config (dry-run, no actual work)
- Missing required config fields produce a clear error message, not a panic
- `config.yaml` structure matches §11 of the tech spec

**Files to create:**
```
cmd/wikismit/main.go
cmd/wikismit/generate.go
cmd/wikismit/update.go
cmd/wikismit/plan.go
cmd/wikismit/validate.go
cmd/wikismit/build.go
internal/config/config.go
internal/config/config_test.go
config.yaml
go.mod
.gitignore
```

### Subtasks

#### S1.1.1 — Initialise Go module and directory skeleton

- Run `go mod init github.com/scalaview/wikismit`
- Create the full directory tree from §6 of the tech spec as empty directories with `.gitkeep`
- Add `go.sum` and `go.mod` with initial dependencies: `github.com/spf13/cobra`, `gopkg.in/yaml.v3`
- Verify `go mod tidy` runs without error

#### S1.1.2 — Define `Config` struct and YAML loader

- Define `Config` struct in `internal/config/config.go` with all fields from §11:
  - `RepoPath`, `OutputDir`, `ArtifactsDir`
  - `LLM` sub-struct: `BaseURL`, `APIKeyEnv`, `PlannerModel`, `AgentModel`, `MaxTokens`, `Temperature`, `TimeoutSeconds`
  - `Analysis` sub-struct: `Languages []string`, `ExcludePatterns []string`, `SharedModuleThreshold int`
  - `Agent` sub-struct: `Concurrency int`, `SkeletonMaxTokens int`
  - `Cache` sub-struct: `Enabled bool`, `Dir string`
  - `Site` sub-struct: `Title`, `RepoURL`, `Logo`
- Implement `LoadConfig(path string) (*Config, error)` using `gopkg.in/yaml.v3`
- Apply defaults for all optional fields (e.g. `Concurrency` defaults to 4, `BaseURL` defaults to `https://api.openai.com/v1`)

#### S1.1.3 — Implement env var override for API key

- After loading YAML, read `os.Getenv(cfg.LLM.APIKeyEnv)` and store the resolved key in a separate `cfg.LLM.resolvedAPIKey` (unexported field)
- If `APIKeyEnv` is set but the env var is empty, return a descriptive error: `"env var OPENAI_API_KEY is not set"`
- Expose `cfg.LLM.APIKey()` method that returns the resolved key

#### S1.1.4 — Config validation

- Implement `cfg.Validate() error` that checks:
  - `RepoPath` is a valid, existing directory
  - `LLM.APIKeyEnv` is non-empty
  - `Agent.Concurrency` is between 1 and 32
  - `Analysis.SharedModuleThreshold` is ≥ 1
- Return a `multierror` (or joined error) listing all violations at once, not just the first

#### S1.1.5 — Cobra CLI wiring

- Implement `cmd/wikismit/main.go` with root command and persistent `--config` flag (default: `./config.yaml`)
- Implement stub subcommands for `generate`, `update`, `plan`, `validate`, `build` — each prints `"not implemented"` for now
- All subcommands call `LoadConfig` and `Validate` before doing any work; config errors are printed to `stderr` and exit with code 1
- Add `--verbose` flag to root command; set a package-level `verbose bool` used by all subcommands

#### S1.1.6 — Config unit tests

- Test: valid `config.yaml` loads without error and all fields are populated correctly
- Test: missing `api_key_env` field returns validation error
- Test: env var named in `api_key_env` is absent → error
- Test: env var present → `APIKey()` returns its value
- Test: `Concurrency: 0` fails validation; `Concurrency: 4` passes
- Use `t.Setenv` for env var tests; no global state

---

## S1.2 — LLM client: basic completion

**Status:** `todo`

**Description:**
Implement `internal/llm/client.go` wrapping `go-openai`. The `Client` interface exposes `Complete(ctx, CompletionRequest) (string, error)`. Base URL, model, and timeout must be configurable. Works with any OpenAI-compatible endpoint.

**Acceptance criteria:**
- `Client` interface is defined and satisfied by `openAIClient`
- A manual smoke test can send a message to a real OpenAI endpoint and print the reply
- Switching to Ollama requires only a config change (no code change)
- API key is read from env var, never hardcoded

**Files to create:**
```
internal/llm/client.go
internal/llm/types.go
internal/llm/client_test.go
```

### Subtasks

#### S1.2.1 — Define `Client` interface and request/response types

- Define in `internal/llm/types.go`:
  ```go
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
- No other methods on the interface in v1 (streaming deferred to Epic 7B)

#### S1.2.2 — Implement `openAIClient`

- Add dependency: `github.com/sashabaranov/go-openai`
- Implement `openAIClient` struct holding `*openai.Client`, `defaultModel string`, `timeout time.Duration`
- Implement `NewClient(cfg config.LLMConfig) (Client, error)`:
  - Build `openai.ClientConfig` with `BaseURL` and `AuthToken` from config
  - Wrap in `openai.NewClientWithConfig`
- Implement `Complete`: call `client.CreateChatCompletion` with a context that has the configured timeout, return `choices[0].Message.Content`

#### S1.2.3 — Error normalisation

- Wrap `go-openai` errors into a local `LLMError` type:
  ```go
  type LLMError struct {
      StatusCode int
      Message    string
      Retryable  bool
  }
  ```
- `Retryable: true` for status codes 429, 500, 503, and `context.DeadlineExceeded`
- `Retryable: false` for 400, 401, 403
- All callers check `LLMError.Retryable` rather than inspecting status codes directly

#### S1.2.4 — Unit test with mock HTTP server

- Use `net/http/httptest` to create a local server returning a valid OpenAI response JSON
- Test: successful completion returns the expected content string
- Test: server returns 401 → `LLMError{StatusCode: 401, Retryable: false}`
- Test: server returns 500 → `LLMError{StatusCode: 500, Retryable: true}`
- Test: context timeout fires → `LLMError{Retryable: true}`

---

## S1.3 — LLM client: retry with exponential backoff

**Status:** `todo`

**Description:**
Implement `internal/llm/retry.go`. Wrap the base client with retry logic for retryable errors. Max 3 retries, initial backoff 2s, max backoff 30s, with jitter.

**Acceptance criteria:**
- `429` retries up to 3 times with increasing delay, then returns error
- `401` fails immediately with no retry
- Retry attempts logged at `DEBUG` level
- All behaviour covered by unit tests with a mock HTTP server

**Files to create:**
```
internal/llm/retry.go
internal/llm/retry_test.go
```

### Subtasks

#### S1.3.1 — Implement `retryingClient` wrapper

- Implement `retryingClient` struct wrapping an inner `Client`
- Constructor: `NewRetryingClient(inner Client, maxRetries int) Client`
- On each `Complete` call, if the returned error is an `LLMError` with `Retryable: true`, wait then retry
- If error is not `LLMError` or `Retryable: false`, return immediately

#### S1.3.2 — Exponential backoff with jitter

- Implement `backoffDuration(attempt int) time.Duration`:
  - Base: `2s * 2^attempt` (attempt 0 → 2s, attempt 1 → 4s, attempt 2 → 8s)
  - Cap at 30s
  - Add random jitter: `±20%` of the computed duration using `math/rand`
- Use `time.NewTimer` (not `time.Sleep`) so the wait is context-cancellable

#### S1.3.3 — Logging

- Add a minimal structured logger interface in `internal/log/log.go`:
  ```go
  type Logger interface {
      Debug(msg string, fields ...any)
      Info(msg string, fields ...any)
      Warn(msg string, fields ...any)
      Error(msg string, fields ...any)
  }
  ```
- Use `log/slog` as the default implementation (stdlib, Go 1.21+)
- `--verbose` flag sets level to `DEBUG`; default level is `INFO`
- `retryingClient` logs at `DEBUG`: attempt number, wait duration, error message

#### S1.3.4 — Retry unit tests

- Use a counter-based mock HTTP server that returns 429 for the first N calls, then 200
- Test: 1 failure then success → total 2 calls, success returned
- Test: 3 consecutive failures → error returned after 3rd attempt, no 4th call
- Test: non-retryable 401 on first call → error returned immediately, no retry
- Test: context cancelled during backoff wait → returns context error immediately
- Assert actual sleep durations are within the expected jitter range using a fake clock or duration capture

---

## S1.4 — Mock LLM client

**Status:** `todo`

**Description:**
Implement `MockClient` satisfying the `Client` interface. Returns pre-configured responses in sequence and records all calls for test assertions.

**Acceptance criteria:**
- Configurable response sequence: first call → resp1, second → resp2
- `MockClient.Calls()` returns all received `CompletionRequest` values
- Fewer configured responses than calls → error on extra calls
- All subsequent Epics use `MockClient` in unit tests

**Files to create:**
```
internal/llm/mock.go
internal/llm/mock_test.go
```

### Subtasks

#### S1.4.1 — Implement `MockClient` struct

- Fields:
  ```go
  type MockClient struct {
      responses []string
      errors    []error        // parallel slice; nil entry = no error
      calls     []CompletionRequest
      mu        sync.Mutex
  }
  ```
- Constructor: `NewMockClient(responses ...string) *MockClient`
- `WithErrors(errs ...error) *MockClient` — chainable setter for per-call errors

#### S1.4.2 — Implement `Complete` method

- On each call, pop the next response/error pair from the front of their slices (thread-safe via `mu`)
- If no responses remain, return `fmt.Errorf("MockClient: no more responses configured (call %d)", callIndex)`
- Append the received `CompletionRequest` to `calls`

#### S1.4.3 — Assertion helpers

- `Calls() []CompletionRequest` — returns a copy of all recorded calls
- `CallCount() int` — number of calls made so far
- `AssertCallCount(t *testing.T, n int)` — fails test if call count ≠ n
- `AssertNthCallContains(t *testing.T, n int, substr string)` — fails if nth call's `UserMsg` does not contain `substr`

#### S1.4.4 — Mock unit tests

- Test: two configured responses → two calls return resp1, resp2 in order
- Test: one configured response, two calls → second call returns "no more responses" error
- Test: configured error on call 2 → `Complete` returns that error on second call
- Test: `Calls()` returns correct `CompletionRequest` values for each call

---

## S1.5 — Artifact store layer

**Status:** `todo`

**Description:**
Implement `pkg/store` as the single read/write interface for all JSON artifacts. All pipeline phases use this package for artifact IO; no phase handles file serialisation directly. Writes are atomic (temp file + rename).

**Acceptance criteria:**
- All four artifact types round-trip correctly (write → read → identical struct)
- Atomic write: partial write never leaves a corrupt artifact
- 100% unit test coverage for read/write/round-trip

**Files to create:**
```
pkg/store/store.go
pkg/store/index.go
pkg/store/artifacts.go
pkg/store/store_test.go
```

### Subtasks

#### S1.5.1 — Define artifact types

- Define all artifact structs in `pkg/store/artifacts.go`, matching schemas from §5 of the tech spec:
  ```go
  type FileIndex map[string]FileEntry

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
      Exported   bool
  }

  type TypeDecl struct {
      Name      string
      Kind      string // "struct" | "interface" | "enum" | "class"
      LineStart int
      Exported  bool
  }

  type Import struct {
      Path     string
      Internal bool
  }

  type DepGraph map[string][]string  // file path → []dependency file paths

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

  type SharedSummary struct {
      Summary      string
      KeyTypes     []string
      KeyFunctions []KeyFunction
      SourceRefs   []string
  }

  type KeyFunction struct {
      Name      string
      Signature string
      Ref       string // "path/file.go#L18"
  }

  type SharedContext map[string]SharedSummary // module id → summary
  ```

#### S1.5.2 — Implement atomic write helper

- Implement `writeJSON(path string, v any) error` in `pkg/store/store.go`:
  1. Marshal `v` to JSON with `encoding/json` (indented for readability)
  2. Write to `path + ".tmp"`
  3. `os.Rename(path+".tmp", path)` — atomic on POSIX systems
  4. Ensure parent directory exists (`os.MkdirAll`) before writing

#### S1.5.3 — Implement read/write for each artifact

- For each artifact type, implement a pair:
  - `WriteFileIndex(dir string, idx FileIndex) error`
  - `ReadFileIndex(dir string) (FileIndex, error)`
  - Same pattern for `DepGraph`, `NavPlan`, `SharedContext`
- All paths are constructed as `filepath.Join(dir, "file_index.json")` etc.
- `Read*` returns a typed error `ErrArtifactNotFound` when the file does not exist, so callers can distinguish "not found" from "corrupt JSON"

#### S1.5.4 — Artifact store unit tests

- Test each type: write a known value, read it back, assert deep equality
- Test atomic write: simulate a crash mid-write by making the `.tmp` file exist but not renamed; verify `ReadFileIndex` returns `ErrArtifactNotFound` (not corrupt data)
- Test `ErrArtifactNotFound`: call `ReadFileIndex` on an empty directory, assert the specific error type
- Test concurrent writes: two goroutines writing the same artifact simultaneously; verify the final file is valid JSON (atomicity guarantee)

# Epic 1 Retry and Mock Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the retry wrapper, logger abstraction, and mock LLM client needed by later epics.

**Architecture:** Keep retry behavior outside the base client so the base transport stays simple. Add a tiny logger interface and a deterministic mock client that later planner/agent/composer tests can reuse.

**Tech Stack:** Go, `log/slog`, `sync`, `context`, standard `testing`.

---

### Task 1: Add retry tests first

**Files:**
- Create: `internal/llm/retry_test.go`

**Step 1: Add failing tests**

```go
func TestRetryingClientRetriesOnceThenSucceeds(t *testing.T) {}
func TestRetryingClientStopsAfterMaxRetries(t *testing.T) {}
func TestRetryingClientDoesNotRetry401(t *testing.T) {}
func TestRetryingClientStopsWhenContextCancelled(t *testing.T) {}
```

**Step 2: Run tests**

```bash
go test ./internal/llm -run TestRetryingClient -v
```

Expected: FAIL.

### Task 2: Implement logging abstraction and retry wrapper

**Files:**
- Create: `internal/log/log.go`
- Create: `internal/llm/retry.go`

**Step 1: Define logger interface**

```go
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}
```

Add a small `slog` implementation and constructor like `func New(verbose bool) Logger`.

**Step 2: Implement backoff helper**

```go
func backoffDuration(attempt int) time.Duration
```

Rules:

- 2s base
- exponential growth
- 30s cap
- ±20% jitter

**Step 3: Implement `retryingClient`**

Add:

- wrapper around `Client`
- constructor `NewRetryingClient(inner Client, maxRetries int, logger log.Logger) Client`
- retry only when `LLMError.Retryable` is true

**Step 4: Run tests**

```bash
go test ./internal/llm -run TestRetryingClient -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/log/log.go internal/llm/retry.go internal/llm/retry_test.go
git commit -m "feat: add retrying llm client"
```

### Task 3: Add mock client tests first

**Files:**
- Create: `internal/llm/mock_test.go`

**Step 1: Add failing tests**

```go
func TestMockClientReturnsResponsesInOrder(t *testing.T) {}
func TestMockClientErrorsWhenResponsesExhausted(t *testing.T) {}
func TestMockClientReturnsConfiguredErrorOnNthCall(t *testing.T) {}
func TestMockClientRecordsCalls(t *testing.T) {}
```

**Step 2: Run tests**

```bash
go test ./internal/llm -run TestMockClient -v
```

Expected: FAIL.

### Task 4: Implement `MockClient`

**Files:**
- Create: `internal/llm/mock.go`

**Step 1: Implement the struct**

```go
type MockClient struct {
	responses []string
	errors    []error
	calls     []CompletionRequest
	mu        sync.Mutex
}
```

**Step 2: Implement behavior**

Add:

- `NewMockClient(responses ...string)`
- `WithErrors(errs ...error)`
- `Complete(...)`
- `Calls()`
- `CallCount()`

**Step 3: Run tests**

```bash
go test ./internal/llm -run TestMockClient -v
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/llm/mock.go internal/llm/mock_test.go
git commit -m "feat: add mock llm client"
```

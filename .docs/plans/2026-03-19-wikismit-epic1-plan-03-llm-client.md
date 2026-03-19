# Epic 1 LLM Client Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the basic OpenAI-compatible completion client required by Epic 1.

**Architecture:** Separate protocol types from transport logic. `types.go` defines the stable contract; `client.go` owns the concrete `go-openai` integration and error normalization.

**Tech Stack:** Go, `github.com/sashabaranov/go-openai`, `net/http/httptest`, `context`, `time`.

---

### Task 1: Define the request/response contract

**Files:**
- Create: `internal/llm/types.go`

**Step 1: Write `CompletionRequest` and `Client`**

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

**Step 2: Verify the package compiles**

Run:

```bash
go test ./internal/llm -run TestNonExistent -count=0
```

### Task 2: Write failing client tests

**Files:**
- Create: `internal/llm/client_test.go`

**Step 1: Add `httptest` cases**

Add:

```go
func TestClientCompleteReturnsResponseContent(t *testing.T) {}
func TestClientCompleteMaps401ToNonRetryableLLMError(t *testing.T) {}
func TestClientCompleteMaps500ToRetryableLLMError(t *testing.T) {}
func TestClientCompleteMapsTimeoutToRetryableLLMError(t *testing.T) {}
```

**Step 2: Run tests and confirm failure**

Run:

```bash
go test ./internal/llm -run TestClient -v
```

### Task 3: Implement the client and normalized error type

**Files:**
- Create: `internal/llm/client.go`
- Modify: `go.mod`

**Step 1: Add dependency**

Run:

```bash
go get github.com/sashabaranov/go-openai
go mod tidy
```

**Step 2: Implement `LLMError`**

```go
type LLMError struct {
	StatusCode int
	Message    string
	Retryable  bool
}
```

Add `Error() string`.

**Step 3: Implement `openAIClient`**

Add:

- `c *openai.Client`
- `defaultModel string`
- `timeout time.Duration`

**Step 4: Implement `NewClient(cfg config.LLMConfig)`**

Use:

- `openai.DefaultConfig(cfg.APIKey())`
- base URL override
- `openai.NewClientWithConfig`

**Step 5: Implement `Complete`**

Required behavior:

- timeout context
- system + user messages
- use request model or default model
- return `choices[0].Message.Content`
- convert API/timeouts into `LLMError`

**Step 6: Run tests**

Run:

```bash
go test ./internal/llm -run TestClient -v
```

Expected: PASS.

**Step 7: Commit**

```bash
git add go.mod go.sum internal/llm/types.go internal/llm/client.go internal/llm/client_test.go
git commit -m "feat: add OpenAI-compatible llm client"
```

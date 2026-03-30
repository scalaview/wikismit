# Epic 8A.3 LLM Request Validation and Loop Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject obviously invalid LLM completion requests locally and replace the non-idiomatic `for true` loop without changing continuation behavior.

**Architecture:** Keep `CompletionRequest` as the validation boundary and keep `openAIClient.Complete` as the place that converts validation failures into non-retryable `LLMError`s before any provider call. Preserve the existing continuation protocol (`assistant` preoutput + `TruncatedOutputMessage`) and only clean up loop semantics plus local guardrails.

**Tech Stack:** Go, `httptest`, existing `internal/llm` client, OpenAI-compatible request payloads, standard `testing`.

---

### Task 1: Add failing tests for invalid requests and no-network rejection

**Files:**
- Modify: `internal/llm/client_test.go:17-241`
- Test: `internal/llm/client_test.go`

- [ ] **Step 1: Add a table-driven invalid-request test**

Add a new test such as `TestClientCompleteRejectsInvalidRequestsBeforeNetworkCall` with cases for:

1. `UserMsg == ""`
2. `MaxTokens == 0`
3. `MaxTokens < 0`

Use an `httptest.Server` whose handler increments a counter and fails the test if invoked:

```go
called := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    called++
    t.Fatalf("provider should not be called for invalid request")
}))
```

For each case, assert:

- `err != nil`
- `err` is `*LLMError`
- `Retryable == false`
- `called == 0`

- [ ] **Step 2: Run the invalid-request test to confirm RED**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteRejectsInvalidRequestsBeforeNetworkCall' -v
```

Expected: FAIL because the client currently sends invalid requests to the provider.

- [ ] **Step 3: Keep existing success/error tests untouched for now**

Do not rewrite:

- `TestClientCompleteReturnsResponseContent`
- `TestClientCompleteMaps401ToNonRetryableLLMError`
- `TestClientCompleteMaps500ToRetryableLLMError`
- `TestClientCompleteMapsTimeoutToRetryableLLMError`

These already lock the transport and error normalization behavior that must survive this epic.

- [ ] **Step 4: Commit the RED invalid-request test**

```bash
git add internal/llm/client_test.go
git commit -m "test: reject invalid completion requests locally"
```

### Task 2: Add failing continuation coverage before touching the loop

**Files:**
- Modify: `internal/llm/client_test.go:17-241`
- Read for context only: `internal/llm/prompt.go:1-4`

- [ ] **Step 1: Add a continuation test that exercises `FinishReasonLength`**

Add `TestClientCompleteContinuesWhenFinishReasonLengthIsReturned`. Use an `httptest.Server` that returns:

1. first response: `finish_reason: "length"`, content `"part1"`
2. second response: `finish_reason: "stop"`, content `"part2"`

Assert:

- final result is `"part1part2"`
- the handler is called exactly twice

- [ ] **Step 2: Capture the second request body and lock the continuation protocol**

Decode the second request body and assert the message list contains four messages in this order:

1. system
2. original user prompt
3. assistant preoutput with `part1`
4. user prompt equal to `TruncatedOutputMessage`

That prevents the loop cleanup from accidentally changing how continuation prompts are built.

- [ ] **Step 3: Run continuation coverage to confirm the current baseline**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteContinuesWhenFinishReasonLengthIsReturned' -v
```

Expected: If the continuation test is written against the current protocol, it should PASS before the loop cleanup. Keep it as a behavior lock, not a red test.

- [ ] **Step 4: Commit the continuation lock**

```bash
git add internal/llm/client_test.go
git commit -m "test: lock completion continuation behavior"
```

### Task 3: Implement request validation and replace `for true`

**Files:**
- Modify: `internal/llm/types.go:1-15`
- Modify: `internal/llm/client.go:62-159`

- [ ] **Step 1: Add a small validation helper on `CompletionRequest`**

Implement a method in `internal/llm/types.go`:

```go
func (r CompletionRequest) Validate() error {
    if r.UserMsg == "" {
        return errors.New("user message is required")
    }
    if r.MaxTokens <= 0 {
        return errors.New("max tokens must be positive")
    }
    return nil
}
```

Keep validation scope narrow. Do not add new rules for `SystemMsg`, `Temperature`, or model selection in this epic.

- [ ] **Step 2: Fail locally at the client boundary**

At the top of `(*openAIClient).Complete`, before timeout setup, add:

```go
if err := req.Validate(); err != nil {
    return "", &LLMError{Message: err.Error(), Retryable: false}
}
```

This must happen before any call to `c.complete(...)` and therefore before `CreateChatCompletion`.

- [ ] **Step 3: Replace `for true` with Go-idiomatic loop semantics**

Change:

```go
for true {
```

to:

```go
for {
```

Keep the existing exit behavior:

- append current chunk to `builder`
- if `FinishReasonLength`, update `preoutput` and continue
- otherwise append final chunk and `break`

Do not change message construction in `complete()`.

- [ ] **Step 4: Run focused LLM tests to confirm GREEN**

Run:

```bash
go test ./internal/llm -run 'TestClientComplete' -v
```

Expected: PASS.

- [ ] **Step 5: Commit the client cleanup**

```bash
git add internal/llm/types.go internal/llm/client.go internal/llm/client_test.go
git commit -m "fix: validate completion requests before provider calls"
```

### Task 4: Finish package and repository verification for the LLM slice

**Files:**
- Modify only if a test-driven fix is required

- [ ] **Step 1: Run the full LLM package suite**

Run:

```bash
go test ./internal/llm -v
```

Expected: PASS.

- [ ] **Step 2: Run the Epic 8A package bundle**

Run:

```bash
go test ./cmd/wikismit ./internal/pipeline ./internal/llm -v
```

Expected: PASS.

- [ ] **Step 3: Run full repository regression**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Confirm the loop cleanup stayed local in scope**

Before closing the slice, verify the diff only changes:

- `CompletionRequest` validation surface
- `Complete()` local guardrail
- `for true` loop syntax
- tests that lock invalid-request and continuation behavior

Do not leave behind retry redesign, logging rewrites, or transport changes.

- [ ] **Step 5: Commit the verification pass**

```bash
git add internal/llm/types.go internal/llm/client.go internal/llm/client_test.go
git commit -m "test: verify llm request validation regression suite"
```

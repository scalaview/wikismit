# Epic 9A.2 Fix `for true` Continuation Loop

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unbounded `for true` loop in `openAIClient.Complete` with a Go-idiomatic capped loop, preventing infinite token consumption when the model repeatedly returns `FinishReasonLength`.

**Architecture:** Add a `const maxContinuations = 5` to `client.go`. Replace `for true` with a counted loop. When the cap is reached and finish reason is still `Length`, log a warning and break with whatever content has been accumulated so far. Preserve the existing content accumulation protocol (`strings.Builder` + `preoutput` + `TruncatedOutputMessage`).

**Tech Stack:** Go, `httptest`, `strings`, existing `internal/llm` client, `log/slog`, standard `testing`.

---

### Task 1: Add failing test proving the continuation limit is enforced

**Files:**
- Modify: `internal/llm/client_test.go`
- Test: `internal/llm/client_test.go`

- [ ] **Step 1: Add a test that exercises the continuation limit**

Add `TestClientCompleteEnforcesContinuationCap` after the existing tests. Use an `httptest.Server` that always returns `finish_reason: "length"` with incremental content, then verify:
- The loop terminates (doesn't hang)
- The accumulated content includes all chunks up to the cap
- A warning was logged when the limit was reached

```go
func TestClientCompleteEnforcesContinuationCap(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":"chunk%d"},"finish_reason":"length"}]}`, callCount)
	}))
	defer server.Close()

	buf := &bytes.Buffer{}
	client := newClientWithTestLogger(t, configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 5,
	}, true, buf)

	got, err := client.Complete(context.Background(), CompletionRequest{
		UserMsg:   "generate a lot of text",
		MaxTokens: 10,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// The loop should stop after maxContinuations (5) iterations, not run forever.
	// Each iteration appends "chunk{i}" so the result should contain at most 5 chunks.
	if callCount > 6 { // maxContinuations + 1 safety margin
		t.Fatalf("server called %d times, expected cap at ~5 continuations", callCount)
	}

	// Must have accumulated some content, not returned empty
	if got == "" {
		t.Fatal("Complete() returned empty string, expected accumulated content")
	}

	// A warning should have been logged when the cap was reached
	if !strings.Contains(buf.String(), "max continuations") {
		t.Fatalf("log output missing max continuations warning:\n%s", buf.String())
	}
}
```

- [ ] **Step 2: Add a test proving normal continuation works correctly**

Add `TestClientCompleteContinuesAcrossPartialResponses` to lock the existing two-partial-response behavior:

```go
func TestClientCompleteContinuesAcrossPartialResponses(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"Hello "},"finish_reason":"length"}]}`)
		} else {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"World"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()

	client, err := NewClient(configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.Complete(context.Background(), CompletionRequest{
		UserMsg:   "say hello",
		MaxTokens: 10,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if got != "Hello World" {
		t.Fatalf("Complete() = %q, want %q", got, "Hello World")
	}
	if callCount != 2 {
		t.Fatalf("server called %d times, want 2", callCount)
	}
}
```

- [ ] **Step 3: Run the cap test to confirm RED**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteEnforcesContinuationCap' -v -timeout 10s
```

Expected: The test hangs or takes very long (the loop has no cap), confirming the bug. The `-timeout 10s` flag prevents it from running forever.

- [ ] **Step 4: Run the continuation behavior test to confirm GREEN (baseline lock)**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteContinuesAcrossPartialResponses' -v
```

Expected: PASS — the existing continuation protocol works for normal cases.

- [ ] **Step 5: Commit the tests**

```bash
git add internal/llm/client_test.go
git commit -m "test: add continuation cap enforcement and behavior lock tests"
```

---

### Task 2: Replace `for true` with capped Go-idiomatic loop

**Files:**
- Modify: `internal/llm/client.go:62-91`

- [ ] **Step 1: Add the max continuations constant**

Add after the import block in `client.go`, near the `LLMError` struct:

```go
// maxContinuations caps the number of LLM continuation rounds when
// the model returns finish_reason "length". Prevents unbounded token
// consumption if the model repeatedly truncates.
const maxContinuations = 5
```

- [ ] **Step 2: Replace `for true` with a counted loop**

Replace lines 73-88 in `client.go`:

```go
	// BEFORE:
	for true {

	// AFTER:
	for i := 0; i < maxContinuations; i++ {
```

- [ ] **Step 3: Add warning log when cap is reached**

Replace the `continue` branch (lines 79-84) with cap-aware logic:

```go
		if resp.FinishReason == openai.FinishReasonLength {
			builder.WriteString(resp.Message.Content)
			preoutput = builder.String()

			if i == maxContinuations-1 {
				c.logger.Warn("max continuations reached, returning truncated output",
					"max_continuations", maxContinuations,
					"accumulated_chars", builder.Len(),
				)
				break
			}

			continue
		}
```

The complete loop block becomes:

```go
	for i := 0; i < maxContinuations; i++ {
		resp, err := c.complete(requestCtx, &req, preoutput)
		if err != nil {
			return "", err
		}

		if resp.FinishReason == openai.FinishReasonLength {
			builder.WriteString(resp.Message.Content)
			preoutput = builder.String()

			if i == maxContinuations-1 {
				c.logger.Warn("max continuations reached, returning truncated output",
					"max_continuations", maxContinuations,
					"accumulated_chars", builder.Len(),
				)
				break
			}

			continue
		}

		builder.WriteString(resp.Message.Content)
		break
	}
```

- [ ] **Step 4: Run the cap test to confirm GREEN**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteEnforcesContinuationCap' -v -timeout 30s
```

Expected: PASS — the loop now terminates after 5 continuations.

- [ ] **Step 5: Run the continuation behavior test to confirm no regression**

Run:

```bash
go test ./internal/llm -run 'TestClientCompleteContinuesAcrossPartialResponses' -v
```

Expected: PASS — normal two-response continuation still works.

- [ ] **Step 6: Commit the fix**

```bash
git add internal/llm/client.go
git commit -m "fix: cap continuation loop at maxContinuations=5 with warning log"
```

---

### Task 3: Verify LLM package regression

**Files:**
- No modifications — verification only

- [ ] **Step 1: Run full LLM package tests**

Run:

```bash
go test ./internal/llm -v
```

Expected: ALL PASS.

- [ ] **Step 2: Run full repository regression**

Run:

```bash
go test ./...
```

Expected: ALL PASS, zero failures.

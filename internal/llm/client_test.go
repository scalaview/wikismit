package llm

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	configpkg "github.com/scalaview/wikismit/internal/config"
)

func TestClientCompleteReturnsResponseContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hello from test"}}]}`)
	}))
	defer server.Close()

	client, err := NewClient(configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.Complete(context.Background(), CompletionRequest{
		SystemMsg:   "system",
		UserMsg:     "user",
		MaxTokens:   100,
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "hello from test" {
		t.Fatalf("Complete() = %q, want %q", got, "hello from test")
	}
}

func TestClientCompleteMaps401ToNonRetryableLLMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient(configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Complete(context.Background(), CompletionRequest{UserMsg: "user"})
	if err == nil {
		t.Fatal("Complete() error = nil, want LLMError")
	}
	llmErr, ok := err.(*LLMError)
	if !ok {
		t.Fatalf("error type = %T, want *LLMError", err)
	}
	if llmErr.StatusCode != http.StatusUnauthorized || llmErr.Retryable {
		t.Fatalf("LLMError = %+v, want status 401 retryable false", llmErr)
	}
}

func TestClientCompleteMaps500ToRetryableLLMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"server error"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Complete(context.Background(), CompletionRequest{UserMsg: "user"})
	if err == nil {
		t.Fatal("Complete() error = nil, want LLMError")
	}
	llmErr, ok := err.(*LLMError)
	if !ok {
		t.Fatalf("error type = %T, want *LLMError", err)
	}
	if llmErr.StatusCode != http.StatusInternalServerError || !llmErr.Retryable {
		t.Fatalf("LLMError = %+v, want status 500 retryable true", llmErr)
	}
}

func TestClientCompleteMapsTimeoutToRetryableLLMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"late"}}]}`)
	}))
	defer server.Close()

	client, err := NewClient(configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 0,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = client.Complete(ctx, CompletionRequest{UserMsg: "user"})
	if err == nil {
		t.Fatal("Complete() error = nil, want timeout-based LLMError")
	}
	llmErr, ok := err.(*LLMError)
	if !ok {
		t.Fatalf("error type = %T, want *LLMError", err)
	}
	if !llmErr.Retryable {
		t.Fatalf("LLMError = %+v, want retryable timeout", llmErr)
	}
}

func TestClientCompleteVerboseLoggingIncludesRequestMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hello from test"}}]}`)
	}))
	defer server.Close()

	buf := &bytes.Buffer{}
	client := newClientWithTestLogger(t, configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 2,
	}, true, buf)

	userPrompt := "summarize this repository"
	_, err := client.Complete(context.Background(), CompletionRequest{
		SystemMsg:   "system instructions should stay private",
		UserMsg:     userPrompt,
		MaxTokens:   123,
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`level=DEBUG`,
		`msg="starting chat completion request"`,
		`msg="finished chat completion request"`,
		`model=gpt-4o`,
		`max_tokens=123`,
		`timeout_seconds=2`,
		`base_url=`,
		server.URL,
		fmt.Sprintf("user_prompt_chars=%d", len(userPrompt)),
		fmt.Sprintf("estimated_user_prompt_tokens=%d", (len(userPrompt)+3)/4),
		`started_at=`,
		`finished_at=`,
		`duration_ms=`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log output missing %q in %q", want, out)
		}
	}

	for _, unwanted := range []string{userPrompt, "system instructions should stay private", "api_key"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("log output = %q, should not contain %q", out, unwanted)
		}
	}
}

func TestClientCompleteVerboseLoggingIncludesErrorTypeOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"server error"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	buf := &bytes.Buffer{}
	client := newClientWithTestLogger(t, configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 2,
	}, true, buf)

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "user prompt"})
	if err == nil {
		t.Fatal("Complete() error = nil, want LLMError")
	}

	out := buf.String()
	for _, want := range []string{
		`msg="starting chat completion request"`,
		`msg="finished chat completion request"`,
		`error_type=*llm.LLMError`,
		`duration_ms=`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log output missing %q in %q", want, out)
		}
	}
}

func TestClientCompleteWithoutVerboseDoesNotEmitDebugLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hello from test"}}]}`)
	}))
	defer server.Close()

	buf := &bytes.Buffer{}
	client := newClientWithTestLogger(t, configpkg.LLMConfig{
		BaseURL:        server.URL,
		AgentModel:     "gpt-4o",
		TimeoutSeconds: 2,
	}, false, buf)

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "quiet prompt", MaxTokens: 10})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("log output = %q, want empty output when verbose is false", got)
	}
}

func newClientWithTestLogger(t *testing.T, cfg configpkg.LLMConfig, verbose bool, buf *bytes.Buffer) *openAIClient {
	t.Helper()

	rawClient, err := newClient(cfg, newBufferLogger(verbose, buf))
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}

	client, ok := rawClient.(*openAIClient)
	if !ok {
		t.Fatalf("client type = %T, want *openAIClient", rawClient)
	}
	return client
}

func newBufferLogger(verbose bool, buf *bytes.Buffer) *bufferLogger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	return &bufferLogger{
		inner: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level})),
	}
}

type bufferLogger struct {
	inner *slog.Logger
}

func (l *bufferLogger) Debug(msg string, fields ...any) {
	l.inner.DebugContext(context.Background(), msg, fields...)
}

func (l *bufferLogger) Info(msg string, fields ...any) {
	l.inner.InfoContext(context.Background(), msg, fields...)
}

func (l *bufferLogger) Warn(msg string, fields ...any) {
	l.inner.WarnContext(context.Background(), msg, fields...)
}

func (l *bufferLogger) Error(msg string, fields ...any) {
	l.inner.ErrorContext(context.Background(), msg, fields...)
}

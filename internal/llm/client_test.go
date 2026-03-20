package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

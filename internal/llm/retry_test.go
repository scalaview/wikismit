package llm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	logpkg "github.com/scalaview/wikismit/internal/log"
)

type stubClient struct {
	responses []string
	errors    []error
	calls     int
}

func (s *stubClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	_ = ctx
	_ = req
	idx := s.calls
	s.calls++
	if idx < len(s.errors) && s.errors[idx] != nil {
		return "", s.errors[idx]
	}
	if idx < len(s.responses) {
		return s.responses[idx], nil
	}
	return "", fmt.Errorf("unexpected call %d", idx)
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

func TestRetryingClientRetriesOnceThenSucceeds(t *testing.T) {
	inner := &stubClient{
		errors:    []error{&LLMError{StatusCode: 500, Message: "temporary", Retryable: true}},
		responses: []string{"", "success"},
	}

	client := newRetryingClient(inner, 3, noopLogger{}, func(int) time.Duration { return 0 })

	got, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "hello"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "success" {
		t.Fatalf("Complete() = %q, want success", got)
	}
	if inner.calls != 2 {
		t.Fatalf("calls = %d, want 2", inner.calls)
	}
}

func TestRetryingClientStopsAfterMaxRetries(t *testing.T) {
	inner := &stubClient{
		errors: []error{
			&LLMError{StatusCode: 500, Message: "e1", Retryable: true},
			&LLMError{StatusCode: 500, Message: "e2", Retryable: true},
			&LLMError{StatusCode: 500, Message: "e3", Retryable: true},
			&LLMError{StatusCode: 500, Message: "e4", Retryable: true},
		},
	}

	client := newRetryingClient(inner, 3, noopLogger{}, func(int) time.Duration { return 0 })

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "hello"})
	if err == nil {
		t.Fatal("Complete() error = nil, want final retryable error")
	}
	if inner.calls != 4 {
		t.Fatalf("calls = %d, want 4", inner.calls)
	}
}

func TestRetryingClientDoesNotRetry401(t *testing.T) {
	inner := &stubClient{
		errors: []error{&LLMError{StatusCode: 401, Message: "unauthorized", Retryable: false}},
	}

	client := newRetryingClient(inner, 3, noopLogger{}, func(int) time.Duration { return 0 })

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "hello"})
	if err == nil {
		t.Fatal("Complete() error = nil, want non-retryable error")
	}
	if inner.calls != 1 {
		t.Fatalf("calls = %d, want 1", inner.calls)
	}
}

func TestRetryingClientStopsWhenContextCancelled(t *testing.T) {
	inner := &stubClient{
		errors: []error{&LLMError{StatusCode: 500, Message: "retry me", Retryable: true}},
	}

	client := newRetryingClient(inner, 3, noopLogger{}, func(int) time.Duration { return 50 * time.Millisecond })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Complete(ctx, CompletionRequest{UserMsg: "hello"})
	if err == nil {
		t.Fatal("Complete() error = nil, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if inner.calls != 1 {
		t.Fatalf("calls = %d, want 1", inner.calls)
	}
}

var _ logpkg.Logger = noopLogger{}

package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMockClientReturnsResponsesInOrder(t *testing.T) {
	client := NewMockClient("first", "second")

	first, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "one"})
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	second, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "two"})
	if err != nil {
		t.Fatalf("second Complete() error = %v", err)
	}

	if first != "first" || second != "second" {
		t.Fatalf("responses = %q, %q; want first, second", first, second)
	}
}

func TestMockClientErrorsWhenResponsesExhausted(t *testing.T) {
	client := NewMockClient("only")

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "one"})
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}

	_, err = client.Complete(context.Background(), CompletionRequest{UserMsg: "two"})
	if err == nil {
		t.Fatal("second Complete() error = nil, want exhaustion error")
	}
	if !strings.Contains(err.Error(), "no more responses") {
		t.Fatalf("error = %v, want no more responses message", err)
	}
}

func TestMockClientReturnsConfiguredErrorOnNthCall(t *testing.T) {
	targetErr := errors.New("boom")
	client := NewMockClient("first", "second").WithErrors(nil, targetErr)

	_, err := client.Complete(context.Background(), CompletionRequest{UserMsg: "one"})
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}

	_, err = client.Complete(context.Background(), CompletionRequest{UserMsg: "two"})
	if !errors.Is(err, targetErr) {
		t.Fatalf("error = %v, want %v", err, targetErr)
	}
}

func TestMockClientRecordsCalls(t *testing.T) {
	client := NewMockClient("first", "second")
	req1 := CompletionRequest{UserMsg: "one"}
	req2 := CompletionRequest{UserMsg: "two"}

	_, _ = client.Complete(context.Background(), req1)
	_, _ = client.Complete(context.Background(), req2)

	if client.CallCount() != 2 {
		t.Fatalf("CallCount() = %d, want 2", client.CallCount())
	}
	calls := client.Calls()
	if len(calls) != 2 {
		t.Fatalf("len(Calls()) = %d, want 2", len(calls))
	}
	if calls[0].UserMsg != "one" || calls[1].UserMsg != "two" {
		t.Fatalf("Calls() = %+v, want one/two", calls)
	}
}

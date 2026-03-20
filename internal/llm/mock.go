package llm

import (
	"context"
	"fmt"
	"sync"
)

type MockClient struct {
	responses []string
	errors    []error
	calls     []CompletionRequest
	mu        sync.Mutex
}

func NewMockClient(responses ...string) *MockClient {
	cloned := append([]string(nil), responses...)
	return &MockClient{responses: cloned}
}

func (m *MockClient) WithErrors(errs ...error) *MockClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append([]error(nil), errs...)
	return m
}

func (m *MockClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	_ = ctx

	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, req)
	callIndex := len(m.calls) - 1

	if callIndex < len(m.errors) && m.errors[callIndex] != nil {
		return "", m.errors[callIndex]
	}
	if callIndex >= len(m.responses) {
		return "", fmt.Errorf("MockClient: no more responses configured (call %d)", callIndex)
	}

	return m.responses[callIndex], nil
}

func (m *MockClient) Calls() []CompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]CompletionRequest(nil), m.calls...)
}

func (m *MockClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

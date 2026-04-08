package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

type mockExploreClient struct {
	response string
	err      error
	lastReq  *llm.CompletionRequest
}

func (m *mockExploreClient) Complete(_ context.Context, req *llm.CompletionRequest) (string, error) {
	m.lastReq = req
	return m.response, m.err
}

func TestExploreAgent_Run_ReadFile(t *testing.T) {
	idx := store.FileIndex{
		"handler.go": &store.FileEntry{
			Path: "handler.go", Language: "go",
			Functions: []*store.FunctionDecl{{
				Name: "Handle", Signature: "func Handle(w ResponseWriter, r *Request)",
				LineStart: 10, LineEnd: 40, Exported: true, Path: "handler.go",
				Src: "func Handle(w ResponseWriter, r *Request) { /* ... */ }",
			}},
		},
	}
	client := &mockExploreClient{
		response: `{"requests":[{"type":"read_file","target":"handler.go","reason":"entry point"}]}`,
	}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, Temperature: 0.2, MaxRequests: 5, Language: "English",
	})

	result, err := ag.Run(context.Background(), idx, nil, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(result.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(result.Requests))
	}
	if result.Files["handler.go"] == nil {
		t.Error("expected handler.go resolved")
	}
	if client.lastReq == nil || !strings.Contains(client.lastReq.SystemMsg, "importance_definition") {
		t.Error("system prompt should contain importance_definition section")
	}
}

func TestExploreAgent_Run_ReadFunction(t *testing.T) {
	idx := store.FileIndex{
		"svc.go": &store.FileEntry{
			Path: "svc.go", Language: "go",
			Functions: []*store.FunctionDecl{{
				Name: "Process", Signature: "func (s *Service) Process(ctx Context) error",
				LineStart: 15, LineEnd: 60, Exported: true, Receiver: "Service", Path: "svc.go",
				Src: "func (s *Service) Process(ctx Context) error { return nil }",
			}},
		},
	}
	client := &mockExploreClient{
		response: `{"requests":[{"type":"read_function","target":"svc.go#Service#Process","reason":"core service"}]}`,
	}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, MaxRequests: 5, Language: "English",
	})

	result, err := ag.Run(context.Background(), idx, nil, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Functions["svc.go#Service#Process"] == nil {
		t.Error("expected svc.go#Service#Process resolved")
	}
}

func TestExploreAgent_Run_UnsupportedType(t *testing.T) {
	client := &mockExploreClient{
		response: `{"requests":[{"type":"rag_search","target":"q","reason":"test"}]}`,
	}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, MaxRequests: 5, Language: "English",
	})

	_, err := ag.Run(context.Background(), store.FileIndex{}, nil, nil)
	if err == nil {
		t.Error("expected error for unsupported request type")
	}
}

func TestExploreAgent_Run_UnknownTarget(t *testing.T) {
	idx := store.FileIndex{
		"handler.go": &store.FileEntry{Path: "handler.go", Language: "go"},
	}
	client := &mockExploreClient{
		response: `{"requests":[{"type":"read_file","target":"nonexistent.go","reason":"test"}]}`,
	}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, MaxRequests: 5, Language: "English",
	})

	result, err := ag.Run(context.Background(), idx, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Error("unknown target should be skipped silently")
	}
}

func TestExploreAgent_Run_RequestTruncation(t *testing.T) {
	client := &mockExploreClient{
		response: `{"requests":[
			{"type":"read_file","target":"a.go","reason":"1"},
			{"type":"read_file","target":"b.go","reason":"2"},
			{"type":"read_file","target":"c.go","reason":"3"},
			{"type":"read_file","target":"d.go","reason":"4"},
			{"type":"read_file","target":"e.go","reason":"5"},
			{"type":"read_file","target":"f.go","reason":"6"}
		]}`,
	}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, MaxRequests: 5, Language: "English",
	})

	result, err := ag.Run(context.Background(), store.FileIndex{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Requests) != 5 {
		t.Errorf("expected 5 after truncation, got %d", len(result.Requests))
	}
}

func TestExploreAgent_buildPrompt(t *testing.T) {
	client := &mockExploreClient{}
	ag := NewExploreAgent(client, &ExploreConfig{
		Model: "test", MaxTokens: 4096, Language: "English",
	})

	req, err := ag.buildPrompt("some skeleton text")
	if err != nil {
		t.Fatalf("buildPrompt failed: %v", err)
	}
	if req.Model != "test" {
		t.Errorf("expected model test, got %s", req.Model)
	}
	if !strings.Contains(req.UserMsg, "some skeleton text") {
		t.Error("user msg should contain skeleton")
	}
}

func TestExploreAgent_parseResponse(t *testing.T) {
	ag := NewExploreAgent(nil, &ExploreConfig{})

	resp, err := ag.parseResponse(`{"requests":[{"type":"read_file","target":"a.go","reason":"test"}]}`)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}
	if len(resp.Requests) != 1 || resp.Requests[0].Type != "read_file" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

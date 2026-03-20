package llm

import "context"

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

package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
	configpkg "github.com/scalaview/wikismit/internal/config"
)

type LLMError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *LLMError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.StatusCode == 0 {
		return e.Message
	}
	return fmt.Sprintf("llm error (%d): %s", e.StatusCode, e.Message)
}

type openAIClient struct {
	c            *openai.Client
	defaultModel string
	timeout      time.Duration
}

func NewClient(cfg configpkg.LLMConfig) (Client, error) {
	clientCfg := openai.DefaultConfig(cfg.APIKey())
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}

	return &openAIClient{
		c:            openai.NewClientWithConfig(clientCfg),
		defaultModel: cfg.AgentModel,
		timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
	}, nil
}

func (c *openAIClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	requestCtx := ctx
	var cancel context.CancelFunc
	if c.timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	resp, err := c.c.CreateChatCompletion(requestCtx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: req.SystemMsg},
			{Role: openai.ChatMessageRoleUser, Content: req.UserMsg},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return "", normalizeLLMError(err)
	}
	if len(resp.Choices) == 0 {
		return "", &LLMError{Message: "empty completion response", Retryable: false}
	}

	return resp.Choices[0].Message.Content, nil
}

func normalizeLLMError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &LLMError{Message: err.Error(), Retryable: true}
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		status := apiErr.HTTPStatusCode
		return &LLMError{
			StatusCode: status,
			Message:    apiErr.Message,
			Retryable:  isRetryableStatus(status),
		}
	}

	return &LLMError{Message: err.Error(), Retryable: false}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable:
		return true
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return false
	default:
		return false
	}
}

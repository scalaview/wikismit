package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
	configpkg "github.com/scalaview/wikismit/internal/config"
	logpkg "github.com/scalaview/wikismit/internal/log"
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
	baseURL      string
	logger       logpkg.Logger
}

func NewClient(cfg configpkg.LLMConfig) (Client, error) {
	return newClient(cfg, nil)
}

func newClient(cfg configpkg.LLMConfig, logger logpkg.Logger) (Client, error) {
	clientCfg := openai.DefaultConfig(cfg.APIKey())
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}
	if logger == nil {
		logger = logpkg.New(false)
	}

	return &openAIClient{
		c:            openai.NewClientWithConfig(clientCfg),
		defaultModel: cfg.AgentModel,
		timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
		baseURL:      clientCfg.BaseURL,
		logger:       logger,
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

	start := time.Now()
	c.logger.Debug("starting chat completion request",
		"model", model,
		"max_tokens", req.MaxTokens,
		"timeout_seconds", int(c.timeout/time.Second),
		"base_url", c.baseURL,
		"user_prompt_chars", len(req.UserMsg),
		"estimated_user_prompt_tokens", estimatePromptTokens(len(req.UserMsg)),
		"started_at", start.Format(time.RFC3339Nano),
	)

	resp, err := c.c.CreateChatCompletion(requestCtx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: req.SystemMsg},
			{Role: openai.ChatMessageRoleUser, Content: req.UserMsg},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	finished := time.Now()
	logCompletion := func(err error) {
		fields := []any{
			"model", model,
			"max_tokens", req.MaxTokens,
			"timeout_seconds", int(c.timeout / time.Second),
			"base_url", c.baseURL,
			"user_prompt_chars", len(req.UserMsg),
			"estimated_user_prompt_tokens", estimatePromptTokens(len(req.UserMsg)),
			"started_at", start.Format(time.RFC3339Nano),
			"finished_at", finished.Format(time.RFC3339Nano),
			"duration_ms", finished.Sub(start).Milliseconds(),
		}
		if err != nil {
			fields = append(fields, "error_type", fmt.Sprintf("%T", err))
		}
		c.logger.Debug("finished chat completion request", fields...)
	}
	if err != nil {
		normalizedErr := normalizeLLMError(err)
		logCompletion(normalizedErr)
		return "", normalizedErr
	}
	if len(resp.Choices) == 0 {
		err := &LLMError{Message: "empty completion response", Retryable: false}
		logCompletion(err)
		return "", err
	}
	logCompletion(nil)

	return resp.Choices[0].Message.Content, nil
}

func estimatePromptTokens(charCount int) int {
	if charCount <= 0 {
		return 0
	}
	return (charCount + 3) / 4
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

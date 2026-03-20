package llm

import (
	"context"
	"errors"
	"math/rand"
	"time"

	logpkg "github.com/scalaview/wikismit/internal/log"
)

type retryingClient struct {
	inner      Client
	maxRetries int
	logger     logpkg.Logger
	backoff    func(int) time.Duration
}

func NewRetryingClient(inner Client, maxRetries int, logger logpkg.Logger) Client {
	return newRetryingClient(inner, maxRetries, logger, backoffDuration)
}

func newRetryingClient(inner Client, maxRetries int, logger logpkg.Logger, backoff func(int) time.Duration) Client {
	if logger == nil {
		logger = logpkg.New(false)
	}
	if backoff == nil {
		backoff = backoffDuration
	}
	return &retryingClient{
		inner:      inner,
		maxRetries: maxRetries,
		logger:     logger,
		backoff:    backoff,
	}
}

func (c *retryingClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		result, err := c.inner.Complete(ctx, req)
		if err == nil {
			return result, nil
		}
		lastErr = err

		var llmErr *LLMError
		if !errors.As(err, &llmErr) || !llmErr.Retryable || attempt == c.maxRetries {
			return "", err
		}

		wait := c.backoff(attempt)
		c.logger.Debug("retrying llm request", "attempt", attempt+1, "wait", wait, "error", err.Error())

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return "", ctx.Err()
		case <-timer.C:
		}
	}

	return "", lastErr
}

func backoffDuration(attempt int) time.Duration {
	base := 2 * time.Second
	duration := base * time.Duration(1<<attempt)
	if duration > 30*time.Second {
		duration = 30 * time.Second
	}

	jitterRange := int64(duration) / 5
	if jitterRange == 0 {
		return duration
	}
	jitter := rand.Int63n(2*jitterRange+1) - jitterRange
	return duration + time.Duration(jitter)
}

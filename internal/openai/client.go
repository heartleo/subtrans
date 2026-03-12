// Package openai provides an OpenAI-compatible chat completions client.
package openai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	goopenai "github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/translator"
)

// ErrMaxRetriesExceeded is returned when all retry attempts are exhausted.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// ClientOption configures a Client.
type ClientOption func(*Client)

// Client wraps the OpenAI SDK client with retry logic.
type Client struct {
	inner          *goopenai.Client
	model          string
	temperature    float64
	maxRetries     int
	initialBackoff time.Duration
}

// NewClient creates a new Client from the given config.
func NewClient(cfg config.Config, opts ...ClientOption) *Client {
	clientCfg := goopenai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}

	c := &Client{
		inner:          goopenai.NewClientWithConfig(clientCfg),
		model:          cfg.Model,
		temperature:    cfg.Temperature,
		maxRetries:     cfg.MaxRetries,
		initialBackoff: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	slog.Info("openai client initialized", "model", c.model, "base_url", clientCfg.BaseURL)

	return c
}

// Complete sends messages to the chat completions endpoint and returns the
// assistant's response text. Retries on HTTP 429 and 5xx.
func (c *Client) Complete(ctx context.Context, messages []translator.Message) (string, error) {
	apiMessages := make([]goopenai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		apiMessages[i] = goopenai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}

	req := goopenai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    apiMessages,
		Temperature: float32(c.temperature),
	}

	req.ResponseFormat = &goopenai.ChatCompletionResponseFormat{
		Type: goopenai.ChatCompletionResponseFormatTypeJSONObject,
	}

	// Count request tokens.
	var reqTokens int
	for _, m := range messages {
		reqTokens += countTokens(c.model, m.Content)
	}

	backoff := c.initialBackoff

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.inner.CreateChatCompletion(ctx, req)
		if err == nil {
			if len(resp.Choices) == 0 {
				return "", errors.New("empty choices in response")
			}

			content := resp.Choices[0].Message.Content
			respTokens := countTokens(c.model, content)
			slog.Info("token usage", "request_tokens", reqTokens, "response_tokens", respTokens, "total_tokens", reqTokens+respTokens)

			return content, nil
		}

		var apiErr *goopenai.APIError
		if errors.As(err, &apiErr) && isRetryable(apiErr.HTTPStatusCode) && attempt < c.maxRetries {
			slog.Warn("API error, retrying", "status", apiErr.HTTPStatusCode, "attempt", attempt+1)

			if backoff > 0 {
				select {
				case <-ctx.Done():
					return "", fmt.Errorf("context cancelled: %w", ctx.Err())
				case <-time.After(backoff):
					backoff *= 2
				}
			}

			continue
		}

		return "", fmt.Errorf("chat completion: %w", err)
	}

	return "", ErrMaxRetriesExceeded
}

func isRetryable(code int) bool {
	return code == 429 || (code >= 500 && code < 600)
}

// countTokens returns the token count for the given text using the model's tokenizer.
// Returns 0 if the model is not supported or counting fails.
func countTokens(model string, text string) int {
	enc, err := tokenizer.ForModel(tokenizer.Model(model))
	if err != nil {
		return 0
	}
	count, err := enc.Count(text)
	if err != nil {
		return 0
	}
	return count
}

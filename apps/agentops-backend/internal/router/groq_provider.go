// Package router provides LLM provider implementations.
package router

import (
	"context"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// GroqProvider implements Provider against Groq's OpenAI-compatible API.
// It uses the official OpenAI Go SDK with a custom base URL.
type GroqProvider struct {
	client *openai.Client
	model  string
}

// NewGroqProvider constructs a GroqProvider from the GROQ_API_KEY environment
// variable. It fails fast if the key is missing.
func NewGroqProvider(model string) (*GroqProvider, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("groq: GROQ_API_KEY is required")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.groq.com/openai/v1"

	return &GroqProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}, nil
}

// Complete sends a chat completion request to Groq and returns the result.
func (g *GroqProvider) Complete(ctx context.Context, modelID, prompt string) (*Completion, error) {
	// The router passes the full model ID (e.g. "groq/openai/gpt-oss-20b").
	// Strip the "groq/" prefix so that Groq receives the raw model name.
	model := strings.TrimPrefix(modelID, "groq/")
	if model == "" {
		model = g.model
	}

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("groq: completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("groq: empty response")
	}

	choice := resp.Choices[0]
	return &Completion{
		Text:         choice.Message.Content,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}
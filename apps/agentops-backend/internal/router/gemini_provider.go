// Package router provides LLM provider implementations.
package router

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider against the native Gemini API.
// It uses the official google.golang.org/genai SDK, which accepts the new
// AQ. authentication keys from Google AI Studio.
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider constructs a GeminiProvider from GEMINI_API_KEY.
func NewGeminiProvider(model string) (*GeminiProvider, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: GEMINI_API_KEY is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Complete sends a content generation request to Gemini and returns the result.
func (g *GeminiProvider) Complete(ctx context.Context, modelID, prompt string) (*Completion, error) {
	// Strip the "gemini/" prefix so Gemini receives the raw model name.
	model := strings.TrimPrefix(modelID, "gemini/")
	if model == "" {
		model = g.model
	}

	// The native API uses "contents" with "parts", not "messages".
	result, err := g.client.Models.GenerateContent(
		ctx,
		model,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("gemini: completion failed: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}

	// The response text is in Candidates[0].Content.Parts[0].Text.
	text := result.Candidates[0].Content.Parts[0].Text
	if text == "" {
		return nil, fmt.Errorf("gemini: empty text in response")
	}

	// Token usage: the native API reports UsageMetadata.
	var inputTokens, outputTokens int
	if result.UsageMetadata != nil {
		inputTokens = int(result.UsageMetadata.PromptTokenCount)
		outputTokens = int(result.UsageMetadata.CandidatesTokenCount)
	}

	return &Completion{
		Text:         text,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}
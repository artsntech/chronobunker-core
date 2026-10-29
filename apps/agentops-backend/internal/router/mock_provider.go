package router

import (
	"context"
	"fmt"
)

// MockProvider is a development provider that returns canned responses.
// It is replaced by real provider adapters in Phase 4.
type MockProvider struct{}

func (m *MockProvider) Complete(ctx context.Context, modelID, prompt string) (*Completion, error) {
	return &Completion{
		Text:         fmt.Sprintf("[mock %s] received: %s", modelID, prompt),
		InputTokens:  len(prompt) / 4,
		OutputTokens: 20,
	}, nil
}
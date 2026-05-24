package copilot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopilotModel_SupportsAnthropicMessages(t *testing.T) {
	tests := []struct {
		name     string
		model    CopilotModel
		expected bool
	}{
		{
			name:     "has v1/messages endpoint",
			model:    CopilotModel{SupportedEndpoints: []string{"/v1/messages", "/chat/completions"}},
			expected: true,
		},
		{
			name:     "no v1/messages endpoint",
			model:    CopilotModel{SupportedEndpoints: []string{"/chat/completions"}},
			expected: false,
		},
		{
			name:     "nil endpoints",
			model:    CopilotModel{SupportedEndpoints: nil},
			expected: false,
		},
		{
			name:     "empty endpoints",
			model:    CopilotModel{SupportedEndpoints: []string{}},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.model.SupportsAnthropicMessages())
		})
	}
}

func TestCopilotModel_HasAdaptiveThinking(t *testing.T) {
	m := CopilotModel{
		Capabilities: CopilotModelCapabilities{
			Supports: CopilotModelSupports{
				AdaptiveThinking: true,
				ReasoningEffort:  []string{"low", "medium", "high"},
			},
		},
	}
	assert.True(t, m.HasAdaptiveThinking())
	assert.Equal(t, []string{"low", "medium", "high"}, m.ReasoningEfforts())

	m2 := CopilotModel{
		Capabilities: CopilotModelCapabilities{
			Supports: CopilotModelSupports{
				AdaptiveThinking: false,
			},
		},
	}
	assert.False(t, m2.HasAdaptiveThinking())
}

func TestCopilotModel_MaxThinkingBudget(t *testing.T) {
	m := CopilotModel{
		Capabilities: CopilotModelCapabilities{
			Supports: CopilotModelSupports{
				MaxThinkingBudget: 10000,
			},
		},
	}
	assert.Equal(t, 10000, m.MaxThinkingBudget())
}

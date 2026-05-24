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

func TestBuildModelInfoMap(t *testing.T) {
	models := []CopilotModel{
		{
			ID:                 "gpt-4.1",
			SupportedEndpoints: []string{"/chat/completions"},
			Capabilities: CopilotModelCapabilities{
				Supports: CopilotModelSupports{
					ReasoningEffort: []string{"low", "medium", "high"},
				},
			},
		},
		{
			ID:                 "claude-sonnet-4-20250514",
			SupportedEndpoints: []string{"/v1/messages", "/chat/completions"},
			Capabilities: CopilotModelCapabilities{
				Supports: CopilotModelSupports{
					AdaptiveThinking: true,
					ReasoningEffort:  []string{"low", "medium", "high"},
				},
			},
		},
	}
	infoMap := BuildModelInfoMap(models)

	assert.NotNil(t, infoMap["gpt-4.1"])
	assert.False(t, infoMap["gpt-4.1"].SupportsAnthropicMessages)
	assert.Equal(t, []string{"low", "medium", "high"}, infoMap["gpt-4.1"].ReasoningEfforts)

	assert.NotNil(t, infoMap["claude-sonnet-4-20250514"])
	assert.True(t, infoMap["claude-sonnet-4-20250514"].SupportsAnthropicMessages)
	assert.True(t, infoMap["claude-sonnet-4-20250514"].SupportsAdaptiveThinking)
}

func TestGenerateVariants_OpenAIReasoning(t *testing.T) {
	info := &CopilotModelInfo{
		ModelID:          "gpt-4.1",
		ReasoningEfforts: []string{"low", "medium", "high"},
	}
	variants := GenerateVariants(info)
	assert.Len(t, variants, 3)
	assert.Equal(t, "gpt-4.1:low", variants[0].ModelID)
	assert.Equal(t, "reasoning", variants[0].Type)
	assert.Equal(t, "low", variants[0].Effort)
	assert.Equal(t, "gpt-4.1:high", variants[2].ModelID)
}

func TestGenerateVariants_AnthropicAdaptive(t *testing.T) {
	info := &CopilotModelInfo{
		ModelID:                  "claude-sonnet-4-20250514",
		SupportsAdaptiveThinking: true,
		ReasoningEfforts:         []string{"low", "medium", "high"},
	}
	variants := GenerateVariants(info)
	assert.Len(t, variants, 3)
	assert.Equal(t, "claude-sonnet-4-20250514:low", variants[0].ModelID)
	assert.Equal(t, "adaptive", variants[0].Type)
	assert.Equal(t, "medium", variants[1].Effort)
}

func TestGenerateVariants_AnthropicBudget(t *testing.T) {
	info := &CopilotModelInfo{
		ModelID:           "claude-opus-4-20250514",
		MaxThinkingBudget: 10000,
	}
	variants := GenerateVariants(info)
	assert.Len(t, variants, 2)
	assert.Equal(t, "claude-opus-4-20250514:high", variants[0].ModelID)
	assert.Equal(t, 5000, variants[0].BudgetTokens)
	assert.Equal(t, "claude-opus-4-20250514:max", variants[1].ModelID)
	assert.Equal(t, 9999, variants[1].BudgetTokens)
}

func TestGenerateVariants_NoCapabilities(t *testing.T) {
	info := &CopilotModelInfo{
		ModelID: "basic-model",
	}
	variants := GenerateVariants(info)
	assert.Empty(t, variants)
}

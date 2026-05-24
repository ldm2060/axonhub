package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

// CopilotModelsResponse is the response from the Copilot /models API.
type CopilotModelsResponse struct {
	Data []CopilotModel `json:"data"`
}

// CopilotModel represents a single model from the Copilot /models API.
type CopilotModel struct {
	ID                 string                   `json:"id"`
	Name               string                   `json:"name"`
	ModelPickerEnabled bool                     `json:"model_picker_enabled"`
	Version            string                   `json:"version"`
	SupportedEndpoints []string                 `json:"supported_endpoints"`
	Policy             CopilotModelPolicy       `json:"policy"`
	Capabilities       CopilotModelCapabilities `json:"capabilities"`
}

type CopilotModelPolicy struct {
	State string `json:"state"`
}

type CopilotModelCapabilities struct {
	Family   string               `json:"family"`
	Limits   CopilotModelLimits   `json:"limits"`
	Supports CopilotModelSupports `json:"supports"`
}

type CopilotModelLimits struct {
	MaxContextWindowTokens int `json:"max_context_window_tokens"`
	MaxOutputTokens        int `json:"max_output_tokens"`
	MaxPromptTokens        int `json:"max_prompt_tokens"`
}

type CopilotModelSupports struct {
	Vision            bool     `json:"vision"`
	ToolCalls         bool     `json:"tool_calls"`
	Streaming         bool     `json:"streaming"`
	StructuredOutputs bool     `json:"structured_outputs"`
	AdaptiveThinking  bool     `json:"adaptive_thinking"`
	ReasoningEffort   []string `json:"reasoning_effort"`
	MaxThinkingBudget int      `json:"max_thinking_budget"`
}

func (m CopilotModel) SupportsAnthropicMessages() bool {
	return slices.Contains(m.SupportedEndpoints, "/v1/messages")
}

func (m CopilotModel) HasAdaptiveThinking() bool {
	return m.Capabilities.Supports.AdaptiveThinking
}

func (m CopilotModel) ReasoningEfforts() []string {
	return m.Capabilities.Supports.ReasoningEffort
}

func (m CopilotModel) MaxThinkingBudget() int {
	return m.Capabilities.Supports.MaxThinkingBudget
}

// CopilotModelInfo is a flattened summary of a CopilotModel, pre-computed for
// fast lookups during request transformation.
type CopilotModelInfo struct {
	ModelID                   string
	SupportedEndpoints        []string
	SupportsAnthropicMessages bool
	SupportsAdaptiveThinking  bool
	ReasoningEfforts          []string
	MaxThinkingBudget         int
	MaxContextWindowTokens    int
	MaxOutputTokens           int
	SupportsVision            bool
	SupportsToolCalls         bool
	SupportsStreaming         bool
	SupportsStructuredOutputs bool
	IsOpus                    bool
}

// ModelVariant represents a derived variant of a base model (e.g. "gpt-4.1:high").
type ModelVariant struct {
	ModelID      string
	DisplayName  string
	Type         string // "reasoning", "adaptive", or "budget"
	Effort       string // "low", "medium", "high", "max"
	BudgetTokens int    // for budget-based variants
}

// BuildModelInfoMap builds a lookup table from model ID to CopilotModelInfo.
func BuildModelInfoMap(models []CopilotModel) map[string]*CopilotModelInfo {
	m := make(map[string]*CopilotModelInfo, len(models))
	for _, model := range models {
		info := &CopilotModelInfo{
			ModelID:                   model.ID,
			SupportedEndpoints:        model.SupportedEndpoints,
			SupportsAnthropicMessages: model.SupportsAnthropicMessages(),
			SupportsAdaptiveThinking:  model.HasAdaptiveThinking(),
			ReasoningEfforts:          model.ReasoningEfforts(),
			MaxThinkingBudget:         model.MaxThinkingBudget(),
			MaxContextWindowTokens:    model.Capabilities.Limits.MaxContextWindowTokens,
			MaxOutputTokens:           model.Capabilities.Limits.MaxOutputTokens,
			SupportsVision:            model.Capabilities.Supports.Vision,
			SupportsToolCalls:         model.Capabilities.Supports.ToolCalls,
			SupportsStreaming:         model.Capabilities.Supports.Streaming,
			SupportsStructuredOutputs: model.Capabilities.Supports.StructuredOutputs,
			IsOpus:                    strings.Contains(model.ID, "opus"),
		}
		m[model.ID] = info
	}
	return m
}

// GenerateVariants produces model variants for a given CopilotModelInfo.
// Models with reasoning efforts get per-effort variants (type "reasoning" or
// "adaptive"). Models without efforts but with a thinking budget get "high" and
// "max" budget variants.
func GenerateVariants(info *CopilotModelInfo) []ModelVariant {
	var variants []ModelVariant

	if len(info.ReasoningEfforts) > 0 {
		for _, effort := range info.ReasoningEfforts {
			variantType := "reasoning"
			if info.SupportsAdaptiveThinking {
				variantType = "adaptive"
			}
			variants = append(variants, ModelVariant{
				ModelID:     info.ModelID + ":" + effort,
				DisplayName: effort,
				Type:        variantType,
				Effort:      effort,
			})
		}
	} else if info.MaxThinkingBudget > 0 {
		variants = append(variants,
			ModelVariant{
				ModelID:      info.ModelID + ":high",
				DisplayName:  "high",
				Type:         "budget",
				Effort:       "high",
				BudgetTokens: info.MaxThinkingBudget / 2,
			},
			ModelVariant{
				ModelID:      info.ModelID + ":max",
				DisplayName:  "max",
				Type:         "budget",
				Effort:       "max",
				BudgetTokens: info.MaxThinkingBudget - 1,
			},
		)
	}

	return variants
}

// FetchModels fetches the available models from the Copilot /models API.
func FetchModels(ctx context.Context, httpClient *httpclient.HttpClient, baseURL, accessToken string) ([]CopilotModel, error) {
	url := baseURL + ModelsEndpoint

	req := &httpclient.Request{
		Method: http.MethodGet,
		URL:    url,
		Headers: http.Header{
			"Authorization": []string{"Bearer " + accessToken},
			"Accept":        []string{"application/json"},
		},
	}

	resp, err := httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch copilot models: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch copilot models: status %d", resp.StatusCode)
	}

	var modelsResp CopilotModelsResponse
	if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse copilot models response: %w", err)
	}

	// Filter: only enabled models that are not disabled by policy
	filtered := make([]CopilotModel, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ModelPickerEnabled && m.Policy.State != "disabled" {
			filtered = append(filtered, m)
		}
	}

	return filtered, nil
}

// ModelIDs extracts model IDs from a list of CopilotModel.
func ModelIDs(models []CopilotModel) []string {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	return ids
}

// FetchModelsWithInfo fetches models and returns both the model IDs and a
// CopilotModelInfo lookup table for routing decisions.
func FetchModelsWithInfo(ctx context.Context, httpClient *httpclient.HttpClient, baseURL, accessToken string) ([]string, map[string]*CopilotModelInfo, error) {
	models, err := FetchModels(ctx, httpClient, baseURL, accessToken)
	if err != nil {
		return nil, nil, err
	}

	return ModelIDs(models), BuildModelInfoMap(models), nil
}

// FetchModelInfoMap fetches models and returns only the CopilotModelInfo lookup table.
func FetchModelInfoMap(ctx context.Context, httpClient *httpclient.HttpClient, baseURL, accessToken string) (map[string]*CopilotModelInfo, error) {
	models, err := FetchModels(ctx, httpClient, baseURL, accessToken)
	if err != nil {
		return nil, err
	}

	return BuildModelInfoMap(models), nil
}

const fetchModelsTimeout = 5 * time.Second

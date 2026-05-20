package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

// CopilotModelsResponse is the response from the Copilot /models API.
type CopilotModelsResponse struct {
	Data []CopilotModel `json:"data"`
}

// CopilotModel represents a single model from the Copilot /models API.
type CopilotModel struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	ModelPickerEnabled bool     `json:"model_picker_enabled"`
	Version            string   `json:"version"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	Policy             struct {
		State string `json:"state"`
	} `json:"policy"`
	Capabilities struct {
		Family string `json:"family"`
		Limits struct {
			MaxContextWindowTokens int `json:"max_context_window_tokens"`
			MaxOutputTokens        int `json:"max_output_tokens"`
			MaxPromptTokens        int `json:"max_prompt_tokens"`
		} `json:"limits"`
		Supports struct {
			Vision            bool     `json:"vision"`
			ToolCalls         bool     `json:"tool_calls"`
			Streaming         bool     `json:"streaming"`
			StructuredOutputs bool     `json:"structured_outputs"`
			AdaptiveThinking  bool     `json:"adaptive_thinking"`
			ReasoningEffort   []string `json:"reasoning_effort"`
		} `json:"supports"`
	} `json:"capabilities"`
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

const fetchModelsTimeout = 5 * time.Second

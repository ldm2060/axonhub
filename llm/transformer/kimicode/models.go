package kimicode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// Model is the normalized Kimi Code /models record persisted in OAuth
// credentials. It aliases the shared serialized representation deliberately.
type Model = oauth.KimiCodeModel

// ModelsResponse accepts both the current upstream envelope (`data`) and the
// legacy `models` alias observed on older servers.
type ModelsResponse struct {
	Data   []rawModel `json:"data"`
	Models []rawModel `json:"models"`
}

// rawModel decodes one /models entry. Current upstream entries use the nested
// shape (limit.context, modalities.input, reasoning.type/effort); older
// servers served the flat legacy shape. Both decode into this struct and
// reasoningEffortBlock is the effort block of the /models reasoning object
// ({valid: [...], default: "..."}).
type reasoningEffortBlock struct {
	Valid   []string `json:"valid"`
	Default string   `json:"default"`
}

// toModel prefers the nested fields, falling back to the legacy ones.
type rawModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Limit       *struct {
		Context int64 `json:"context"`
	} `json:"limit"`
	ContextLength        int64                       `json:"context_length"`
	Protocol             string                      `json:"protocol"`
	SupportsReasoning    bool                        `json:"supports_reasoning"`
	SupportsImageIn      bool                        `json:"supports_image_in"`
	SupportsVideoIn      bool                        `json:"supports_video_in"`
	SupportsToolUse      *bool                       `json:"supports_tool_use"`
	SupportsThinkingType string                      `json:"supports_thinking_type"`
	ThinkEfforts         *oauth.KimiCodeThinkEfforts `json:"think_efforts"`
	Modalities           *struct {
		Input []string `json:"input"`
	} `json:"modalities"`
	Reasoning *struct {
		Type   string                `json:"type"`
		Effort *reasoningEffortBlock `json:"effort"`
	} `json:"reasoning"`
}

func ModelsURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return baseURL + ModelsPath
}

func FetchModels(ctx context.Context, httpClient *httpclient.HttpClient, baseURL, accessToken string, identity Identity) ([]Model, error) {
	if httpClient == nil {
		return nil, errors.New("http client is nil")
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("Kimi Code access token is empty")
	}
	headers, err := BuildIdentityHeaders(identity)
	if err != nil {
		return nil, fmt.Errorf("build Kimi Code model request: %w", err)
	}
	headers.Set("Accept", "application/json")
	request := &httpclient.Request{
		Method: http.MethodGet, URL: ModelsURL(baseURL), Headers: headers,
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: accessToken},
	}
	response, err := httpClient.Do(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("fetch Kimi Code models: %w", err)
	}
	var decoded ModelsResponse
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return nil, fmt.Errorf("decode Kimi Code models: %w", err)
	}
	rawModels := decoded.Data
	if len(rawModels) == 0 {
		rawModels = decoded.Models
	}
	if len(rawModels) == 0 {
		return nil, errors.New("Kimi Code models response is empty")
	}
	models := make([]Model, 0, len(rawModels))
	for _, raw := range rawModels {
		model, err := toModel(raw)
		if err != nil {
			return nil, err
		}
		if err := ValidateModel(model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, nil
}

// toModel normalizes one raw /models entry, mirroring the official CLI's
// toModelInfo: the nested limit/modalities/reasoning fields win and the flat
// legacy fields act as fallbacks for older servers.
func toModel(raw rawModel) (Model, error) {
	if strings.TrimSpace(raw.ID) == "" {
		return Model{}, errors.New("Kimi Code model missing id")
	}
	model := Model{
		ID:                   raw.ID,
		DisplayName:          strings.TrimSpace(raw.DisplayName),
		Protocol:             normalizeModelProtocol(raw.Protocol),
		SupportsReasoning:    raw.SupportsReasoning,
		SupportsImageIn:      raw.SupportsImageIn,
		SupportsVideoIn:      raw.SupportsVideoIn,
		SupportsToolUse:      true,
		SupportsThinkingType: raw.SupportsThinkingType,
		ThinkEfforts:         raw.ThinkEfforts,
	}
	if raw.SupportsToolUse != nil {
		model.SupportsToolUse = *raw.SupportsToolUse
	}
	if raw.Limit != nil && raw.Limit.Context > 0 {
		model.ContextLength = raw.Limit.Context
	} else {
		model.ContextLength = raw.ContextLength
	}
	if raw.Modalities != nil {
		model.SupportsImageIn = false
		model.SupportsVideoIn = false
		for _, input := range raw.Modalities.Input {
			switch input {
			case "image":
				model.SupportsImageIn = true
			case "video":
				model.SupportsVideoIn = true
			}
		}
	}
	if raw.Reasoning != nil {
		model.ThinkEfforts = thinkEffortsFromReasoning(raw.Reasoning.Effort)
		// The unified schema names the non-thinking state 'none'; the stored
		// three-state keeps the legacy 'no'.
		switch raw.Reasoning.Type {
		case "only", "both":
			model.SupportsThinkingType = raw.Reasoning.Type
			model.SupportsReasoning = true
		case "none":
			model.SupportsThinkingType = "no"
			model.SupportsReasoning = false
		}
	}
	return model, nil
}

// thinkEffortsFromReasoning converts the reasoning.effort block
// ({valid: [...], default: "..."}) into the persisted think_efforts shape. A
// default that is not listed in valid is dropped, matching the official CLI.
func thinkEffortsFromReasoning(effort *reasoningEffortBlock) *oauth.KimiCodeThinkEfforts {
	if effort == nil {
		return nil
	}
	valid := make([]string, 0, len(effort.Valid))
	for _, value := range effort.Valid {
		if strings.TrimSpace(value) != "" {
			valid = append(valid, value)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	result := &oauth.KimiCodeThinkEfforts{Support: true, ValidEfforts: valid}
	if slices.Contains(valid, effort.Default) {
		result.DefaultEffort = effort.Default
	}
	return result
}

// normalizeModelProtocol maps the raw catalog protocol to the persisted
// spelling. Only 'anthropic' and 'response' are recognized; everything else
// (including absent) means the Kimi OpenAI-compatible wire.
func normalizeModelProtocol(protocol string) string {
	switch strings.TrimSpace(protocol) {
	case ProtocolAnthropic:
		return ProtocolAnthropic
	case ProtocolResponse:
		return ProtocolResponse
	default:
		return ""
	}
}

func ValidateModel(model Model) error {
	if strings.TrimSpace(model.ID) == "" {
		return errors.New("Kimi Code model missing id")
	}
	if model.ContextLength <= 0 {
		return fmt.Errorf("Kimi Code model %q missing positive context length", model.ID)
	}
	if model.Protocol != "" && model.Protocol != ProtocolKimi && model.Protocol != ProtocolAnthropic && model.Protocol != ProtocolResponse {
		return fmt.Errorf("Kimi Code model %q has unsupported protocol %q", model.ID, model.Protocol)
	}
	return nil
}

func NewMetadata(models []Model) *oauth.KimiCodeMetadata {
	copied := append([]Model(nil), models...)
	return &oauth.KimiCodeMetadata{Models: copied}
}

package kimicode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// Model is the normalized Kimi Code /models record persisted in OAuth
// credentials. It aliases the shared serialized representation deliberately.
type Model = oauth.KimiCodeModel

type ModelsResponse struct {
	Data   []Model `json:"data"`
	Models []Model `json:"models"`
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
	models := decoded.Data
	if len(models) == 0 {
		models = decoded.Models
	}
	if len(models) == 0 {
		return nil, errors.New("Kimi Code models response is empty")
	}
	for _, model := range models {
		if err := ValidateModel(model); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func ValidateModel(model Model) error {
	if strings.TrimSpace(model.ID) == "" {
		return errors.New("Kimi Code model missing id")
	}
	if model.ContextLength <= 0 {
		return fmt.Errorf("Kimi Code model %q missing positive context_length", model.ID)
	}
	if model.Protocol != "" && model.Protocol != ProtocolKimi && model.Protocol != ProtocolAnthropic {
		return fmt.Errorf("Kimi Code model %q has unsupported protocol %q", model.ID, model.Protocol)
	}
	return nil
}

func NewMetadata(models []Model) *oauth.KimiCodeMetadata {
	copied := append([]Model(nil), models...)
	return &oauth.KimiCodeMetadata{Models: copied}
}

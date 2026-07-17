package kimicode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/auth"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/pipeline"
	"github.com/ldm2060/axonhub/llm/streams"
	"github.com/ldm2060/axonhub/llm/transformer"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic"
	"github.com/ldm2060/axonhub/llm/transformer/openai"
)

const protocolMetadataKey = "kimi_code_protocol"

// OutboundTransformer routes each request from immutable persisted model
// metadata. Protocol selection is written to request metadata so concurrent
// requests never share mutable routing state.
type OutboundTransformer struct {
	tokens    *TokenProvider
	identity  Identity
	baseURL   string
	models    map[string]Model
	openAI    transformer.Outbound
	anthropic transformer.Outbound
}

var (
	_ transformer.Outbound               = (*OutboundTransformer)(nil)
	_ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)
)

type Params struct {
	TokenProvider *TokenProvider
	BaseURL       string
	Identity      Identity
	Models        []Model
}

func NewOutboundTransformer(params Params) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("Kimi Code token provider is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(params.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	modelMap := make(map[string]Model, len(params.Models))
	for _, model := range params.Models {
		if err := ValidateModel(model); err != nil {
			return nil, err
		}
		modelMap[model.ID] = model
	}
	if len(modelMap) == 0 {
		return nil, errors.New("Kimi Code model metadata is required")
	}
	openAIOutbound, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType: openai.PlatformOpenAI, BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider("placeholder"),
	})
	if err != nil {
		return nil, fmt.Errorf("create Kimi Code OpenAI transformer: %w", err)
	}
	anthropicOutbound, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
		Type: anthropic.PlatformDirect, BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider("placeholder"), EndpointPath: MessagesPath,
	})
	if err != nil {
		return nil, fmt.Errorf("create Kimi Code Anthropic transformer: %w", err)
	}
	return &OutboundTransformer{
		tokens: params.TokenProvider, identity: params.Identity, baseURL: baseURL, models: modelMap,
		openAI: openAIOutbound, anthropic: anthropicOutbound,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat { return llm.APIFormatOpenAIChatCompletion }

func (t *OutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil {
		return nil, errors.New("Kimi Code request is nil")
	}
	model, ok := t.models[request.Model]
	if !ok {
		return nil, fmt.Errorf("Kimi Code model metadata not found for %q", request.Model)
	}
	credentials, err := t.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}
	if model.Protocol == ProtocolAnthropic {
		return t.transformAnthropicRequest(ctx, request, credentials.AccessToken)
	}
	return t.transformKimiRequest(ctx, request, credentials.AccessToken)
}

func (t *OutboundTransformer) transformKimiRequest(ctx context.Context, original *llm.Request, token string) (*httpclient.Request, error) {
	copyRequest := *original
	if original.StreamOptions != nil {
		streamOptions := *original.StreamOptions
		copyRequest.StreamOptions = &streamOptions
	}
	// Kimi Code prefers the modern field when clients supplied max_tokens.
	if copyRequest.MaxCompletionTokens == nil && copyRequest.MaxTokens != nil {
		copyRequest.MaxCompletionTokens = copyRequest.MaxTokens
		copyRequest.MaxTokens = nil
	}
	if copyRequest.Stream != nil && *copyRequest.Stream {
		if copyRequest.StreamOptions == nil {
			copyRequest.StreamOptions = &llm.StreamOptions{}
		}
		copyRequest.StreamOptions.IncludeUsage = true
	}
	result, err := t.openAI.TransformRequest(ctx, &copyRequest)
	if err != nil {
		return nil, err
	}
	t.applyKimiIdentity(result, token, ProtocolKimi)
	return result, nil
}

func (t *OutboundTransformer) transformAnthropicRequest(ctx context.Context, original *llm.Request, token string) (*httpclient.Request, error) {
	copyRequest := *original
	// Kimi's beta Messages API accepts adaptive thinking but not arbitrary
	// budget-based thinking. Do not overwrite an explicit disabled request.
	if copyRequest.TransformerMetadata == nil {
		copyRequest.TransformerMetadata = make(map[string]any)
	}
	if thinkingType, _ := copyRequest.TransformerMetadata[anthropic.TransformerMetadataKeyThinkingType].(string); thinkingType != "disabled" && copyRequest.ReasoningEffort != "none" && (copyRequest.ReasoningEffort != "" || copyRequest.ReasoningBudget != nil) {
		copyRequest.TransformerMetadata[anthropic.TransformerMetadataKeyThinkingType] = "adaptive"
		copyRequest.ReasoningBudget = nil
	}
	result, err := t.anthropic.TransformRequest(ctx, &copyRequest)
	if err != nil {
		return nil, err
	}
	if result.Query == nil {
		result.Query = make(url.Values)
	}
	result.Query.Set("beta", "true")
	// Kimi expects beta features in the JSON body rather than Anthropic-Beta.
	result.Headers.Del("Anthropic-Beta")
	result.Body, err = putBetasInBody(result.Body)
	if err != nil {
		return nil, err
	}
	t.applyKimiIdentity(result, token, ProtocolAnthropic)
	return result, nil
}

func putBetasInBody(raw []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("decode Kimi Code Anthropic request: %w", err)
	}
	body["betas"] = []string{"interleaved-thinking-2025-05-14"}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Kimi Code Anthropic request: %w", err)
	}
	return encoded, nil
}

func (t *OutboundTransformer) applyKimiIdentity(request *httpclient.Request, token, protocol string) {
	identityHeaders := BuildIdentityHeaders(t.identity)
	maps.Copy(request.Headers, identityHeaders)
	request.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: token}
	request.Headers.Del("Authorization")
	if request.Metadata == nil {
		request.Metadata = make(map[string]string)
	}
	request.Metadata[protocolMetadataKey] = protocol
}

func (t *OutboundTransformer) protocolForRequest(request *httpclient.Request) string {
	if request != nil && request.Metadata != nil && request.Metadata[protocolMetadataKey] == ProtocolAnthropic {
		return ProtocolAnthropic
	}
	return ProtocolKimi
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	if t.protocolForRequest(response.Request) == ProtocolAnthropic {
		return t.anthropic.TransformResponse(ctx, response)
	}
	return t.openAI.TransformResponse(ctx, response)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, request *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	if t.protocolForRequest(request) == ProtocolAnthropic {
		return t.anthropic.TransformStream(ctx, request, stream)
	}
	return t.openAI.TransformStream(ctx, request, stream)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, request *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if t.protocolForRequest(request) == ProtocolAnthropic {
		return t.anthropic.AggregateStreamChunks(ctx, request, chunks)
	}
	return t.openAI.AggregateStreamChunks(ctx, request, chunks)
}

func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr != nil && rawErr.StatusCode == http.StatusUnauthorized {
		return &llm.ResponseError{StatusCode: rawErr.StatusCode, Detail: llm.ErrorDetail{Message: "Kimi Code login is required", Type: "authentication_error"}}
	}
	return t.openAI.TransformError(ctx, rawErr)
}

func (t *OutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return &retryingExecutor{inner: executor, tokens: t.tokens}
}

type retryingExecutor struct {
	inner  pipeline.Executor
	tokens *TokenProvider
}

func (e *retryingExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	response, err := e.inner.Do(ctx, request)
	if !isUnauthorized(err) {
		return response, err
	}
	if err := e.refreshAndReplace(ctx, request); err != nil {
		return nil, err
	}
	return e.inner.Do(ctx, request)
}

func (e *retryingExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	stream, err := e.inner.DoStream(ctx, request)
	if !isUnauthorized(err) {
		return stream, err
	}
	if err := e.refreshAndReplace(ctx, request); err != nil {
		return nil, err
	}
	return e.inner.DoStream(ctx, request)
}

func (e *retryingExecutor) refreshAndReplace(ctx context.Context, request *httpclient.Request) error {
	credentials, err := e.tokens.ForceRefresh(ctx)
	if err != nil {
		return fmt.Errorf("Kimi Code login is required after upstream 401: %w", err)
	}
	request.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: credentials.AccessToken}
	request.Headers.Set("Authorization", "Bearer "+credentials.AccessToken)
	return nil
}

func isUnauthorized(err error) bool {
	var httpErr *httpclient.Error
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusUnauthorized
}

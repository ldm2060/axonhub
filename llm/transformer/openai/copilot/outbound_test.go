package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

// mockTokenProvider is a mock implementation of TokenProvider for testing.
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetToken(ctx context.Context) (string, error) {
	return m.token, m.err
}

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name        string
		params      OutboundTransformerParams
		wantErr     bool
		errContains string
	}{
		{
			name: "successful creation with defaults",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
			},
			wantErr: false,
		},
		{
			name: "successful creation with custom base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
				BaseURL:       "https://custom.copilot.api",
			},
			wantErr: false,
		},
		{
			name: "successful creation with trailing slash in base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: "test-token"},
				BaseURL:       "https://custom.copilot.api/",
			},
			wantErr: false,
		},
		{
			name:        "error when token provider is nil",
			params:      OutboundTransformerParams{},
			wantErr:     true,
			errContains: "token provider is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, transformer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transformer)
				assert.NotNil(t, transformer.tokenProvider)
				assert.False(t, strings.HasSuffix(transformer.baseURL, "/"), "base URL should not have trailing slash")
			}
		})
	}
}

func TestOutboundTransformer_APIFormat(t *testing.T) {
	transformer := &OutboundTransformer{}
	assert.Equal(t, llm.APIFormatOpenAIChatCompletion, transformer.APIFormat())
}

func TestUsesResponsesAPI(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "gpt-5 uses responses API",
			model:    "gpt-5",
			expected: true,
		},
		{
			name:     "gpt-5-mini does not use responses API",
			model:    "gpt-5-mini",
			expected: false,
		},
		{
			name:     "gpt-5.3 uses responses API",
			model:    "gpt-5.3",
			expected: true,
		},
		{
			name:     "gpt-5.4 uses responses API",
			model:    "gpt-5.4",
			expected: true,
		},
		{
			name:     "gpt-5.4 preview uses responses API",
			model:    "gpt-5.4-preview",
			expected: true,
		},
		{
			name:     "gpt-5.5 uses responses API",
			model:    "gpt-5.5",
			expected: true,
		},
		{
			name:     "gpt-5.10 uses responses API",
			model:    "gpt-5.10",
			expected: true,
		},
		{
			name:     "gpt-6 uses responses API",
			model:    "gpt-6",
			expected: true,
		},
		{
			name:     "gpt-6.1 uses responses API",
			model:    "gpt-6.1",
			expected: true,
		},
		{
			name:     "gpt-6-preview uses responses API",
			model:    "gpt-6-preview",
			expected: true,
		},
		{
			name:     "regular chat model does not use responses API",
			model:    "gpt-4o",
			expected: false,
		},
		{
			name:     "claude model does not use responses API",
			model:    "claude-sonnet-4.6",
			expected: false,
		},
		{
			name:     "claude-3-5-sonnet does not use responses API",
			model:    "claude-3-5-sonnet",
			expected: false,
		},
		{
			name:     "grok-4.6 uses responses API",
			model:    "grok-4.6",
			expected: true,
		},
		{
			name:     "grok-4.5 uses responses API",
			model:    "grok-4.5",
			expected: true,
		},
		{
			name:     "Grok-4.6 uses responses API",
			model:    "Grok-4.6",
			expected: true,
		},
		{
			name:     "non-grok non-gpt-5 model does not use responses API",
			model:    "o4-mini",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, usesResponsesAPI(tt.model))
		})
	}
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	// Create a valid LLM request
	createValidRequest := func() *llm.Request {
		return &llm.Request{
			Model: "gpt-4o",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
				},
			},
		}
	}

	tests := []struct {
		name        string
		params      OutboundTransformerParams
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(t *testing.T, req *httpclient.Request)
	}{
		{
			name: "successful transformation with default headers",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: createValidRequest(),
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				// Validate method and URL
				assert.Equal(t, "POST", req.Method)
				assert.Equal(t, DefaultCopilotBaseURL+CopilotChatCompletionsEndpoint, req.URL)

				// Validate headers
				assert.Equal(t, "application/json", req.Headers.Get("Content-Type"))
				assert.Equal(t, "application/json", req.Headers.Get("Accept"))

				// Validate copilot headers
				assert.Equal(t, DefaultUserAgent, req.Headers.Get(UserAgentHeader))
				assert.Equal(t, DefaultOpenAIIntent, req.Headers.Get(OpenAIIntentHeader))

				// Vision header should NOT be present for text-only request
				assert.Empty(t, req.Headers.Get(CopilotVisionRequestHeader))

				// Validate auth config
				assert.NotNil(t, req.Auth)
				assert.Equal(t, httpclient.AuthTypeBearer, req.Auth.Type)
				assert.Equal(t, mockToken, req.Auth.APIKey)

				// Validate API format
				assert.Equal(t, string(llm.APIFormatOpenAIChatCompletion), req.APIFormat)

				// Validate body is valid JSON
				var body map[string]any

				err := json.Unmarshal(req.Body, &body)
				assert.NoError(t, err)
				assert.Equal(t, "gpt-4o", body["model"])
			},
		},
		{
			name:        "error when request is nil",
			params:      OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request:     nil,
			wantErr:     true,
			errContains: "request is nil",
		},
		{
			name:   "error when model is empty",
			params: OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request: &llm.Request{
				Model: "",
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
				},
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name:   "error when messages are empty",
			params: OutboundTransformerParams{TokenProvider: &mockTokenProvider{token: mockToken}},
			request: &llm.Request{
				Model:    "gpt-4o",
				Messages: []llm.Message{},
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name: "error when token provider fails",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{err: errors.New("token fetch failed")},
			},
			request:     createValidRequest(),
			wantErr:     true,
			errContains: "failed to get copilot token",
		},
		{
			name: "successful transformation with custom base URL",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
				BaseURL:       "https://custom.copilot.github.com",
			},
			request: createValidRequest(),
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, "https://custom.copilot.github.com"+CopilotChatCompletionsEndpoint, req.URL)
			},
		},
		{
			name: "gpt-5.4 uses responses API endpoint",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: &llm.Request{
				Model: "gpt-5.4",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, DefaultCopilotBaseURL+"/v1/responses", req.URL)
			},
		},
		{
			name: "codex model uses responses API endpoint",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: &llm.Request{
				Model: "gpt-5.2-codex",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, DefaultCopilotBaseURL+"/v1/responses", req.URL)
			},
		},
		{
			name: "grok-4.6 uses responses API endpoint",
			params: OutboundTransformerParams{
				TokenProvider: &mockTokenProvider{token: mockToken},
			},
			request: &llm.Request{
				Model: "grok-4.6",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello, Copilot!")},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, req *httpclient.Request) {
				assert.Equal(t, DefaultCopilotBaseURL+"/v1/responses", req.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.params)
			require.NoError(t, err)

			httpReq, err := transformer.TransformRequest(ctx, tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, httpReq)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, httpReq)

				if tt.validate != nil {
					tt.validate(t, httpReq)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_VisionHeaders(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		request      *llm.Request
		expectVision bool
		visionValue  string
	}{
		{
			name: "text only - no vision header",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Just text")},
					},
				},
			},
			expectVision: false,
		},
		{
			name: "image_url type - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"},
								},
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "data:image URL in text - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,iVBORw0KGgo...")},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "data:image URL in multiple content - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("data:image/jpeg;base64,/9j/4AAQ..."),
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
		{
			name: "mixed text and image - vision header present",
			request: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "system",
						Content: llm.MessageContent{Content: lo.ToPtr("You are a helpful assistant")},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Describe this:"),
								},
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/photo.jpg", Detail: lo.ToPtr("high")},
								},
							},
						},
					},
				},
			},
			expectVision: true,
			visionValue:  "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, err := transformer.TransformRequest(ctx, tt.request)
			require.NoError(t, err)
			require.NotNil(t, httpReq)

			visionHeader := httpReq.Headers.Get(CopilotVisionRequestHeader)
			if tt.expectVision {
				assert.Equal(t, tt.visionValue, visionHeader)
			} else {
				assert.Empty(t, visionHeader)
			}
		})
	}
}

func TestXInitiatorDefault(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	request := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)
	// With message-based inference, user message means initiator is "user"
	assert.Equal(t, "user", httpReq.Headers.Get(InitiatorHeader))
}

func TestXInitiatorForwarding(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		initiatorValue string
		expected       string
	}{
		{
			name:           "valid user initiator value",
			initiatorValue: "user",
			expected:       "user",
		},
		{
			name:           "valid agent initiator value",
			initiatorValue: "agent",
			expected:       "agent",
		},
		{
			name:           "case insensitive - USER",
			initiatorValue: "USER",
			expected:       "user",
		},
		{
			name:           "case insensitive - Agent",
			initiatorValue: "Agent",
			expected:       "agent",
		},
		{
			name:           "invalid value - ignored, inference applies",
			initiatorValue: "editor",
			expected:       "user", // Invalid, falls through to inference (user message)
		},
		{
			name:           "empty string - inference applies",
			initiatorValue: "",
			expected:       "user", // Empty header, so inference from message applies
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
				RawRequest: &httpclient.Request{
					Headers: make(http.Header),
				},
			}
			if tt.initiatorValue != "" {
				request.RawRequest.Headers.Set(InitiatorHeader, tt.initiatorValue)
			}

			httpReq, err := transformer.TransformRequest(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, httpReq)
			assert.Equal(t, tt.expected, httpReq.Headers.Get(InitiatorHeader))
		})
	}
}

func TestHasVisionContent(t *testing.T) {
	tests := []struct {
		name     string
		request  *llm.Request
		expected bool
	}{
		{
			name: "text only - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Just text")},
					},
				},
			},
			expected: false,
		},
		{
			name: "image_url type - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "image_url with nil ImageURL - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:     "image_url",
									ImageURL: nil,
								},
							},
						},
					},
				},
			},
			expected: true, // Type is image_url, so it should detect vision
		},
		{
			name: "data:image in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,abc123")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image/jpeg in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/jpeg;base64,xyz789")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image/webp in single content - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/webp;base64,webp123")},
					},
				},
			},
			expected: true,
		},
		{
			name: "data:image in multiple content text field - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("data:image/gif;base64,gif456"),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "text with data: prefix but not image - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:text/plain;base64,hello")},
					},
				},
			},
			expected: false,
		},
		{
			name: "regular URL with image in path - no vision (not data URL)",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("https://example.com/images/photo.png")},
					},
				},
			},
			expected: false,
		},
		{
			name: "empty content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("")},
					},
				},
			},
			expected: false,
		},
		{
			name: "nil content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{},
					},
				},
			},
			expected: false,
		},
		{
			name: "text type in multiple content - no vision",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Just text content"),
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "mixed content with image - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "system",
						Content: llm.MessageContent{Content: lo.ToPtr("You are helpful")},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Look at this:"),
								},
								{
									Type:     "image_url",
									ImageURL: &llm.ImageURL{URL: "https://example.com/img.png"},
								},
							},
						},
					},
					{
						Role:    "assistant",
						Content: llm.MessageContent{Content: lo.ToPtr("I see the image")},
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple messages with data URL - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("First message")},
					},
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,second")},
					},
				},
			},
			expected: true,
		},
		{
			name: "assistant message with vision - vision detected",
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role:    "assistant",
						Content: llm.MessageContent{Content: lo.ToPtr("data:image/png;base64,assistant")},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasVisionContent(tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImageDataURL(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "PNG data URL",
			content:  "data:image/png;base64,iVBORw0KGgo=",
			expected: true,
		},
		{
			name:     "JPEG data URL",
			content:  "data:image/jpeg;base64,/9j/4AAQ=",
			expected: true,
		},
		{
			name:     "WEBP data URL",
			content:  "data:image/webp;base64,UklGR=",
			expected: true,
		},
		{
			name:     "GIF data URL",
			content:  "data:image/gif;base64,R0lGOD=",
			expected: true,
		},
		{
			name:     "SVG data URL",
			content:  "data:image/svg+xml;base64,PHN2Zw=",
			expected: true,
		},
		{
			name:     "Plain text",
			content:  "Hello, world!",
			expected: false,
		},
		{
			name:     "Regular HTTP URL with image",
			content:  "https://example.com/image.png",
			expected: false,
		},
		{
			name:     "data:text URL",
			content:  "data:text/plain;base64,SGVsbG8=",
			expected: false,
		},
		{
			name:     "data:application URL",
			content:  "data:application/pdf;base64,JVBERi0=",
			expected: false,
		},
		{
			name:     "Empty string",
			content:  "",
			expected: false,
		},
		{
			name:     "Partial data prefix",
			content:  "data:ima",
			expected: false,
		},
		{
			name:     "Case sensitive - uppercase",
			content:  "DATA:IMAGE/PNG;base64,ABC=",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImageDataURL(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	ctx := context.Background()
	transformer := &OutboundTransformer{}

	tests := []struct {
		name        string
		httpResp    *httpclient.Response
		wantErr     bool
		errContains string
		validate    func(t *testing.T, resp *llm.Response)
	}{
		{
			name:        "error when http response is nil",
			httpResp:    nil,
			wantErr:     true,
			errContains: "http response is nil",
		},
		{
			name: "error on HTTP 400 status",
			httpResp: &httpclient.Response{
				StatusCode: 400,
				Body:       []byte(`{"error": "bad request"}`),
			},
			wantErr:     true,
			errContains: "HTTP error 400",
		},
		{
			name: "error on HTTP 500 status",
			httpResp: &httpclient.Response{
				StatusCode: 500,
				Body:       []byte(`{"error": "internal error"}`),
			},
			wantErr:     true,
			errContains: "HTTP error 500",
		},
		{
			name: "error when body is empty",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body:       []byte{},
			},
			wantErr:     true,
			errContains: "response body is empty",
		},
		{
			name: "error when body is invalid JSON",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(`not valid json`),
			},
			wantErr:     true,
			errContains: "failed to unmarshal response",
		},
		{
			name: "successful transformation",
			httpResp: &httpclient.Response{
				StatusCode: 200,
				Body: []byte(`{
					"id": "chatcmpl-123",
					"object": "chat.completion",
					"created": 1700000000,
					"model": "gpt-4o",
					"choices": [
						{
							"index": 0,
							"message": {
								"role": "assistant",
								"content": "Hello! How can I help you today?"
							},
							"finish_reason": "stop"
						}
					]
				}`),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *llm.Response) {
				assert.Equal(t, "chatcmpl-123", resp.ID)
				assert.Equal(t, "chat.completion", resp.Object)
				assert.Equal(t, "gpt-4o", resp.Model)
				assert.Len(t, resp.Choices, 1)
				assert.Equal(t, "assistant", resp.Choices[0].Message.Role)

				if resp.Choices[0].Message.Content.Content != nil {
					assert.Equal(t, "Hello! How can I help you today?", *resp.Choices[0].Message.Content.Content)
				}

				if resp.Choices[0].FinishReason != nil {
					assert.Equal(t, "stop", *resp.Choices[0].FinishReason)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.TransformResponse(ctx, tt.httpResp)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)

				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformError(t *testing.T) {
	ctx := context.Background()
	transformer := &OutboundTransformer{}

	tests := []struct {
		name     string
		rawErr   *httpclient.Error
		validate func(t *testing.T, respErr *llm.ResponseError)
	}{
		{
			name:   "nil error - returns generic error",
			rawErr: nil,
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 500, respErr.StatusCode)
				assert.Equal(t, "Internal Server Error", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with OpenAI format - error field",
			rawErr: &httpclient.Error{
				StatusCode: 401,
				Body:       []byte(`{"error": {"message": "Invalid API key", "type": "authentication_error"}}`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 401, respErr.StatusCode)
				assert.Equal(t, "Invalid API key", respErr.Detail.Message)
				assert.Equal(t, "authentication_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with OpenAI format - errors field",
			rawErr: &httpclient.Error{
				StatusCode: 429,
				Body:       []byte(`{"errors": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 429, respErr.StatusCode)
				assert.Equal(t, "Rate limit exceeded", respErr.Detail.Message)
				assert.Equal(t, "rate_limit_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with non-JSON body - uses status text",
			rawErr: &httpclient.Error{
				StatusCode: 503,
				Body:       []byte(`service unavailable`),
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 503, respErr.StatusCode)
				assert.Equal(t, "Service Unavailable", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
		{
			name: "error with empty body - uses status text",
			rawErr: &httpclient.Error{
				StatusCode: 502,
				Body:       []byte{},
			},
			validate: func(t *testing.T, respErr *llm.ResponseError) {
				assert.Equal(t, 502, respErr.StatusCode)
				assert.Equal(t, "Bad Gateway", respErr.Detail.Message)
				assert.Equal(t, "api_error", respErr.Detail.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respErr := transformer.TransformError(ctx, tt.rawErr)
			assert.NotNil(t, respErr)
			tt.validate(t, respErr)
		})
	}
}

func TestInferCopilotInitiator(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expected string
	}{
		{
			name:     "empty messages - default to user",
			messages: []llm.Message{},
			expected: "user",
		},
		{
			name: "last message from user - returns user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
			},
			expected: "user",
		},
		{
			name: "last message from assistant - returns agent",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi there")}},
			},
			expected: "agent",
		},
		{
			name: "last message from user with tool_result - returns agent",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Let me check")}},
				{Role: "user", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "tool_result", Text: lo.ToPtr("result")},
				}}},
			},
			expected: "agent",
		},
		{
			name: "last message from system - returns agent",
			messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("You are helpful")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("Additional context")}},
			},
			expected: "agent",
		},
		// P2 fix: tool_result positioning tests
		{
			name: "tool_result NOT in last position - returns user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Let me check")}},
				{Role: "user", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "tool_result", Text: lo.ToPtr("result")},
					{Type: "text", Text: lo.ToPtr("Now answer my question")},
				}}},
			},
			expected: "user", // text is last, user is prompting
		},
		{
			name: "tool_result in last position - returns agent",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Let me check")}},
				{Role: "user", Content: llm.MessageContent{MultipleContent: []llm.MessageContentPart{
					{Type: "text", Text: lo.ToPtr("What about")},
					{Type: "tool_result", Text: lo.ToPtr("result")},
				}}},
			},
			expected: "agent", // tool_result is last
		},
		// P2 fix: attribution field tests
		{
			name: "attribution field user - returns user",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}, Attribution: "user"},
			},
			expected: "user",
		},
		{
			name: "attribution field agent - returns agent",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}, Attribution: "agent"},
			},
			expected: "agent",
		},
		{
			name: "attribution field overrides role inference",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}, Attribution: "user"},
			},
			expected: "user", // attribution overrides role
		},
		{
			name: "attribution field case insensitive",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}, Attribution: "  AGENT  "},
			},
			expected: "agent", // normalized
		},
		{
			name: "invalid attribution ignored",
			messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}, Attribution: "invalid"},
			},
			expected: "user", // falls through to role inference
		},
		{
			name: "invalid attribution falls through to role inference (agent role)",
			messages: []llm.Message{
				{Role: "assistant", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}, Attribution: "invalid"},
			},
			expected: "agent", // invalid attribution ignored, role != "user" -> agent
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferCopilotInitiator(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUsesAnthropicMessages(t *testing.T) {
	// Create model info map with various models
	modelInfo := map[string]*CopilotModelInfo{
		"claude-sonnet-4.6": {
			ModelID:                   "claude-sonnet-4.6",
			SupportsAnthropicMessages: true,
		},
		"claude-opus-4.6": {
			ModelID:                   "claude-opus-4.6",
			SupportsAnthropicMessages: true,
		},
		"gpt-5.4": {
			ModelID:                   "gpt-5.4",
			SupportsAnthropicMessages: false,
		},
		"gpt-4o": {
			ModelID:                   "gpt-4o",
			SupportsAnthropicMessages: false,
		},
	}

	tests := []struct {
		name      string
		modelInfo map[string]*CopilotModelInfo
		model     string
		expected  bool
	}{
		{
			name:      "claude model with Anthropic support returns true",
			modelInfo: modelInfo,
			model:     "claude-sonnet-4.6",
			expected:  true,
		},
		{
			name:      "another claude model with Anthropic support returns true",
			modelInfo: modelInfo,
			model:     "claude-opus-4.6",
			expected:  true,
		},
		{
			name:      "GPT model without Anthropic support returns false",
			modelInfo: modelInfo,
			model:     "gpt-5.4",
			expected:  false,
		},
		{
			name:      "GPT-4o without Anthropic support returns false",
			modelInfo: modelInfo,
			model:     "gpt-4o",
			expected:  false,
		},
		{
			name:      "unknown model returns false",
			modelInfo: modelInfo,
			model:     "unknown-model",
			expected:  false,
		},
		{
			name:      "nil modelInfo returns false",
			modelInfo: nil,
			model:     "claude-sonnet-4.6",
			expected:  false,
		},
		{
			name:      "empty modelInfo map returns false",
			modelInfo: map[string]*CopilotModelInfo{},
			model:     "claude-sonnet-4.6",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := &OutboundTransformer{
				modelInfo: tt.modelInfo,
			}
			result := transformer.usesAnthropicMessages(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnthropicBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "default Copilot base URL",
			baseURL:  "https://api.githubcopilot.com",
			expected: "https://api.githubcopilot.com/v1",
		},
		{
			name:     "custom base URL",
			baseURL:  "https://custom.copilot.api",
			expected: "https://custom.copilot.api/v1",
		},
		{
			name:     "base URL without trailing slash",
			baseURL:  "https://api.githubcopilot.com",
			expected: "https://api.githubcopilot.com/v1",
		},
		{
			name:     "base URL with port",
			baseURL:  "https://localhost:8080",
			expected: "https://localhost:8080/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := &OutboundTransformer{
				baseURL: tt.baseURL,
			}
			result := transformer.anthropicBaseURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAnthropicTransformer_LazyInit(t *testing.T) {
	transformer := &OutboundTransformer{
		baseURL: "https://api.githubcopilot.com",
	}

	// First call should initialize
	anthropicT := transformer.getAnthropicTransformer()
	assert.NotNil(t, anthropicT)
	assert.Equal(t, llm.APIFormatAnthropicMessage, anthropicT.APIFormat())

	// Second call should return the same instance
	anthropicT2 := transformer.getAnthropicTransformer()
	assert.NotNil(t, anthropicT2)
	// Same pointer check confirms sync.Once works
	assert.Same(t, anthropicT, anthropicT2,
		"getAnthropicTransformer should return the same instance on subsequent calls")
}

func TestGetAnthropicTransformer_WithModelInfo(t *testing.T) {
	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: "test-token"},
		BaseURL:       "https://api.githubcopilot.com",
		ModelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
	})
	require.NoError(t, err)

	// Verify modelInfo is set correctly
	assert.NotNil(t, transformer.modelInfo)
	assert.True(t, transformer.usesAnthropicMessages("claude-sonnet-4.6"))
	assert.False(t, transformer.usesAnthropicMessages("gpt-4o"))

	// Verify base URL is correct
	assert.Equal(t, "https://api.githubcopilot.com/v1", transformer.anthropicBaseURL())
}

func TestTransformAnthropicRequest_Headers(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
		ModelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
	})
	require.NoError(t, err)

	request := &llm.Request{
		Model: "claude-sonnet-4.6",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify anthropic-beta header
	assert.Equal(t, "interleaved-thinking-2025-05-14", httpReq.Headers.Get("anthropic-beta"))

	// Verify Authorization is Bearer with Copilot token
	assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	assert.Equal(t, mockToken, httpReq.Auth.APIKey)

	// Verify Copilot-specific headers
	assert.Equal(t, DefaultUserAgent, httpReq.Headers.Get(UserAgentHeader))
	assert.Equal(t, DefaultOpenAIIntent, httpReq.Headers.Get(OpenAIIntentHeader))

	// Verify X-Api-Key is removed (Anthropic uses Bearer auth for Copilot)
	assert.Empty(t, httpReq.Headers.Get("X-Api-Key"))

	// Verify X-Initiator header
	assert.Equal(t, "user", httpReq.Headers.Get(InitiatorHeader))

	// Verify Content-Type is application/json (Anthropic format)
	assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))
}

func TestTransformAnthropicRequest_VisionHeader(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
		ModelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
	})
	require.NoError(t, err)

	t.Run("vision header present for image request", func(t *testing.T) {
		request := &llm.Request{
			Model: "claude-sonnet-4.6",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type:     "image_url",
								ImageURL: &llm.ImageURL{URL: "https://example.com/image.png"},
							},
						},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "true", httpReq.Headers.Get(CopilotVisionRequestHeader))
	})

	t.Run("no vision header for text-only request", func(t *testing.T) {
		request := &llm.Request{
			Model: "claude-sonnet-4.6",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Empty(t, httpReq.Headers.Get(CopilotVisionRequestHeader))
	})
}

func TestTransformAnthropicRequest_XInitiator(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
		ModelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
	})
	require.NoError(t, err)

	t.Run("agent initiator for assistant last message", func(t *testing.T) {
		request := &llm.Request{
			Model: "claude-sonnet-4.6",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
				{
					Role:    "assistant",
					Content: llm.MessageContent{Content: lo.ToPtr("Hi there")},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "agent", httpReq.Headers.Get(InitiatorHeader))
	})

	t.Run("user initiator for user last message", func(t *testing.T) {
		request := &llm.Request{
			Model: "claude-sonnet-4.6",
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "user", httpReq.Headers.Get(InitiatorHeader))
	})
}

func TestStripMaxTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes max_tokens",
			input:    `{"model":"gpt-4o","messages":[],"max_tokens":4096}`,
			expected: `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "removes max_output_tokens",
			input:    `{"model":"gpt-4o","messages":[],"max_output_tokens":16384}`,
			expected: `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "removes both max_tokens and max_output_tokens",
			input:    `{"model":"gpt-4o","max_tokens":4096,"max_output_tokens":16384,"messages":[]}`,
			expected: `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "no changes when fields are absent",
			input:    `{"model":"gpt-4o","messages":[]}`,
			expected: `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "returns original on invalid JSON",
			input:    `not json`,
			expected: `not json`,
		},
		{
			name:     "empty body",
			input:    ``,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMaxTokens([]byte(tt.input))

			// For valid JSON cases, compare as JSON objects
			if tt.input != `` && tt.input != `not json` {
				var expectedMap, resultMap map[string]any
				errExpected := json.Unmarshal([]byte(tt.expected), &expectedMap)
				errResult := json.Unmarshal(result, &resultMap)
				require.NoError(t, errExpected)
				require.NoError(t, errResult)
				assert.Equal(t, expectedMap, resultMap)
			} else {
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestIsGPTModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// GPT models
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-4.1", true},
		{"gpt-5", true},
		{"gpt-5.4", true},
		{"gpt-5-mini", true},
		{"GPT-4O", true},
		// o-series models
		{"o1", true},
		{"o1-mini", true},
		{"o1-preview", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4", true},
		{"o4-mini", true},
		// non-GPT/non-o models
		{"claude-sonnet-4.6", false},
		{"claude-opus-4.6", false},
		{"gemini-pro", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.expected, isGPTModel(tt.model))
		})
	}
}

func TestTransformRequest_StripsMaxTokensForGPT(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	request := &llm.Request{
		Model: "gpt-4o",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	var body map[string]any
	err = json.Unmarshal(httpReq.Body, &body)
	require.NoError(t, err)

	_, hasMaxTokens := body["max_tokens"]
	assert.False(t, hasMaxTokens, "max_tokens should be stripped from GPT model request")

	_, hasMaxOutputTokens := body["max_output_tokens"]
	assert.False(t, hasMaxOutputTokens, "max_output_tokens should be stripped from GPT model request")
}

func TestTransformRequest_StripsMaxTokensForOSeries(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	for _, model := range []string{"o1", "o3", "o4-mini"} {
		t.Run(model, func(t *testing.T) {
			request := &llm.Request{
				Model: model,
				Messages: []llm.Message{
					{
						Role:    "user",
						Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
					},
				},
			}

			httpReq, err := transformer.TransformRequest(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, httpReq)

			var body map[string]any
			err = json.Unmarshal(httpReq.Body, &body)
			require.NoError(t, err)

			_, hasMaxTokens := body["max_tokens"]
			assert.False(t, hasMaxTokens, "max_tokens should be stripped from o-series model")

			_, hasMaxOutputTokens := body["max_output_tokens"]
			assert.False(t, hasMaxOutputTokens, "max_output_tokens should be stripped from o-series model")
		})
	}
}

func TestTransformRequest_DoesNotStripMaxTokensForNonGPT(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
	})
	require.NoError(t, err)

	request := &llm.Request{
		Model: "claude-sonnet-4.6",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
			},
		},
	}

	httpReq, err := transformer.TransformRequest(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	var body map[string]any
	err = json.Unmarshal(httpReq.Body, &body)
	require.NoError(t, err)

	assert.Contains(t, body, "model")
}

func TestTransformResponse_AnthropicRouting(t *testing.T) {
	ctx := context.Background()

	transformer := &OutboundTransformer{
		modelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
		lastModel: "claude-sonnet-4.6",
		baseURL:   "https://api.githubcopilot.com",
	}

	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body: []byte(`{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"model": "claude-sonnet-4-20250514",
			"content": [{"type": "text", "text": "Hello from Anthropic"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 20}
		}`),
	}

	resp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "msg_123", resp.ID)
}

func TestTransformResponse_NonAnthropicRouting(t *testing.T) {
	ctx := context.Background()

	transformer := &OutboundTransformer{
		modelInfo: nil,
		lastModel: "gpt-4o",
	}

	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body: []byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello!"},
				"finish_reason": "stop"
			}]
		}`),
	}

	resp, err := transformer.TransformResponse(ctx, httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "chat.completion", resp.Object)
}

func TestTransformStream_AnthropicRouting(t *testing.T) {
	ctx := context.Background()

	transformer := &OutboundTransformer{
		modelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
		lastModel: "claude-sonnet-4.6",
		baseURL:   "https://api.githubcopilot.com",
	}

	mockStream := &mockHTTPStream{
		events: []*httpclient.StreamEvent{
			{Data: []byte(`data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"usage":{"input_tokens":10,"output_tokens":0}}}`)},
			{Data: []byte(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)},
			{Data: []byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)},
			{Data: []byte(`data: {"type":"content_block_stop","index":0}`)},
			{Data: []byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`)},
			{Data: []byte(`data: {"type":"message_stop"}`)},
		},
	}

	req := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.githubcopilot.com/v1/messages",
	}

	result, err := transformer.TransformStream(ctx, req, mockStream)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Next())
	resp := result.Current()
	assert.NotNil(t, resp)

	result.Close()
}

func TestTransformStream_NonAnthropicRouting(t *testing.T) {
	ctx := context.Background()

	transformer := &OutboundTransformer{
		modelInfo: nil,
		lastModel: "gpt-4o",
		baseURL:   "https://api.githubcopilot.com",
	}

	mockStream := &mockHTTPStream{
		events: []*httpclient.StreamEvent{
			{Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`)},
			{Data: []byte(`[DONE]`)},
		},
	}

	req := &httpclient.Request{
		Method: "POST",
		URL:    "https://api.githubcopilot.com/chat/completions",
	}

	result, err := transformer.TransformStream(ctx, req, mockStream)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Next())
	resp := result.Current()
	assert.NotNil(t, resp)

	result.Close()
}

// mockHTTPStream implements streams.Stream[*httpclient.StreamEvent] for testing.
type mockHTTPStream struct {
	events   []*httpclient.StreamEvent
	position int
	current  *httpclient.StreamEvent
	closed   bool
}

func (s *mockHTTPStream) Next() bool {
	if s.position >= len(s.events) {
		return false
	}
	s.current = s.events[s.position]
	s.position++
	return true
}

func (s *mockHTTPStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *mockHTTPStream) Err() error {
	return nil
}

func (s *mockHTTPStream) Close() error {
	s.closed = true
	return nil
}

func TestTransformRequest_LastModelSet(t *testing.T) {
	ctx := context.Background()
	mockToken := "ghu_testtoken123"

	transformer, err := NewOutboundTransformer(OutboundTransformerParams{
		TokenProvider: &mockTokenProvider{token: mockToken},
		ModelInfo: map[string]*CopilotModelInfo{
			"claude-sonnet-4.6": {
				ModelID:                   "claude-sonnet-4.6",
				SupportsAnthropicMessages: true,
			},
		},
	})
	require.NoError(t, err)

	t.Run("lastModel set for Anthropic model", func(t *testing.T) {
		request := &llm.Request{
			Model: "claude-sonnet-4.6",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
			},
		}
		_, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "claude-sonnet-4.6", transformer.lastModel)
	})

	t.Run("lastModel set for GPT model", func(t *testing.T) {
		request := &llm.Request{
			Model: "gpt-4o",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hello")}},
			},
		}
		_, err := transformer.TransformRequest(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4o", transformer.lastModel)
	})
}

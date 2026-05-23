package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/project"
	"github.com/ldm2060/axonhub/internal/ent/request"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/httpclient"
	anthropictransformer "github.com/ldm2060/axonhub/llm/transformer/anthropic"
)

func TestRequestService_CreateRequest_PreservesAnthropicOutputConfigEffort(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithProjectID(ctx, proj.ID)

	req, err := svc.CreateRequest(
		ctx,
		&llm.Request{
			Model:           "claude-opus-4-1",
			ReasoningEffort: "xhigh",
			TransformerMetadata: map[string]any{
				anthropictransformer.TransformerMetadataKeyOutputConfigEffort: "max",
			},
		},
		&httpclient.Request{JSONBody: []byte(`{"model":"claude-opus-4-1","output_config":{"effort":"max"}}`)},
		llm.APIFormatAnthropicMessage,
	)
	require.NoError(t, err)
	require.Equal(t, "max", req.ReasoningEffort)
}

func TestRequestService_CreateRequestExecution_PersistsActualReasoningEffort(t *testing.T) {
	tests := []struct {
		name            string
		channelType     channel.Type
		modelID         string
		format          llm.APIFormat
		jsonBody        string
		reasoningEffort string
		wantEffort      string
	}{
		{
			name:        "anthropic output config effort",
			channelType: channel.TypeAnthropic,
			modelID:     "claude-opus-4-1",
			format:      llm.APIFormatAnthropicMessage,
			jsonBody:    `{"model":"claude-opus-4-1","output_config":{"effort":"max"}}`,
			wantEffort:  "max",
		},
		{
			name:            "non anthropic provider-specific body falls back to internal effort",
			channelType:     channel.TypeGemini,
			modelID:         "gemini-2.5-pro",
			format:          llm.APIFormatGeminiContents,
			jsonBody:        `{"contents":[{"parts":[{"text":"hello"}]}],"generation_config":{"thinking_config":{"thinking_level":"high"}}}`,
			reasoningEffort: "xhigh",
			wantEffort:      "xhigh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, client, ctx := setupTestRequestService(t)
			defer client.Close()

			proj, err := client.Project.Create().
				SetName("test-project").
				SetStatus(project.StatusActive).
				Save(ctx)
			require.NoError(t, err)

			ctx = contexts.WithProjectID(ctx, proj.ID)

			entChannel, err := client.Channel.Create().
				SetType(tt.channelType).
				SetName("test-channel").
				SetBaseURL("https://example.com").
				SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
				SetSupportedModels([]string{tt.modelID}).
				SetDefaultTestModel(tt.modelID).
				SetStatus(channel.StatusEnabled).
				Save(ctx)
			require.NoError(t, err)

			storedRequest, err := client.Request.Create().
				SetProjectID(proj.ID).
				SetModelID(tt.modelID).
				SetReasoningEffort("max").
				SetFormat(tt.format.String()).
				SetRequestBody([]byte(`{}`)).
				SetStatus(request.StatusProcessing).
				SetStream(false).
				Save(ctx)
			require.NoError(t, err)

			execution, err := svc.CreateRequestExecution(
				ctx,
				&Channel{Channel: entChannel},
				tt.modelID,
				tt.reasoningEffort,
				storedRequest,
				httpclient.Request{JSONBody: []byte(tt.jsonBody)},
				tt.format,
			)
			require.NoError(t, err)
			require.Equal(t, tt.wantEffort, execution.ReasoningEffort)
		})
	}
}

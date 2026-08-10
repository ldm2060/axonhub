package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/contexts"
	entprivacy "github.com/ldm2060/axonhub/internal/ent/privacy"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic"
)

type AnthropicHandlersParams struct {
	fx.In

	ChannelService              *biz.ChannelService
	ModelService                *biz.ModelService
	DefaultSelector             *orchestrator.DefaultSelector
	RequestService              *biz.RequestService
	SystemService               *biz.SystemService
	UsageLogService             *biz.UsageLogService
	PromptService               *biz.PromptService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	QuotaService                *biz.QuotaService
	HttpClient                  *httpclient.HttpClient
	LiveStreamRegistry          *biz.LiveStreamRegistry
	ChannelLimiterManager       *orchestrator.ChannelLimiterManager
	ProviderQuotaStatusProvider orchestrator.ProviderQuotaStatusProvider
	TimeoutConfig               TimeoutConfig
	SSEKeepAliveConfig          SSEKeepAliveConfig
}

type AnthropicHandlers struct {
	ChannelService         *biz.ChannelService
	ModelService           *biz.ModelService
	SystemService          *biz.SystemService
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	handler := (&ChatCompletionHandlers{
		ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
			params.ChannelService,
			params.DefaultSelector,
			params.RequestService,
			params.HttpClient,
			anthropic.NewInboundTransformer(),
			params.SystemService,
			params.UsageLogService,
			params.PromptService,
			params.QuotaService,
			params.PromptProtectionRuleService,
			params.LiveStreamRegistry,
			params.ChannelLimiterManager,
			params.ProviderQuotaStatusProvider,
		),
	}).WithTimeouts(params.TimeoutConfig)

	handler.sseKeepAlive = params.SSEKeepAliveConfig
	handler.sseHeartbeatFormat = sseHeartbeatAnthropic

	return &AnthropicHandlers{
		ChatCompletionHandlers: handler,
		ChannelService:         params.ChannelService,
		ModelService:           params.ModelService,
		SystemService:          params.SystemService,
	}
}

func (handlers *AnthropicHandlers) CreateMessage(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}

// CreateMessageWebSocket handles GET /v1/messages and /anthropic/v1/messages over WebSocket.
func (handlers *AnthropicHandlers) CreateMessageWebSocket(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletionWebSocket(c, WSFrameJSONEvents)
}

type AnthropicModel struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created"`
}

// ListModels returns all available models.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *AnthropicHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		requestID, _ := contexts.GetRequestID(ctx)
		status := http.StatusInternalServerError
		errorType := "internal_server_error"
		if errors.Is(err, entprivacy.Deny) {
			status = http.StatusForbidden
			errorType = "permission_error"
		}
		_ = c.Error(err)
		c.JSON(status, anthropic.AnthropicError{
			StatusCode: status,
			Type:       errorType,
			RequestID:  requestID,
			Error: anthropic.ErrorDetail{
				Type:    errorType,
				Message: err.Error(),
			},
		})

		return
	}

	anthropicModels := make([]AnthropicModel, 0, len(models))
	for _, model := range models {
		anthropicModels = append(anthropicModels, AnthropicModel{
			ID:          model.ID,
			Type:        "model",
			DisplayName: model.DisplayName,
			CreatedAt:   model.CreatedAt,
		})
	}

	var firstID string
	if len(anthropicModels) > 0 {
		firstID = anthropicModels[0].ID
	}

	var lastID string
	if len(anthropicModels) > 0 {
		lastID = anthropicModels[len(anthropicModels)-1].ID
	}

	c.JSON(http.StatusOK, gin.H{
		"object":   "list",
		"data":     anthropicModels,
		"has_more": false,
		"first_id": firstID,
		"last_id":  lastID,
	})
}

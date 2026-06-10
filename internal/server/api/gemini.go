package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
	"github.com/ldm2060/axonhub/llm/transformer/gemini"
)

type GeminiHandlersParams struct {
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
}

type GeminiHandlers struct {
	ChannelService         *biz.ChannelService
	ModelService           *biz.ModelService
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewGeminiHandlers(params GeminiHandlersParams) *GeminiHandlers {
	return &GeminiHandlers{
		ChatCompletionHandlers: NewChatCompletionHandlers(
			orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				gemini.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
				params.ChannelLimiterManager,
				params.ProviderQuotaStatusProvider,
			),
		).WithTimeouts(params.TimeoutConfig),
		ChannelService: params.ChannelService,
		ModelService:   params.ModelService,
	}
}

func (handlers *GeminiHandlers) GenerateContent(c *gin.Context) {
	alt := c.Query("alt")
	switch alt {
	case "sse":
		handlers.ChatCompletionHandlers.WithStreamWriter(WriteSSEStreamWithOptions).ChatCompletion(c)
	default:
		handlers.ChatCompletionHandlers.WithStreamWriter(WriteGeminiStreamWithOptions).ChatCompletion(c)
	}
}

func WriteGeminiStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteGeminiStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteGeminiStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	c.Header("Content-Type", "application/json; charset=UTF-8")

	_, _ = c.Writer.Write([]byte("["))

	first := true

	for {
		result := nextStreamEvent(ctx, stream, opts.IdleTimeout)
		if result.ok {
			cur := result.event

			if !first {
				_, _ = c.Writer.Write([]byte(","))
			}

			_, _ = c.Writer.Write(cur.Data)
			first = false

			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()
			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				clientDisconnected = true
				log.Warn(ctx, "Client disconnected, stop streaming", log.Cause(result.err))
				return
			}

			if errors.Is(result.err, ErrStreamIdleTimeout) {
				log.Warn(ctx, "Stream idle timeout, stopping Gemini stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
			} else {
				log.Error(ctx, "Error in stream", log.Cause(result.err))
			}
		}

		_, _ = c.Writer.Write([]byte("]"))
		return
	}
}

// GeminiModel represents a model in the list models response.
type GeminiModel struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// ListModels returns all available Gemini models.
// This endpoint is compatible with Gemini's /v1/models API.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *GeminiHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gemini.GeminiError{
			Error: gemini.ErrorDetail{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
				Status:  "internal_server_error",
			},
		})

		return
	}

	geminiModels := make([]GeminiModel, 0, len(models))
	for i, model := range models {
		geminiModels = append(geminiModels, GeminiModel{
			Name:                       "models/" + model.ID,
			BaseModelID:                model.ID,
			Version:                    fmt.Sprintf("%s-%d", model.ID, i),
			DisplayName:                model.DisplayName,
			Description:                model.DisplayName,
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"models": geminiModels,
	})
}

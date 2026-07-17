package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/request"
	"github.com/ldm2060/axonhub/internal/server/api"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/gql"
	"github.com/ldm2060/axonhub/internal/server/gql/openapi"
	"github.com/ldm2060/axonhub/internal/server/middleware"
	"github.com/ldm2060/axonhub/internal/server/static"
)

type Handlers struct {
	fx.In

	Graphql        *gql.GraphqlHandler
	OpenAPIGraphql *openapi.GraphqlHandler
	OpenAI         *api.OpenAIHandlers
	Doubao         *api.DoubaoHandlers
	Anthropic      *api.AnthropicHandlers
	Gemini         *api.GeminiHandlers
	AiSDK          *api.AiSDKHandlers
	Playground     *api.PlaygroundHandlers
	System         *api.SystemHandlers
	Auth           *api.AuthHandlers
	Avatar         *api.AvatarHandlers
	Jina           *api.JinaHandlers
	Codex          *api.CodexHandlers
	ClaudeCode     *api.ClaudeCodeHandlers
	Antigravity    *api.AntigravityHandlers
	Copilot        *api.CopilotHandlers
	KimiCode       *api.KimiCodeHandlers
	RequestContent *api.RequestContentHandlers
	OIDC           *api.OIDCHandlers
	RequestPreview *api.RequestPreviewHandlers
	SignUp         *api.SignUpHandlers
	EmailToken     *api.EmailTokenAPI
}

type Services struct {
	fx.In

	TraceService  *biz.TraceService
	ThreadService *biz.ThreadService
	AuthService   *biz.AuthService
	SystemService *biz.SystemService
}

func SetupRoutes(server *Server, handlers Handlers, client *ent.Client, services Services) {
	// IP 访问控制 - 全局最优先，拦截所有请求包括静态文件
	server.Use(middleware.WithIPAccessControl(
		server.Config.IPAccessControl.Enabled,
		server.Config.IPAccessControl.AllowedIPs,
		server.Config.IPAccessControl.RedirectURL,
	))

	// Serve static frontend files
	server.NoRoute(static.Handler())

	server.Use(middleware.AccessLog())
	server.Use(middleware.WithEntClient(client))
	server.Use(middleware.WithLoggingTracing(server.Config.Trace))
	server.Use(middleware.WithMetrics())

	// Setup CORS middleware at server level if enabled
	if server.Config.CORS.Enabled {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowOrigins = server.Config.CORS.AllowedOrigins
		corsConfig.AllowMethods = server.Config.CORS.AllowedMethods
		corsConfig.AllowHeaders = server.Config.CORS.AllowedHeaders
		corsConfig.ExposeHeaders = server.Config.CORS.ExposedHeaders
		corsConfig.AllowCredentials = server.Config.CORS.AllowCredentials
		corsConfig.MaxAge = server.Config.CORS.MaxAge

		corsHandler := cors.New(corsConfig)
		server.Use(corsHandler)
		server.OPTIONS("*any", corsHandler)
	}

	publicGroup := server.Group("", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// Favicon API - DO NOT AUTH
		publicGroup.GET("/favicon", handlers.System.GetFavicon)
		// Health check endpoint - no authentication required
		publicGroup.GET("/health", handlers.System.Health)
		publicGroup.GET("/avatars/:userID", handlers.Avatar.Serve)
	}

	// Auth routes (preferred, no /admin prefix)
	authGroup := server.Group("/auth", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		authGroup.POST("/signin", handlers.Auth.SignIn)
		authGroup.POST("/signup", handlers.SignUp.SignUp)
		authGroup.POST("/signup/verification-code", handlers.SignUp.SendVerificationCode)
		authGroup.GET("/signup/allowed", handlers.SignUp.AllowSignUp)
		authGroup.GET("/verify-email", handlers.EmailToken.VerifyEmail)
		authGroup.POST("/resend-verification", handlers.EmailToken.ResendVerification)
		authGroup.POST("/forgot-password", handlers.EmailToken.ForgotPassword)
		authGroup.POST("/reset-password", handlers.EmailToken.ResetPassword)
	}

	unSecureAdminGroup := server.Group("/admin", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		// System Status and Initialize - DO NOT AUTH
		unSecureAdminGroup.GET("/system/status", handlers.System.GetSystemStatus)
		unSecureAdminGroup.POST("/system/initialize", handlers.System.InitializeSystem)
		// Legacy auth routes (backward compatibility)
		unSecureAdminGroup.POST("/auth/signin", handlers.Auth.SignIn)
		unSecureAdminGroup.POST("/auth/signup", handlers.SignUp.SignUp)
		unSecureAdminGroup.POST("/auth/signup/verification-code", handlers.SignUp.SendVerificationCode)
		unSecureAdminGroup.GET("/auth/signup/allowed", handlers.SignUp.AllowSignUp)
		unSecureAdminGroup.GET("/auth/verify-email", handlers.EmailToken.VerifyEmail)
		unSecureAdminGroup.POST("/auth/resend-verification", handlers.EmailToken.ResendVerification)
		unSecureAdminGroup.POST("/auth/forgot-password", handlers.EmailToken.ForgotPassword)
		unSecureAdminGroup.POST("/auth/reset-password", handlers.EmailToken.ResetPassword)
	}

	oauthGroup := server.Group("/oauth", middleware.WithTimeout(server.Config.RequestTimeout))
	{
		handlers.OIDC.RegisterRoutes(oauthGroup)
	}

	adminGroup := server.Group("/admin", middleware.WithJWTAuth(services.AuthService), middleware.WithProjectID())
	// 管理员路由 - 使用 JWT 认证
	{
		adminGroup.POST("/me/avatar", handlers.Avatar.Upload)
		adminGroup.GET("/playground", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Playground.ServeHTTP(c.Writer, c.Request)
		})
		adminGroup.POST("/graphql", middleware.WithTimeout(server.Config.RequestTimeout), func(c *gin.Context) {
			handlers.Graphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})

		adminGroup.POST("/codex/oauth/start", handlers.Codex.StartOAuth)
		adminGroup.POST("/codex/oauth/exchange", handlers.Codex.Exchange)
		adminGroup.POST("/codex/auth/decode", handlers.Codex.DecodeAuthJSON)

		adminGroup.POST("/claudecode/oauth/start", handlers.ClaudeCode.StartOAuth)
		adminGroup.POST("/claudecode/oauth/exchange", handlers.ClaudeCode.Exchange)

		adminGroup.POST("/antigravity/oauth/start", handlers.Antigravity.StartOAuth)
		adminGroup.POST("/antigravity/oauth/exchange", handlers.Antigravity.Exchange)

		adminGroup.POST("/copilot/oauth/start", handlers.Copilot.StartOAuth)
		adminGroup.POST("/copilot/oauth/poll", handlers.Copilot.PollOAuth)

		adminGroup.POST("/kimicode/oauth/start", handlers.KimiCode.StartOAuth)
		adminGroup.POST("/kimicode/oauth/poll", handlers.KimiCode.PollOAuth)

		// OIDC Manual Linking
		adminGroup.GET("/oidc/link/:provider", handlers.OIDC.GetLinkAuthorizeURL)

		// Playground API with channel specification support
		adminGroup.POST(
			"/playground/chat",
			middleware.WithSource(request.SourcePlayground),
			handlers.Playground.ChatCompletion,
		)

		adminGroup.GET(
			"/requests/:request_id/content",
			middleware.WithTimeout(server.Config.RequestTimeout),
			handlers.RequestContent.DownloadRequestContent,
		)
		adminGroup.GET(
			"/requests/:request_id/preview",
			middleware.WithTimeout(server.Config.RequestTimeout),
			handlers.RequestPreview.PreviewRequest,
		)
	}

	openAPIGroup := server.Group(
		"/openapi",
		middleware.WithIPBlocklist(services.SystemService),
		middleware.WithOpenAPIAuth(services.AuthService),
		middleware.WithTimeout(server.Config.RequestTimeout),
	)
	{
		openAPIGroup.POST("/v1/graphql", func(c *gin.Context) {
			handlers.OpenAPIGraphql.Graphql.ServeHTTP(c.Writer, c.Request)
		})
		openAPIGroup.GET("/v1/playground", func(c *gin.Context) {
			handlers.OpenAPIGraphql.Playground.ServeHTTP(c.Writer, c.Request)
		})

		openAPIGroup.POST("/webhook/echo", handlers.System.WebhookEcho)
	}

	apiGroup := server.Group(
		"/",
		middleware.WithIPBlocklist(services.SystemService),
		middleware.WithAPIKeyConfig(services.AuthService, nil),
		middleware.WithSource(request.SourceAPI),
		middleware.WithThread(server.Config.Trace, services.ThreadService),
		middleware.WithTrace(server.Config.Trace, services.TraceService),
	)

	{
		openaiGroup := apiGroup.Group("/v1")
		openaiGroup.POST("/chat/completions", handlers.OpenAI.ChatCompletion)
		openaiGroup.POST("/completions", handlers.OpenAI.Completion)
		openaiGroup.POST("/responses/compact", handlers.OpenAI.CompactResponse)
		openaiGroup.POST("/responses", handlers.OpenAI.CreateResponse)
		openaiGroup.GET("/models", handlers.OpenAI.ListModels)
		openaiGroup.GET("/models/*model", handlers.OpenAI.RetrieveModel)
		openaiGroup.POST("/embeddings", handlers.OpenAI.CreateEmbedding)
		openaiGroup.POST("/images/generations", handlers.OpenAI.CreateImage)
		openaiGroup.POST("/images/edits", handlers.OpenAI.CreateImageEdit)
		openaiGroup.POST("/videos", handlers.OpenAI.CreateVideo)
		openaiGroup.GET("/videos/:id", handlers.OpenAI.GetVideo)
		openaiGroup.DELETE("/videos/:id", handlers.OpenAI.DeleteVideo)
		openaiGroup.POST("/audio/speech", handlers.OpenAI.CreateSpeech)
		openaiGroup.POST("/audio/transcriptions", handlers.OpenAI.CreateTranscription)
		openaiGroup.POST("/audio/translations", handlers.OpenAI.CreateTranslation)
		// DO NOT SUPPORT IMAGE VARIATION
		// openaiGroup.POST("/images/variations", handlers.OpenAI.CreateImageVariation)

		// OpenAI-compatible Anthropic endpoint
		openaiGroup.POST("/messages", handlers.Anthropic.CreateMessage)

		// WebSocket upgrade routes (GET only â WS handshake is an HTTP Upgrade)
		openaiGroup.GET("/chat/completions", handlers.OpenAI.ChatCompletionWebSocket)
		openaiGroup.GET("/responses", handlers.OpenAI.CreateResponseWebSocket)
		openaiGroup.GET("/responses/compact", handlers.OpenAI.CompactResponseWebSocket)
		openaiGroup.GET("/messages", handlers.Anthropic.CreateMessageWebSocket)

		// Compatible with OpenAI API
		openaiGroup.POST("/rerank", handlers.Jina.Rerank)
	}

	{
		jinaGroup := apiGroup.Group("/jina/v1")
		jinaGroup.POST("/embeddings", handlers.Jina.CreateEmbedding)
		jinaGroup.POST("/rerank", handlers.Jina.Rerank)
	}

	{
		anthropicGroup := apiGroup.Group("/anthropic/v1")
		anthropicGroup.POST("/messages", handlers.Anthropic.CreateMessage)
		anthropicGroup.GET("/messages", handlers.Anthropic.CreateMessageWebSocket)
		anthropicGroup.GET("/models", handlers.Anthropic.ListModels)
	}

	{
		doubaoGroup := apiGroup.Group("/doubao/v3")
		doubaoGroup.POST("/contents/generations/tasks", handlers.Doubao.CreateTask)
		doubaoGroup.GET("/contents/generations/tasks/:id", handlers.Doubao.GetTask)
		doubaoGroup.DELETE("/contents/generations/tasks/:id", handlers.Doubao.DeleteTask)
	}

	{
		registerGeminiRoutes := func(group *gin.RouterGroup) {
			group.POST("/models/*action", handlers.Gemini.GenerateContent)
			group.GET("/models/*action", handlers.Gemini.GenerateContentWebSocket)
			group.GET("/models", handlers.Gemini.ListModels)
		}

		geminiGroup := server.Group(
			"/gemini/:gemini-api-version",
			middleware.WithIPBlocklist(services.SystemService),
			middleware.WithGeminiKeyAuth(services.AuthService),
			middleware.WithSource(request.SourceAPI),
			middleware.WithThread(server.Config.Trace, services.ThreadService),
			middleware.WithTrace(server.Config.Trace, services.TraceService),
		)

		registerGeminiRoutes(geminiGroup)

		// Alias for Gemini API
		geminiAliasGroup := server.Group(
			"/v1beta",
			middleware.WithIPBlocklist(services.SystemService),
			middleware.WithGeminiKeyAuth(services.AuthService),
			middleware.WithSource(request.SourceAPI),
			middleware.WithThread(server.Config.Trace, services.ThreadService),
			middleware.WithTrace(server.Config.Trace, services.TraceService),
		)

		registerGeminiRoutes(geminiAliasGroup)
	}
}

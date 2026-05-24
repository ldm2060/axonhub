# Align GitHub Copilot Channel with OpenCode Implementation

## Background

OpenCode (https://github.com/anomalyco/opencode) implements GitHub Copilot support with several capabilities that AxonHub's current implementation lacks. This spec covers aligning AxonHub's `github_copilot` channel type with OpenCode's feature set.

### Current State

AxonHub's Copilot implementation supports:
- OAuth Device Flow authentication (same client ID as OpenCode)
- OpenAI Chat Completions API (`/chat/completions`) for most models
- OpenAI Responses API (`/responses`) for GPT-5+ models
- Copilot-specific headers (`Openai-Intent`, `X-Initiator`, `Copilot-Vision-Request`)
- Model fetching from `/models` endpoint with basic filtering

### Gap Analysis

OpenCode features AxonHub lacks:
1. **Anthropic `/v1/messages` passthrough** - Models with `/v1/messages` in `supported_endpoints` route directly to `{baseURL}/v1/messages` using Anthropic Messages API format
2. **Adaptive thinking / reasoning variants** - Generate model variants from `adaptive_thinking`, `reasoning_effort`, and `max_thinking_budget` capabilities
3. **Anthropic-specific parameter modifications** - Disable tool streaming for Anthropic models through Copilot, set `anthropic-beta` header
4. **GPT maxOutputTokens omission** - Match Copilot CLI behavior by omitting `maxOutputTokens` for GPT models

## Design

### 1. Anthropic `/v1/messages` Passthrough

**Model Detection:**

When fetching models from `{baseURL}/models`, check each model's `supported_endpoints` array. If it contains `/v1/messages`, flag the model as Anthropic-compatible.

Extend `CopilotModel` to capture this:

```go
type CopilotModel struct {
    // ... existing fields ...
    SupportedEndpoints []string `json:"supported_endpoints"`
}
```

Add a helper:

```go
func (m CopilotModel) SupportsAnthropicMessages() bool {
    for _, ep := range m.SupportedEndpoints {
        if ep == "/v1/messages" {
            return true
        }
    }
    return false
}
```

**Model Metadata Storage:**

Store per-model endpoint info in the channel's model metadata. Add a `CopilotModelInfo` struct that gets cached alongside the model list:

```go
type CopilotModelInfo struct {
    SupportedEndpoints       []string
    SupportsAdaptiveThinking bool
    ReasoningEffort          []string
    MaxThinkingBudget        int
    MaxContextWindowTokens   int
    MaxOutputTokens          int
    SupportsVision           bool
    SupportsToolCalls        bool
    SupportsStreaming        bool
    SupportsStructuredOutputs bool
}
```

This gets stored in a map keyed by model ID, accessible to the transformer.

**Transformer Routing:**

The `OutboundTransformer` already routes between Chat Completions and Responses API. Extend this to a third route:

```
TransformRequest(model):
  1. Check model metadata for endpoint preference
  2. If usesResponsesAPI(model) → /responses
  3. If model usesAnthropicMessages → /v1/messages via Anthropic transformer
  4. Default → /chat/completions via OpenAI transformer
```

Add an Anthropic outbound transformer delegate to the Copilot transformer:

```go
type OutboundTransformer struct {
    tokenProvider        TokenProvider
    baseURL              string
    responses            *responses.OutboundTransformer    // for /responses
    openAITransformer    transformer.Outbound               // for /chat/completions (lazy)
    anthropicTransformer transformer.Outbound               // for /v1/messages (lazy)
    modelInfo            map[string]*CopilotModelInfo       // per-model metadata
}
```

**Anthropic Transformer Configuration:**

When routing to `/v1/messages`, create an Anthropic outbound transformer with:
- `BaseURL`: `{copilotBaseURL}/v1` (e.g., `https://api.githubcopilot.com/v1`)
- `EndpointPath`: leave empty — the Anthropic transformer automatically appends its default path `/messages`, producing the correct final URL `https://api.githubcopilot.com/v1/messages`
- Auth: Bearer token from Copilot token provider (override the Anthropic transformer's own auth config, since Copilot uses its own OAuth token, not an Anthropic API key)

**Response Handling:**

In `TransformResponse` and `TransformStream`, detect Anthropic-formatted responses:
- Anthropic responses have `type: "message"` or use Anthropic SSE event types (`message_start`, `content_block_start`, etc.)
- Route to Anthropic transformer for parsing

**Stream Detection:**

For streaming, peek at the first SSE event:
- `response.` prefix → Responses API
- `message_start` / `content_block_start` → Anthropic Messages
- Default → OpenAI Chat Completions

### 2. Adaptive Thinking / Reasoning Variants

**Capability Parsing:**

Extend `CopilotModel` to parse all thinking-related capabilities:

```go
type CopilotModel struct {
    // ... existing fields ...
    Capabilities struct {
        // ... existing fields ...
        Supports struct {
            // ... existing fields ...
            AdaptiveThinking  bool     `json:"adaptive_thinking"`
            ReasoningEffort   []string `json:"reasoning_effort"`
        } `json:"supports"`
    } `json:"capabilities"`
}
```

**Variant Generation Logic (matching OpenCode):**

For each model that has thinking capabilities, generate variant model entries:

1. **Non-Anthropic models with `reasoning_effort`**: Create variants for each effort level with `reasoning_effort` and `reasoning_summary: "auto"` parameters. These map to the OpenAI `reasoning_effort` parameter.

2. **Anthropic models with `adaptive_thinking` + `reasoning_effort`**: Create variants for each effort level with `thinking: { type: "adaptive", effort: "<level>" }`. Opus 4.7 models additionally get `display: "summarized"`.

3. **Anthropic models with only `max_thinking_budget`** (no adaptive_thinking): Create `max` variant with `budgetTokens: max-1` and `high` variant with `budgetTokens: floor(max/2)`.

**Implementation Approach:**

Rather than creating separate model entries for each variant, store variant info in the model metadata. When a request comes in for a variant model (e.g., `claude-sonnet-4-20250514:thinking`), the transformer:
1. Strips the variant suffix to get the base model ID
2. Looks up the variant config from model metadata
3. Applies the thinking/reasoning parameters to the request before sending

Variant model IDs use a `:` separator to match common convention:
- `gpt-4.1:low`, `gpt-4.1:medium`, `gpt-4.1:high`
- `claude-sonnet-4-20250514:low`, `claude-sonnet-4-20250514:adaptive`

### 3. Anthropic-Specific Request Modifications

**Header Injection:**

For Anthropic models routed through Copilot, inject the `anthropic-beta` header:

```
anthropic-beta: interleaved-thinking-2025-05-14
```

This enables interleaved thinking for tool use, matching OpenCode's behavior.

**Implementation:** In the Copilot transformer's `transformAnthropicMessagesRequest`, after delegating to the Anthropic transformer, override headers:
- Set `anthropic-beta: interleaved-thinking-2025-05-14`
- Keep Copilot-specific headers (`Authorization`, `Openai-Intent`, etc.)
- Remove any Anthropic API key headers (Copilot uses its own auth)

**Tool Streaming Disable:**

For Anthropic models through Copilot, the Copilot `/v1/messages` shim rejects the `eager_input_streaming` field. When building the Anthropic request, ensure tool streaming is disabled.

**Implementation:** When creating the Anthropic transformer delegate or in the request transformation step, strip the `eager_input_streaming` / tool streaming fields from the request body.

### 4. GPT maxOutputTokens Omission

**Matching Copilot CLI behavior:** For GPT models, omit `maxOutputTokens` / `max_tokens` from the request body.

**Implementation:** In `TransformRequest`, after building the OpenAI request, check if the model name contains "gpt" and remove `max_tokens` / `max_output_tokens` from the serialized body.

### 5. Default Endpoint Registration

**Current:** Copilot channel only registers `openai/chat_completions` endpoint.

**New:** Register both `openai/chat_completions` and `anthropic/messages` endpoints:

```go
channel.TypeGithubCopilot: {
    {APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
    {APIFormat: llm.APIFormatAnthropicMessage.String()},
},
```

This allows clients to call the Copilot channel through either the OpenAI or Anthropic inbound API.

### 6. Model Fetching Enhancement

Extend `FetchModels` to return `CopilotModelInfo` alongside model IDs. Store this in the channel's cached model metadata so the transformer can access it.

The channel model sync (`channel_model_sync.go`) should store `CopilotModelInfo` as part of the model entry metadata, making it available at request routing time.

## Files to Modify

| File | Change |
|------|--------|
| `llm/transformer/openai/copilot/models.go` | Extend `CopilotModel` with `SupportedEndpoints`, `AdaptiveThinking`, `ReasoningEffort`, `MaxThinkingBudget`. Return model info map from `FetchModels`. |
| `llm/transformer/openai/copilot/outbound.go` | Add Anthropic transformer delegate. Implement `transformAnthropicMessagesRequest`. Add model info routing logic in `TransformRequest`. Add Anthropic response/stream detection in `TransformResponse`/`TransformStream`. Strip `maxOutputTokens` for GPT models. |
| `internal/server/biz/channel_endpoint.go` | Add `anthropic/messages` as a default endpoint for `TypeGithubCopilot`. |
| `internal/server/biz/channel_llm.go` | Pass model info to transformer during construction. |
| `internal/server/biz/model_fetcher.go` | Store `CopilotModelInfo` during model sync. |

## Out of Scope

- GitHub Enterprise support (deferred)
- SSE timeout wrapper (AxonHub already has retry/circuit-breaker mechanisms)
- Sub-agent detection beyond what we already have (current `inferCopilotInitiator` covers the main cases)
- Plugin-based architecture (AxonHub uses a different pattern than OpenCode)

## Testing Strategy

1. **Unit tests** for model detection (`SupportsAnthropicMessages`, variant generation)
2. **Unit tests** for the routing logic in `TransformRequest` (which format each model type gets)
3. **Unit tests** for Anthropic response/stream parsing through the Copilot transformer
4. **Integration tests** with mocked Copilot API responses for all three API formats
5. **Manual testing** with actual Copilot token against Anthropic models (claude-sonnet-4) and GPT models

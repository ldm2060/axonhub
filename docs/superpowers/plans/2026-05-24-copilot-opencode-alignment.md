# Align GitHub Copilot Channel with OpenCode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align AxonHub's GitHub Copilot channel with OpenCode's implementation, adding Anthropic `/v1/messages` passthrough, adaptive thinking/reasoning variants, and request parameter adjustments.

**Architecture:** Extend the existing `copilot` outbound transformer to support three API formats: OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages. Model metadata from `/models` endpoint drives routing decisions. A lazy-initialized Anthropic transformer delegate handles the new format.

**Tech Stack:** Go 1.26+, Ent ORM, existing llm/transformer pipeline

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `llm/transformer/openai/copilot/models.go` | Modify | Extend `CopilotModel` with `SupportedEndpoints`, thinking capabilities; add `CopilotModelInfo` struct and variant generation |
| `llm/transformer/openai/copilot/models_test.go` | Modify | Tests for model detection, variant generation |
| `llm/transformer/openai/copilot/outbound.go` | Modify | Add Anthropic transformer delegate, routing logic, request/response handling for `/v1/messages` |
| `llm/transformer/openai/copilot/outbound_test.go` | Modify | Tests for Anthropic routing, GPT maxOutputTokens omission |
| `llm/transformer/openai/copilot/token_provider.go` | No change | Already provides Bearer token — reused for Anthropic passthrough |
| `internal/server/biz/channel_endpoint.go` | Modify | Add `anthropic/messages` as default endpoint for `TypeGithubCopilot` |
| `internal/server/biz/channel_llm.go` | Modify | Pass `CopilotModelInfo` map to transformer during construction |
| `internal/server/biz/model_fetcher.go` | Modify | Store `CopilotModelInfo` during model sync |

---

### Task 1: Extend CopilotModel with endpoint and thinking capabilities

**Files:**
- Modify: `llm/transformer/openai/copilot/models.go`
- Test: `llm/transformer/openai/copilot/models_test.go`

- [ ] **Step 1: Write failing tests for new model fields**

Add tests in `models_test.go`:

```go
func TestCopilotModel_SupportsAnthropicMessages(t *testing.T) {
    tests := []struct {
        name     string
        model    CopilotModel
        expected bool
    }{
        {
            name: "has v1/messages endpoint",
            model: CopilotModel{SupportedEndpoints: []string{"/v1/messages", "/chat/completions"}},
            expected: true,
        },
        {
            name: "no v1/messages endpoint",
            model: CopilotModel{SupportedEndpoints: []string{"/chat/completions"}},
            expected: false,
        },
        {
            name: "nil endpoints",
            model: CopilotModel{SupportedEndpoints: nil},
            expected: false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.model.SupportsAnthropicMessages())
        })
    }
}

func TestCopilotModel_HasAdaptiveThinking(t *testing.T) {
    m := CopilotModel{
        Capabilities: CopilotModelCapabilities{
            Supports: CopilotModelSupports{
                AdaptiveThinking: true,
                ReasoningEffort:  []string{"low", "medium", "high"},
            },
        },
    }
    assert.True(t, m.HasAdaptiveThinking())
    assert.Equal(t, []string{"low", "medium", "high"}, m.ReasoningEfforts())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestCopilotModel_SupportsAnthropicMessages|TestCopilotModel_HasAdaptiveThinking" -v`
Expected: FAIL — `SupportedEndpoints`, `SupportsAnthropicMessages`, `HasAdaptiveThinking`, `ReasoningEfforts` not defined

- [ ] **Step 3: Extend CopilotModel struct in models.go**

Add the new fields to `CopilotModel`:

```go
type CopilotModel struct {
    ID               string   `json:"id"`
    Name             string   `json:"name"`
    ModelPickerLabel string   `json:"model_picker_label"`
    Vendor           string   `json:"vendor"`
    Policy           struct {
        State string `json:"state"`
    } `json:"policy"`
    Capabilities CopilotModelCapabilities `json:"capabilities"`
    SupportedEndpoints []string `json:"supported_endpoints"`
}

type CopilotModelCapabilities struct {
    Family string `json:"family"`
    Limits struct {
        MaxContextWindowTokens int `json:"max_context_window_tokens"`
        MaxOutputTokens       int `json:"max_output_tokens"`
    } `json:"limits"`
    Supports CopilotModelSupports `json:"supports"`
    Thinking bool `json:"thinking"`
}

type CopilotModelSupports struct {
    Streaming             bool     `json:"streaming"`
    ParallelToolCalls     bool     `json:"parallel_tool_calls"`
    ToolCalls             bool     `json:"tool_calls"`
    Vision                bool     `json:"vision"`
    StructuredOutputs     bool     `json:"structured_outputs"`
    AdaptiveThinking      bool     `json:"adaptive_thinking"`
    ReasoningEffort       []string `json:"reasoning_effort"`
    MaxThinkingBudget     int      `json:"max_thinking_budget"`
    InputModalities       []string `json:"input_modalities"`
    OutputModalities      []string `json:"output_modalities"`
}
```

Add helper methods:

```go
func (m CopilotModel) SupportsAnthropicMessages() bool {
    for _, ep := range m.SupportedEndpoints {
        if ep == "/v1/messages" {
            return true
        }
    }
    return false
}

func (m CopilotModel) HasAdaptiveThinking() bool {
    return m.Capabilities.Supports.AdaptiveThinking
}

func (m CopilotModel) ReasoningEfforts() []string {
    return m.Capabilities.Supports.ReasoningEffort
}

func (m CopilotModel) MaxThinkingBudget() int {
    return m.Capabilities.Supports.MaxThinkingBudget
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestCopilotModel_SupportsAnthropicMessages|TestCopilotModel_HasAdaptiveThinking" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/models.go llm/transformer/openai/copilot/models_test.go
git commit -m "feat(copilot): extend CopilotModel with endpoint and thinking capabilities"
```

---

### Task 2: Add CopilotModelInfo and variant generation

**Files:**
- Modify: `llm/transformer/openai/copilot/models.go`
- Test: `llm/transformer/openai/copilot/models_test.go`

- [ ] **Step 1: Write failing tests for CopilotModelInfo and variant generation**

```go
func TestBuildModelInfoMap(t *testing.T) {
    models := []CopilotModel{
        {
            ID: "gpt-4.1",
            SupportedEndpoints: []string{"/chat/completions"},
            Capabilities: CopilotModelCapabilities{
                Supports: CopilotModelSupports{
                    ReasoningEffort: []string{"low", "medium", "high"},
                },
            },
        },
        {
            ID: "claude-sonnet-4-20250514",
            SupportedEndpoints: []string{"/v1/messages", "/chat/completions"},
            Capabilities: CopilotModelCapabilities{
                Supports: CopilotModelSupports{
                    AdaptiveThinking:  true,
                    ReasoningEffort:   []string{"low", "medium", "high"},
                },
            },
        },
    }
    infoMap := BuildModelInfoMap(models)

    assert.NotNil(t, infoMap["gpt-4.1"])
    assert.False(t, infoMap["gpt-4.1"].SupportsAnthropicMessages)
    assert.Equal(t, []string{"low", "medium", "high"}, infoMap["gpt-4.1"].ReasoningEfforts)

    assert.NotNil(t, infoMap["claude-sonnet-4-20250514"])
    assert.True(t, infoMap["claude-sonnet-4-20250514"].SupportsAnthropicMessages)
    assert.True(t, infoMap["claude-sonnet-4-20250514"].SupportsAdaptiveThinking)
}

func TestGenerateVariants_OpenAIReasoning(t *testing.T) {
    info := &CopilotModelInfo{
        ModelID:         "gpt-4.1",
        ReasoningEfforts: []string{"low", "medium", "high"},
    }
    variants := GenerateVariants(info)
    assert.Len(t, variants, 3)
    assert.Equal(t, "gpt-4.1:low", variants[0].ModelID)
    assert.Equal(t, "reasoning", variants[0].Type)
    assert.Equal(t, "low", variants[0].Effort)
}

func TestGenerateVariants_AnthropicAdaptive(t *testing.T) {
    info := &CopilotModelInfo{
        ModelID:              "claude-sonnet-4-20250514",
        SupportsAdaptiveThinking: true,
        ReasoningEfforts:     []string{"low", "medium", "high"},
    }
    variants := GenerateVariants(info)
    assert.Len(t, variants, 3)
    assert.Equal(t, "claude-sonnet-4-20250514:low", variants[0].ModelID)
    assert.Equal(t, "adaptive", variants[0].Type)
}

func TestGenerateVariants_AnthropicBudget(t *testing.T) {
    info := &CopilotModelInfo{
        ModelID:           "claude-opus-4-20250514",
        MaxThinkingBudget: 10000,
    }
    variants := GenerateVariants(info)
    assert.Len(t, variants, 2)
    assert.Equal(t, "claude-opus-4-20250514:high", variants[0].ModelID)
    assert.Equal(t, 4999, variants[0].BudgetTokens)
    assert.Equal(t, "claude-opus-4-20250514:max", variants[1].ModelID)
    assert.Equal(t, 9999, variants[1].BudgetTokens)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestBuildModelInfoMap|TestGenerateVariants" -v`
Expected: FAIL — types/functions not defined

- [ ] **Step 3: Implement CopilotModelInfo and variant generation**

Add to `models.go`:

```go
type CopilotModelInfo struct {
    ModelID                    string
    SupportedEndpoints         []string
    SupportsAnthropicMessages  bool
    SupportsAdaptiveThinking   bool
    ReasoningEfforts           []string
    MaxThinkingBudget          int
    MaxContextWindowTokens     int
    MaxOutputTokens            int
    SupportsVision             bool
    SupportsToolCalls          bool
    SupportsStreaming          bool
    SupportsStructuredOutputs  bool
    IsOpus                     bool
}

type ModelVariant struct {
    ModelID      string
    DisplayName  string
    Type         string // "reasoning" or "adaptive" or "budget"
    Effort       string // "low", "medium", "high"
    BudgetTokens int    // for budget-based variants
}

func BuildModelInfoMap(models []CopilotModel) map[string]*CopilotModelInfo {
    m := make(map[string]*CopilotModelInfo, len(models))
    for _, model := range models {
        info := &CopilotModelInfo{
            ModelID:                   model.ID,
            SupportedEndpoints:        model.SupportedEndpoints,
            SupportsAnthropicMessages: model.SupportsAnthropicMessages(),
            SupportsAdaptiveThinking:  model.HasAdaptiveThinking(),
            ReasoningEfforts:          model.ReasoningEfforts(),
            MaxThinkingBudget:         model.MaxThinkingBudget(),
            MaxContextWindowTokens:    model.Capabilities.Limits.MaxContextWindowTokens,
            MaxOutputTokens:           model.Capabilities.Limits.MaxOutputTokens,
            SupportsVision:            model.Capabilities.Supports.Vision,
            SupportsToolCalls:         model.Capabilities.Supports.ToolCalls,
            SupportsStreaming:         model.Capabilities.Supports.Streaming,
            SupportsStructuredOutputs: model.Capabilities.Supports.StructuredOutputs,
            IsOpus:                    strings.Contains(model.ID, "opus"),
        }
        m[model.ID] = info
    }
    return m
}

func GenerateVariants(info *CopilotModelInfo) []ModelVariant {
    var variants []ModelVariant

    if len(info.ReasoningEfforts) > 0 {
        for _, effort := range info.ReasoningEfforts {
            if info.SupportsAdaptiveThinking {
                variant := ModelVariant{
                    ModelID:     info.ModelID + ":" + effort,
                    DisplayName: effort,
                    Type:        "adaptive",
                    Effort:      effort,
                }
                if info.IsOpus && effort == "high" {
                    variant.DisplayName = "summarized"
                }
                variants = append(variants, variant)
            } else {
                variants = append(variants, ModelVariant{
                    ModelID:     info.ModelID + ":" + effort,
                    DisplayName: effort,
                    Type:        "reasoning",
                    Effort:      effort,
                })
            }
        }
    } else if info.MaxThinkingBudget > 0 {
        variants = append(variants,
            ModelVariant{
                ModelID:      info.ModelID + ":high",
                DisplayName:  "high",
                Type:         "budget",
                Effort:       "high",
                BudgetTokens: info.MaxThinkingBudget / 2,
            },
            ModelVariant{
                ModelID:      info.ModelID + ":max",
                DisplayName:  "max",
                Type:         "budget",
                Effort:       "max",
                BudgetTokens: info.MaxThinkingBudget - 1,
            },
        )
    }

    return variants
}
```

Add `"strings"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestBuildModelInfoMap|TestGenerateVariants" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/models.go llm/transformer/openai/copilot/models_test.go
git commit -m "feat(copilot): add CopilotModelInfo and model variant generation"
```

---

### Task 3: Update FetchModels to return model info and variant models

**Files:**
- Modify: `llm/transformer/openai/copilot/models.go`
- Test: `llm/transformer/openai/copilot/models_test.go`

- [ ] **Step 1: Write failing test for FetchModels returning model info**

```go
func TestFetchModels_ReturnsModelInfo(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data": []map[string]interface{}{
                {
                    "id":                "gpt-4.1",
                    "model_picker_enabled": true,
                    "policy":            map[string]string{"state": "enabled"},
                    "capabilities": map[string]interface{}{
                        "supports": map[string]interface{}{
                            "reasoning_effort": []string{"low", "medium", "high"},
                        },
                    },
                },
                {
                    "id":                "claude-sonnet-4-20250514",
                    "model_picker_enabled": true,
                    "policy":            map[string]string{"state": "enabled"},
                    "supported_endpoints": []string{"/v1/messages", "/chat/completions"},
                    "capabilities": map[string]interface{}{
                        "supports": map[string]interface{}{
                            "adaptive_thinking": true,
                        },
                    },
                },
                {
                    "id":                "disabled-model",
                    "model_picker_enabled": false,
                    "policy":            map[string]string{"state": "enabled"},
                },
            },
        })
    }))
    defer server.Close()

    models, infoMap, err := FetchModelsWithInfo(context.Background(), server.URL, "test-token")
    assert.NoError(t, err)

    assert.Len(t, models, 2) // disabled-model filtered out
    assert.NotNil(t, infoMap["gpt-4.1"])
    assert.False(t, infoMap["gpt-4.1"].SupportsAnthropicMessages)
    assert.NotNil(t, infoMap["claude-sonnet-4-20250514"])
    assert.True(t, infoMap["claude-sonnet-4-20250514"].SupportsAnthropicMessages)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestFetchModels_ReturnsModelInfo" -v`
Expected: FAIL — `FetchModelsWithInfo` not defined

- [ ] **Step 3: Implement FetchModelsWithInfo**

Add to `models.go`:

```go
func FetchModelsWithInfo(ctx context.Context, baseURL, token string) ([]string, map[string]*CopilotModelInfo, error) {
    copilotModels, err := fetchCopilotModels(ctx, baseURL, token)
    if err != nil {
        return nil, nil, err
    }

    var modelIDs []string
    for _, m := range copilotModels {
        modelIDs = append(modelIDs, m.ID)
    }

    infoMap := BuildModelInfoMap(copilotModels)
    return modelIDs, infoMap, nil
}
```

Rename the current `FetchModels` internals to `fetchCopilotModels` that returns `[]CopilotModel`, then have `FetchModels` call it and return just the IDs for backward compatibility. Update the existing `FetchModels`:

```go
func FetchModels(ctx context.Context, baseURL, token string) ([]string, error) {
    models, _, err := FetchModelsWithInfo(ctx, baseURL, token)
    return models, err
}

func fetchCopilotModels(ctx context.Context, baseURL, token string) ([]CopilotModel, error) {
    // ... existing FetchModels body, but return []CopilotModel instead of []string
    // Keep the same filtering logic (model_picker_enabled, policy.state)
}
```

The `fetchCopilotModels` function body is the current `FetchModels` body, but instead of appending `m.ID` to a string slice, it appends the full `CopilotModel` to a slice. The filtering logic (`model_picker_enabled == true` and `policy.state == "enabled"`) stays the same.

- [ ] **Step 4: Run all copilot model tests**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestFetchModels" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/models.go llm/transformer/openai/copilot/models_test.go
git commit -m "feat(copilot): add FetchModelsWithInfo returning model metadata"
```

---

### Task 4: Add Anthropic transformer delegate to OutboundTransformer

**Files:**
- Modify: `llm/transformer/openai/copilot/outbound.go`
- Test: `llm/transformer/openai/copilot/outbound_test.go`

- [ ] **Step 1: Write failing test for Anthropic routing decision**

```go
func TestOutboundTransformer_RouteToAnthropic(t *testing.T) {
    infoMap := map[string]*CopilotModelInfo{
        "claude-sonnet-4-20250514": {
            ModelID:                   "claude-sonnet-4-20250514",
            SupportsAnthropicMessages: true,
        },
        "gpt-4.1": {
            ModelID: "gpt-4.1",
        },
    }

    transformer := &OutboundTransformer{
        modelInfo: infoMap,
    }

    assert.True(t, transformer.usesAnthropicMessages("claude-sonnet-4-20250514"))
    assert.False(t, transformer.usesAnthropicMessages("gpt-4.1"))
    assert.False(t, transformer.usesAnthropicMessages("unknown-model"))
}

func TestOutboundTransformer_AnthropicBaseURL(t *testing.T) {
    transformer := &OutboundTransformer{
        baseURL: "https://api.githubcopilot.com",
    }
    assert.Equal(t, "https://api.githubcopilot.com/v1", transformer.anthropicBaseURL())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_RouteToAnthropic|TestOutboundTransformer_AnthropicBaseURL" -v`
Expected: FAIL — `modelInfo` field and methods not defined

- [ ] **Step 3: Add modelInfo field and routing methods to OutboundTransformer**

In `outbound.go`, add `modelInfo` field to `OutboundTransformer`:

```go
type OutboundTransformer struct {
    tokenProvider        TokenProvider
    baseURL              string
    responses            *responses.OutboundTransformer
    anthropicTransformer anthropic.Outbound // lazy-initialized
    anthropicOnce        sync.Once
    modelInfo            map[string]*CopilotModelInfo
}
```

Update `Config` to include `ModelInfo`:

```go
type Config struct {
    TokenProvider TokenProvider
    BaseURL       string
    ModelInfo     map[string]*CopilotModelInfo
}
```

Update `NewOutboundTransformer` to set `modelInfo`:

```go
func NewOutboundTransformer(cfg Config) *OutboundTransformer {
    // ... existing code ...
    return &OutboundTransformer{
        tokenProvider: cfg.TokenProvider,
        baseURL:       cfg.BaseURL,
        modelInfo:     cfg.ModelInfo,
    }
}
```

Add routing helper methods:

```go
func (t *OutboundTransformer) usesAnthropicMessages(model string) bool {
    info, ok := t.modelInfo[model]
    return ok && info.SupportsAnthropicMessages
}

func (t *OutboundTransformer) anthropicBaseURL() string {
    return strings.TrimRight(t.baseURL, "/") + "/v1"
}

func (t *OutboundTransformer) getAnthropicTransformer() anthropic.Outbound {
    t.anthropicOnce.Do(func() {
        cfg := anthropic.Config{
            BaseURL: t.anthropicBaseURL(),
        }
        t.anthropicTransformer = anthropic.NewOutboundTransformer(cfg)
    })
    return t.anthropicTransformer
}
```

Add `"sync"` and `"strings"` to imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_RouteToAnthropic|TestOutboundTransformer_AnthropicBaseURL" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/outbound.go llm/transformer/openai/copilot/outbound_test.go
git commit -m "feat(copilot): add Anthropic routing decision and model info to transformer"
```

---

### Task 5: Implement Anthropic Messages request transformation

**Files:**
- Modify: `llm/transformer/openai/copilot/outbound.go`
- Test: `llm/transformer/openai/copilot/outbound_test.go`

- [ ] **Step 1: Write failing test for Anthropic request transformation**

```go
func TestOutboundTransformer_TransformRequest_Anthropic(t *testing.T) {
    tp := &mockTokenProvider{token: "copilot-token-123"}
    infoMap := map[string]*CopilotModelInfo{
        "claude-sonnet-4-20250514": {
            ModelID:                   "claude-sonnet-4-20250514",
            SupportsAnthropicMessages: true,
        },
    }

    transformer := NewOutboundTransformer(Config{
        TokenProvider: tp,
        BaseURL:       "https://api.githubcopilot.com",
        ModelInfo:     infoMap,
    })

    req := &llm.Request{
        Model: "claude-sonnet-4-20250514",
        Messages: []llm.Message{
            {Role: llm.RoleUser, Content: llm.TextContent{Text: "Hello"}},
        },
    }

    result, err := transformer.TransformRequest(context.Background(), req)
    assert.NoError(t, err)
    assert.Equal(t, "https://api.githubcopilot.com/v1/messages", result.URL)
    assert.Equal(t, "Bearer copilot-token-123", result.Header.Get("Authorization"))
    assert.Equal(t, "interleaved-thinking-2025-05-14", result.Header.Get("anthropic-beta"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformRequest_Anthropic" -v`
Expected: FAIL — Anthropic routing not yet wired into TransformRequest

- [ ] **Step 3: Implement Anthropic request transformation in TransformRequest**

Update `TransformRequest` to add Anthropic routing before the existing logic:

```go
func (t *OutboundTransformer) TransformRequest(ctx context.Context, req *llm.Request) (*llm.HTTPRequest, error) {
    token, err := t.tokenProvider.GetToken(ctx)
    if err != nil {
        return nil, err
    }

    // Strip variant suffix if present (e.g., "claude-sonnet-4:high" -> "claude-sonnet-4")
    baseModel, variant := parseModelVariant(req.Model)
    if variant != nil {
        req = t.applyVariant(req, variant)
        req.Model = baseModel
    }

    // Route to Anthropic Messages API
    if t.usesAnthropicMessages(baseModel) {
        return t.transformAnthropicRequest(ctx, req, token)
    }

    // ... existing Responses API and Chat Completions routing ...
}
```

Add the Anthropic transformation method:

```go
func (t *OutboundTransformer) transformAnthropicRequest(ctx context.Context, req *llm.Request, token string) (*llm.HTTPRequest, error) {
    // Use the Anthropic transformer to build the request body
    delegate := t.getAnthropicTransformer()
    result, err := delegate.TransformRequest(ctx, req)
    if err != nil {
        return nil, err
    }

    // Override URL to Copilot's Anthropic endpoint
    result.URL = t.anthropicBaseURL() + "/messages"

    // Override auth with Copilot token
    result.Header.Set("Authorization", "Bearer "+token)

    // Inject anthropic-beta header for interleaved thinking
    result.Header.Set("anthropic-beta", "interleaved-thinking-2025-05-14")

    // Inject Copilot-specific headers
    result.Header.Set("Openai-Intent", "conversation-panel")
    result.Header.Set("X-Initiator", "user")
    result.Header.Set("Copilot-Vision-Request", "true")
    result.Header.Set("Editor-Version", "vscode/1.99.3")
    result.Header.Set("Editor-Plugin-Version", "copilot/1.0.0")

    return result, nil
}
```

Add variant parsing:

```go
func parseModelVariant(modelID string) (string, *ModelVariant) {
    parts := strings.SplitN(modelID, ":", 2)
    if len(parts) == 1 {
        return modelID, nil
   }
    return parts[0], &ModelVariant{
        ModelID: modelID,
        Type:    "variant",
        Effort:  parts[1],
    }
}

func (t *OutboundTransformer) applyVariant(req *llm.Request, variant *ModelVariant) *llm.Request {
    // Clone request to avoid mutating original
    cloned := *req

    // Look up variant from model info
    info, ok := t.modelInfo[variant.ModelID]
    if !ok {
        return &cloned
    }

    // Generate variants for this model and find the matching one
    variants := GenerateVariants(info)
    for _, v := range variants {
        if v.ModelID == variant.ModelID {
            switch v.Type {
            case "reasoning":
                if cloned.ReasoningEffort == "" {
                    cloned.ReasoningEffort = v.Effort
                }
                if cloned.ReasoningSummary == "" {
                    cloned.ReasoningSummary = "auto"
                }
            case "adaptive":
                if cloned.Thinking == nil {
                    cloned.Thinking = &llm.Thinking{
                        Type:   "adaptive",
                        Effort: v.Effort,
                    }
                }
                if info.IsOpus && v.Effort == "high" {
                    cloned.Thinking.Display = "summarized"
                }
            case "budget":
                if cloned.Thinking == nil {
                    cloned.Thinking = &llm.Thinking{
                        Type:         "enabled",
                        BudgetTokens: v.BudgetTokens,
                    }
                }
            }
            break
        }
    }

    return &cloned
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformRequest_Anthropic" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/outbound.go llm/transformer/openai/copilot/outbound_test.go
git commit -m "feat(copilot): implement Anthropic Messages request transformation"
```

---

### Task 6: Implement Anthropic Messages response and stream transformation

**Files:**
- Modify: `llm/transformer/openai/copilot/outbound.go`
- Test: `llm/transformer/openai/copilot/outbound_test.go`

- [ ] **Step 1: Write failing tests for Anthropic response parsing**

```go
func TestOutboundTransformer_TransformResponse_Anthropic(t *testing.T) {
    transformer := NewOutboundTransformer(Config{
        TokenProvider: &mockTokenProvider{token: "test"},
        BaseURL:       "https://api.githubcopilot.com",
        ModelInfo: map[string]*CopilotModelInfo{
            "claude-sonnet-4-20250514": {SupportsAnthropicMessages: true},
        },
    })

    anthropicResp := map[string]interface{}{
        "type": "message",
        "id":   "msg_123",
        "content": []map[string]interface{}{
            {"type": "text", "text": "Hello from Claude"},
        },
    }
    body, _ := json.Marshal(anthropicResp)
    resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}

    result, err := transformer.TransformResponse(context.Background(), "claude-sonnet-4-20250514", resp)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

func TestOutboundTransformer_IsAnthropicStream(t *testing.T) {
    assert.True(t, isAnthropicSSEEvent("event: message_start"))
    assert.True(t, isAnthropicSSEEvent("event: content_block_start"))
    assert.True(t, isAnthropicSSEEvent("event: content_block_delta"))
    assert.False(t, isAnthropicSSEEvent("event: response.created"))
    assert.False(t, isAnthropicSSEEvent("data: {\"id\":\"chatcmpl-123\"}"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformResponse_Anthropic|TestOutboundTransformer_IsAnthropicStream" -v`
Expected: FAIL

- [ ] **Step 3: Implement Anthropic response and stream detection**

Add stream event detection helper:

```go
func isAnthropicSSEEvent(line string) bool {
    return strings.Contains(line, "message_start") ||
        strings.Contains(line, "message_delta") ||
        strings.Contains(line, "content_block_start") ||
        strings.Contains(line, "content_block_delta") ||
        strings.Contains(line, "content_block_stop")
}
```

Update `TransformResponse` to handle Anthropic format. Add routing before existing logic:

```go
func (t *OutboundTransformer) TransformResponse(ctx context.Context, model string, resp *http.Response) (*llm.Response, error) {
    // Route to Anthropic handler
    if t.usesAnthropicMessages(model) {
        delegate := t.getAnthropicTransformer()
        return delegate.TransformResponse(ctx, model, resp)
    }

    // ... existing Responses API and Chat Completions handling ...
}
```

Update `TransformStream` to detect Anthropic SSE events. The stream needs to peek at the first event type:

```go
func (t *OutboundTransformer) TransformStream(ctx context.Context, model string, reader io.Reader) (<-chan llm.StreamEvent, error) {
    // For Anthropic models, delegate to the Anthropic transformer
    if t.usesAnthropicMessages(model) {
        delegate := t.getAnthropicTransformer()
        return delegate.TransformStream(ctx, model, reader)
    }

    // ... existing Responses API and Chat Completions handling ...
}
```

Note: Since the model name is available at stream time (passed as parameter), we can route directly without peeking at SSE events. This is simpler and more reliable than trying to detect the stream format from the first event.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformResponse_Anthropic|TestOutboundTransformer_IsAnthropicStream" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/outbound.go llm/transformer/openai/copilot/outbound_test.go
git commit -m "feat(copilot): implement Anthropic Messages response and stream handling"
```

---

### Task 7: Omit maxOutputTokens for GPT models

**Files:**
- Modify: `llm/transformer/openai/copilot/outbound.go`
- Test: `llm/transformer/openai/copilot/outbound_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestOutboundTransformer_TransformRequest_GPTNoMaxTokens(t *testing.T) {
    tp := &mockTokenProvider{token: "test-token"}
    transformer := NewOutboundTransformer(Config{
        TokenProvider: tp,
        BaseURL:       "https://api.githubcopilot.com",
        ModelInfo:     map[string]*CopilotModelInfo{},
    })

    req := &llm.Request{
        Model:       "gpt-4.1",
        MaxTokens:   intPtr(4096),
        Messages:    []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent{Text: "Hi"}}},
    }

    result, err := transformer.TransformRequest(context.Background(), req)
    assert.NoError(t, err)

    // Parse the body to verify max_tokens is absent
    var body map[string]interface{}
    json.Unmarshal(result.Body, &body)
    _, hasMaxTokens := body["max_tokens"]
    _, hasMaxOutputTokens := body["max_output_tokens"]
    assert.False(t, hasMaxTokens, "max_tokens should be omitted for GPT models")
    assert.False(t, hasMaxOutputTokens, "max_output_tokens should be omitted for GPT models")
}

func intPtr(v int) *int { return &v }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformRequest_GPTNoMaxTokens" -v`
Expected: FAIL — max_tokens is currently included

- [ ] **Step 3: Implement maxOutputTokens omission for GPT models**

In `TransformRequest`, after building the OpenAI Chat Completions request body and before returning, strip max_tokens for GPT models:

```go
func (t *OutboundTransformer) transformChatCompletionsRequest(ctx context.Context, req *llm.Request, token string) (*llm.HTTPRequest, error) {
    // ... existing transformation logic ...
    result, err := t.openAITransformer.TransformRequest(ctx, req)
    if err != nil {
        return nil, err
    }

    // Omit maxOutputTokens for GPT models (match Copilot CLI behavior)
    if isGPTModel(req.Model) {
        result = stripMaxTokens(result)
    }

    // ... existing header injection ...
    return result, nil
}

func isGPTModel(model string) bool {
    lower := strings.ToLower(model)
    return strings.HasPrefix(lower, "gpt") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4")
}

func stripMaxTokens(result *llm.HTTPRequest) *llm.HTTPRequest {
    var body map[string]interface{}
    if err := json.Unmarshal(result.Body, &body); err != nil {
        return result
    }
    delete(body, "max_tokens")
    delete(body, "max_output_tokens")
    newBody, err := json.Marshal(body)
    if err != nil {
        return result
    }
    result.Body = newBody
    return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd llm && go test ./transformer/openai/copilot/... -run "TestOutboundTransformer_TransformRequest_GPTNoMaxTokens" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add llm/transformer/openai/copilot/outbound.go llm/transformer/openai/copilot/outbound_test.go
git commit -m "feat(copilot): omit maxOutputTokens for GPT models via Copilot"
```

---

### Task 8: Register anthropic/messages as default endpoint for GithubCopilot

**Files:**
- Modify: `internal/server/biz/channel_endpoint.go`

- [ ] **Step 1: Add anthropic/messages endpoint registration**

Find the `defaultEndpoints` map (or equivalent structure) in `channel_endpoint.go` and add the `anthropic/messages` format for `TypeGithubCopilot`:

```go
channel.TypeGithubCopilot: {
    {APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
    {APIFormat: llm.APIFormatAnthropicMessage.String()},
},
```

- [ ] **Step 2: Verify the API format constant exists**

Run: `cd C:/PythonProject/axonhub && grep -r "APIFormatAnthropicMessage" internal/ llm/`
If not found, check the actual constant name used for the Anthropic Messages API format in `llm/`.

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/channel_endpoint.go
git commit -m "feat(copilot): register anthropic/messages as default endpoint"
```

---

### Task 9: Pass CopilotModelInfo from model fetcher to transformer

**Files:**
- Modify: `internal/server/biz/model_fetcher.go`
- Modify: `internal/server/biz/channel_llm.go`

- [ ] **Step 1: Update model fetcher to return CopilotModelInfo**

In `model_fetcher.go`, find the Copilot-specific model fetching code. Update it to call `FetchModelsWithInfo` instead of `FetchModels`, and store the `CopilotModelInfo` map.

The model fetcher likely returns `[]string` (model IDs). Update it to also return the info map, or store the info map in a separate field that the channel can access.

- [ ] **Step 2: Update channel_llm.go to pass ModelInfo to transformer**

Find where `copilot.NewOutboundTransformer` is constructed in `channel_llm.go`. Update the `Config` to include `ModelInfo`:

```go
cfg := copilot.Config{
    TokenProvider: tokenProvider,
    BaseURL:       channel.BaseURL,
    ModelInfo:     modelInfoMap, // from model fetcher
}
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/biz/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/biz/model_fetcher.go internal/server/biz/channel_llm.go
git commit -m "feat(copilot): pass CopilotModelInfo from fetcher to transformer"
```

---

### Task 10: Run full verification suite

**Files:** None — verification only

- [ ] **Step 1: Build both modules**

Run: `go build ./... && cd llm && go build ./...`
Expected: PASS

- [ ] **Step 2: Lint both modules**

Run: `golangci-lint run --timeout 10m --max-same-issues 50 ./... && cd llm && golangci-lint run --timeout 10m --max-same-issues 50 ./...`
Expected: PASS (fix any issues found)

- [ ] **Step 3: Test both modules**

Run: `go test ./... && cd llm && go test ./...`
Expected: PASS

- [ ] **Step 4: Final commit if any fixes were needed**

If lint/test issues required fixes, commit them:

```bash
git add -u
git commit -m "fix: resolve lint/test issues from copilot alignment"
```

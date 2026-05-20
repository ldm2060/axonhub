# Copilot Connection Optimization Design

## Context

AxonHub's current GitHub Copilot connection uses a two-step token exchange flow (OAuth → copilot_internal/v2 token) and fetches models from a static external JSON config. Referencing the [opencode](https://github.com/anomalyco/opencode) project's Copilot implementation, this design simplifies the token flow to use OAuth tokens directly and replaces static model fetching with dynamic API-based fetching with DB caching.

Reference: `packages/opencode/src/plugin/github-copilot/copilot.ts` and `models.ts` from the opencode project.

## Design

### 1. Token Flow Simplification

**Remove the TokenExchanger layer entirely.** The Copilot API accepts OAuth access_tokens directly as Bearer tokens (as demonstrated by opencode's implementation).

**Changes:**

- **Delete** `llm/transformer/openai/copilot/token_exchanger.go` — entire file
- **Delete** `llm/transformer/openai/copilot/token_exchanger_test.go` — tests for deleted file
- **Simplify** `llm/transformer/openai/copilot/token_provider.go`:
  - Remove `TokenExchanger` creation from `NewTokenProvider()`
  - Create `DeviceFlowProvider` without a `tokenExchanger`
  - `GetToken()` now returns OAuth access_token directly via `DeviceFlowProvider.getAccessTokenWithRefresh()`
- **No changes** to `llm/oauth/device_flow_provider.go` — it already has the direct token path when `tokenExchanger` is nil
- **No changes** to `outbound.go` — `tokenProvider.GetToken()` is already an interface call

**Token flow after:**
```
OAuth access_token → refresh if expired → used directly as Bearer
```

### 2. Dynamic Model Fetching

**Fetch models from `{baseURL}/models` API endpoint, cache in DB, fallback to static JSON.**

**Copilot Models API response schema** (from opencode):
```json
{
  "data": [{
    "id": "gpt-4o",
    "name": "GPT-4o",
    "model_picker_enabled": true,
    "version": "gpt-4o-2025-01-01",
    "supported_endpoints": ["/v1/chat/completions"],
    "policy": { "state": "enabled" },
    "capabilities": {
      "family": "gpt-4o",
      "limits": {
        "max_context_window_tokens": 128000,
        "max_output_tokens": 16384,
        "max_prompt_tokens": 128000
      },
      "supports": {
        "vision": true,
        "tool_calls": true,
        "streaming": true,
        "structured_outputs": true,
        "adaptive_thinking": false,
        "reasoning_effort": ["low", "medium", "high"]
      }
    }
  }]
}
```

**Changes:**

- **New file** `llm/transformer/openai/copilot/models.go`:
  - Response type definitions matching the API schema
  - `FetchModels(ctx, baseURL, accessToken string) ([]CopilotModel, error)` function
  - `ParseToModelIdentify(models []CopilotModel) []ModelIdentify` conversion function
  - Filters: `model_picker_enabled == true && policy.state != "disabled"`

- **Modify** `internal/server/biz/model_fetcher.go`:
  - Add `fetchCopilotModelsDynamic(ctx, channel)` method that:
    1. Gets channel's OAuth credentials
    2. Calls `copilot.FetchModels(ctx, baseURL, accessToken)`
    3. Converts response to `[]ModelIdentify`
    4. Caches result (1-hour TTL)
  - Modify `fetchCopilotModels()` to try dynamic first, fallback to static JSON
  - Modify `prepareModelsEndpoint()` to return proper endpoint info for dynamic path

- **Modify** `llm/transformer/openai/copilot/constants.go`:
  - Add `ModelsEndpoint = "/models"` constant

- **Keep** existing static JSON fallback via `ProviderConfURL` as fallback when dynamic fetch fails

### Files Changed Summary

| File | Action | Description |
|------|--------|-------------|
| `llm/transformer/openai/copilot/token_exchanger.go` | DELETE | Remove token exchange layer |
| `llm/transformer/openai/copilot/token_exchanger_test.go` | DELETE | Remove exchange tests |
| `llm/transformer/openai/copilot/token_provider.go` | MODIFY | Remove TokenExchanger, simplify to direct OAuth |
| `llm/transformer/openai/copilot/models.go` | NEW | API response types, FetchModels, conversion |
| `llm/transformer/openai/copilot/constants.go` | MODIFY | Add ModelsEndpoint constant |
| `internal/server/biz/model_fetcher.go` | MODIFY | Add dynamic model fetching with fallback |

### Verification

1. **Token flow**: After changes, create a Copilot channel, verify that:
   - Device flow still works (start + poll)
   - Requests use OAuth access_token directly as Bearer (no v2 token exchange)
   - Token auto-refresh still works when OAuth token expires
   - Existing Copilot API calls succeed with simplified flow

2. **Model fetching**: Verify that:
   - Dynamic model list is fetched from `{baseURL}/models` on channel create/update
   - Models are parsed correctly with capabilities (vision, tool_calls, etc.)
   - Static JSON fallback works when dynamic fetch fails (no credentials, network error)
   - Cache TTL of 1 hour is respected

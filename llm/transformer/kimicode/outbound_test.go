package kimicode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

func TestBuildIdentityHeadersSanitizesValues(t *testing.T) {
	headers, err := BuildIdentityHeaders(Identity{
		UserAgentProduct: CLIUserAgentProduct,
		Version:          CLIVersion + "\n",
		UserAgentSuffix:  "wire\t1.0",
		Hostname:         "host\x00name",
		DeviceModel:      "Windows 10.0.26200 x64",
		OSVersion:        "10.0.26200",
		DeviceID:         "device",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := headers.Get("X-Msh-Platform"), "kimi_code_cli"; got != want {
		t.Fatalf("platform = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Version"), CLIVersion; got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Device-Name"), "hostname"; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Device-Model"), "Windows 10.0.26200 x64"; got != want {
		t.Fatalf("device model = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Os-Version"), "10.0.26200"; got != want {
		t.Fatalf("OS version = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Device-Id"), "device"; got != want {
		t.Fatalf("device ID = %q, want %q", got, want)
	}
	if got, want := headers.Get("User-Agent"), CLIUserAgentProduct+"/"+CLIVersion+" (wire1.0)"; got != want {
		t.Fatalf("user agent = %q, want %q", got, want)
	}
}

func TestFetchModelsRejectsIncompleteMetadata(t *testing.T) {
	if err := ValidateModel(Model{ID: "kimi", ContextLength: 0}); err == nil {
		t.Fatal("expected context_length validation error")
	}
	if err := ValidateModel(Model{ID: "kimi", ContextLength: 1, Protocol: "unknown"}); err == nil {
		t.Fatal("expected protocol validation error")
	}
	if err := ValidateModel(Model{ID: "kimi", ContextLength: 1, Protocol: ProtocolAnthropic}); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}
}

func TestOutboundRoutesByPerRequestProtocol(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, BaseURL: "https://api.kimi.com/coding/v1", Models: []Model{{ID: "kimi", ContextLength: 128}, {ID: "claude", ContextLength: 128, Protocol: ProtocolAnthropic}, {ID: "kimi-responses", ContextLength: 128, Protocol: ProtocolResponse}}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(model string) *llm.Request {
		return &llm.Request{
			Model:           model,
			Messages:        []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
			Stream:          lo.ToPtr(true),
			ReasoningEffort: "high",
			RawRequest: &httpclient.Request{Headers: http.Header{
				"User-Agent":         []string{"ClaudeCLI/1.2.3"},
				"X-Msh-Platform":     []string{"attacker-platform"},
				"X-Msh-Version":      []string{"attacker-version"},
				"X-Msh-Device-Name":  []string{"attacker-host"},
				"X-Msh-Device-Model": []string{"attacker-model"},
				"X-Msh-Os-Version":   []string{"attacker-os"},
				"X-Msh-Device-Id":    []string{"attacker-device"},
			}},
		}
	}
	kimiRequest, err := outbound.TransformRequest(context.Background(), request("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := kimiRequest.URL, "https://api.kimi.com/coding/v1/chat/completions"; got != want {
		t.Fatalf("Kimi URL = %q, want %q", got, want)
	}
	if got := kimiRequest.Metadata[protocolMetadataKey]; got != ProtocolKimi {
		t.Fatalf("Kimi protocol = %q", got)
	}
	if got := kimiRequest.Headers.Get("X-Msh-Device-Id"); got != "device" {
		t.Fatalf("device header = %q", got)
	}
	kimiRequest = httpclient.MergeInboundRequest(kimiRequest, request("kimi").RawRequest)
	assertKimiIdentityHeaders(t, kimiRequest.Headers)
	if kimiRequest.Auth == nil || kimiRequest.Auth.APIKey != "access" {
		t.Fatal("Kimi bearer auth was not configured")
	}
	var kimiBody map[string]any
	if err := json.Unmarshal(kimiRequest.Body, &kimiBody); err != nil {
		t.Fatal(err)
	}
	streamOptions, ok := kimiBody["stream_options"].(map[string]any)
	if !ok || streamOptions["include_usage"] != true {
		t.Fatalf("stream_options.include_usage missing from body: %s", kimiRequest.Body)
	}
	thinking, ok := kimiBody["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" || thinking["effort"] != "high" {
		t.Fatalf("Kimi thinking missing from body: %s", kimiRequest.Body)
	}
	if _, ok := kimiBody["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort should be replaced by thinking: %s", kimiRequest.Body)
	}
	anthropicRequest, err := outbound.TransformRequest(context.Background(), request("claude"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := anthropicRequest.URL, "https://api.kimi.com/coding/v1/messages"; got != want {
		t.Fatalf("Anthropic URL = %q, want %q", got, want)
	}
	if got := anthropicRequest.Query.Get("beta"); got != "true" {
		t.Fatalf("beta query = %q", got)
	}
	if got := anthropicRequest.Metadata[protocolMetadataKey]; got != ProtocolAnthropic {
		t.Fatalf("Anthropic protocol = %q", got)
	}
	anthropicRequest = httpclient.MergeInboundRequest(anthropicRequest, request("claude").RawRequest)
	assertKimiIdentityHeaders(t, anthropicRequest.Headers)
	var body map[string]any
	if err := json.Unmarshal(anthropicRequest.Body, &body); err != nil {
		t.Fatal(err)
	}
	if thinking, ok := body["thinking"].(map[string]any); !ok || thinking["type"] != "enabled" {
		t.Fatalf("enabled thinking missing from body: %s", anthropicRequest.Body)
	}
	if _, ok := body["thinking"].(map[string]any); ok {
		if _, hasBudget := body["thinking"].(map[string]any)["budget_tokens"]; hasBudget {
			t.Fatalf("budget_tokens must not be sent to Kimi: %s", anthropicRequest.Body)
		}
	}
	if outputConfig, ok := body["output_config"].(map[string]any); !ok || outputConfig["effort"] != "high" {
		t.Fatalf("output_config.effort missing from body: %s", anthropicRequest.Body)
	}
	betas, ok := body["betas"].([]any)
	if !ok || len(betas) != 1 || betas[0] != ContextManagementBeta {
		t.Fatalf("betas body field wrong: %#v", body["betas"])
	}
	responsesRequest, err := outbound.TransformRequest(context.Background(), request("kimi-responses"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := responsesRequest.URL, "https://api.kimi.com/coding/v1/responses"; got != want {
		t.Fatalf("Responses URL = %q, want %q", got, want)
	}
	if got := responsesRequest.Metadata[protocolMetadataKey]; got != ProtocolResponse {
		t.Fatalf("Responses protocol = %q", got)
	}
}

func assertKimiIdentityHeaders(t *testing.T, headers http.Header) {
	t.Helper()
	want := map[string]string{
		"User-Agent":         CLIUserAgentProduct + "/" + CLIVersion,
		"X-Msh-Platform":     "kimi_code_cli",
		"X-Msh-Version":      CLIVersion,
		"X-Msh-Device-Name":  "host",
		"X-Msh-Device-Model": "model",
		"X-Msh-Os-Version":   "os",
		"X-Msh-Device-Id":    "device",
	}
	for name, expected := range want {
		if got := headers.Get(name); got != expected {
			t.Fatalf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestKimiStreamingUsageDoesNotMutateOriginalRequest(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, Models: []Model{{ID: "kimi", ContextLength: 1}}})
	if err != nil {
		t.Fatal(err)
	}

	originalOptions := &llm.StreamOptions{IncludeUsage: false}
	original := &llm.Request{
		Model:         "kimi",
		Messages:      []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
		Stream:        lo.ToPtr(true),
		StreamOptions: originalOptions,
	}
	request, err := outbound.TransformRequest(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	streamOptions, ok := body["stream_options"].(map[string]any)
	if !ok || streamOptions["include_usage"] != true {
		t.Fatalf("stream_options.include_usage missing from body: %s", request.Body)
	}
	if original.StreamOptions != originalOptions || original.StreamOptions.IncludeUsage {
		t.Fatal("original stream options were mutated")
	}
}

func TestKimiNonStreamingRequestDoesNotInjectStreamOptions(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, Models: []Model{{ID: "kimi", ContextLength: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "kimi", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}}, Stream: lo.ToPtr(false)})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["stream_options"]; ok {
		t.Fatalf("non-streaming request contains stream_options: %s", request.Body)
	}
}

func TestKimiRequestPreservesExplicitThinkingDisabled(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, Models: []Model{{ID: "claude", ContextLength: 1, Protocol: ProtocolAnthropic}}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "claude", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}}, ReasoningEffort: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(request.Body), `"type":"disabled"`) {
		t.Fatalf("explicit disabled thinking not preserved: %s", request.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if betas, ok := body["betas"].([]any); !ok || len(betas) != 1 || betas[0] != ContextManagementBeta {
		t.Fatalf("disabled thinking should carry the context-management beta: %#v", body["betas"])
	}
	if request.Headers.Get("Anthropic-Beta") != "" {
		t.Fatal("Anthropic-Beta header should not be sent to Kimi")
	}
	if request.Method != http.MethodPost {
		t.Fatalf("method = %s", request.Method)
	}
}

func TestAnthropicRequestWithoutThinkingKeepsDefaultBeta(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, Models: []Model{{ID: "claude", ContextLength: 1, Protocol: ProtocolAnthropic}}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "claude", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}}})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["thinking"]; ok {
		t.Fatalf("thinking should be absent when not requested: %s", request.Body)
	}
	if betas, ok := body["betas"].([]any); !ok || len(betas) != 1 || betas[0] != InterleavedThinkingBeta {
		t.Fatalf("unconfigured thinking should keep the interleaved-thinking beta: %#v", body["betas"])
	}
}

func TestAnthropicNativeBudgetRequestBecomesPlainEnabled(t *testing.T) {
	identity := Identity{UserAgentProduct: CLIUserAgentProduct, Version: CLIVersion, Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, Models: []Model{{ID: "claude", ContextLength: 1, Protocol: ProtocolAnthropic}}})
	if err != nil {
		t.Fatal(err)
	}
	original := &llm.Request{
		Model:           "claude",
		Messages:        []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
		ReasoningBudget: lo.ToPtr(int64(8000)),
	}
	request, err := outbound.TransformRequest(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("budget request should map to plain enabled thinking: %s", request.Body)
	}
	if _, hasBudget := thinking["budget_tokens"]; hasBudget {
		t.Fatalf("budget_tokens must not be sent to Kimi: %s", request.Body)
	}
}

func TestKimiThinkingEffortNormalization(t *testing.T) {
	tests := []struct {
		effort string
		want   map[string]any
	}{
		{effort: "none", want: map[string]any{"type": "disabled"}},
		{effort: "minimal", want: map[string]any{"type": "enabled", "effort": "low"}},
		{effort: "xhigh", want: map[string]any{"type": "enabled", "effort": "max"}},
		{effort: "max", want: map[string]any{"type": "enabled", "effort": "max"}},
		{effort: "high", want: map[string]any{"type": "enabled", "effort": "high"}},
	}
	for _, test := range tests {
		body, err := applyKimiThinking([]byte(`{"reasoning_effort":"high","model":"kimi"}`), test.effort)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		if _, ok := decoded["reasoning_effort"]; ok {
			t.Fatalf("reasoning_effort should be removed for effort %q: %s", test.effort, body)
		}
		got, ok := decoded["thinking"].(map[string]any)
		if !ok {
			t.Fatalf("thinking missing for effort %q: %s", test.effort, body)
		}
		if fmt.Sprint(got) != fmt.Sprint(test.want) {
			t.Fatalf("thinking for effort %q = %v, want %v", test.effort, got, test.want)
		}
	}
	noEffort, err := applyKimiThinking([]byte(`{"model":"kimi"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if string(noEffort) != `{"model":"kimi"}` {
		t.Fatalf("empty effort should leave the body untouched: %s", noEffort)
	}
}

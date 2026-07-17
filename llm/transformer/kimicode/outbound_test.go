package kimicode

import (
	"context"
	"encoding/json"
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
	headers := BuildIdentityHeaders(Identity{
		Version: "v1\n", Hostname: "host\x00name", DeviceModel: "model", OSVersion: "os", DeviceID: "device", UserAgent: "Axon\x01Hub",
	})
	if got, want := headers.Get("X-Msh-Platform"), "kimi_code_cli"; got != want {
		t.Fatalf("platform = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Version"), "v1"; got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
	if got, want := headers.Get("X-Msh-Device-Name"), "hostname"; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if got, want := headers.Get("User-Agent"), "AxonHub"; got != want {
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
	identity := Identity{Version: "test", Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device", UserAgent: "AxonHub/test"}
	provider, err := NewTokenProvider(TokenProviderParams{Credentials: &oauth.OAuthCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, HTTPClient: httpclient.NewHttpClient(), Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := NewOutboundTransformer(Params{TokenProvider: provider, Identity: identity, BaseURL: "https://api.kimi.com/coding/v1", Models: []Model{{ID: "kimi", ContextLength: 128}, {ID: "claude", ContextLength: 128, Protocol: ProtocolAnthropic}}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(model string) *llm.Request {
		return &llm.Request{Model: model, Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}}, Stream: lo.ToPtr(true), ReasoningEffort: "high"}
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
	if kimiRequest.Auth == nil || kimiRequest.Auth.APIKey != "access" {
		t.Fatal("Kimi bearer auth was not configured")
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
	var body map[string]any
	if err := json.Unmarshal(anthropicRequest.Body, &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(anthropicRequest.Body), `"type":"adaptive"`) {
		t.Fatalf("adaptive thinking missing from body: %s", anthropicRequest.Body)
	}
	if betas, ok := body["betas"].([]any); !ok || len(betas) != 1 {
		t.Fatalf("betas body field missing: %#v", body["betas"])
	}
}

func TestKimiRequestPreservesExplicitThinkingDisabled(t *testing.T) {
	identity := Identity{Version: "test", Hostname: "host", DeviceModel: "model", OSVersion: "os", DeviceID: "device", UserAgent: "AxonHub/test"}
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
	if request.Headers.Get("Anthropic-Beta") != "" {
		t.Fatal("Anthropic-Beta header should not be sent to Kimi")
	}
	if request.Method != http.MethodPost {
		t.Fatalf("method = %s", request.Method)
	}
}

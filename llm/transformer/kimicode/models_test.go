package kimicode

import (
	"encoding/json"
	"testing"
)

// TestParseModelsCurrentShape decodes the /models payload shape served by the
// current Kimi Code backend (nested limit/modalities/reasoning, matching the
// official MoonshotAI/kimi-code fixtures).
func TestParseModelsCurrentShape(t *testing.T) {
	payload := `{"data":[
		{
			"id": "kimi-for-coding",
			"limit": {"context": 262144},
			"modalities": {"input": ["text", "image", "video"], "output": ["text"]},
			"reasoning": {"type": "both", "effort": {"valid": ["low", "high", "max"], "default": "max"}},
			"display_name": "Kimi for Coding"
		},
		{
			"id": "kimi-k2.5",
			"limit": {"context": 250000},
			"supports_tool_use": false
		},
		{
			"id": "claude-sonnet-via-kimi",
			"limit": {"context": 200000},
			"protocol": "anthropic"
		},
		{
			"id": "kimi-responses-model",
			"limit": {"context": 200000},
			"protocol": "response"
		}
	]}`
	var decoded ModelsResponse
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 4 {
		t.Fatalf("decoded %d models, want 4", len(decoded.Data))
	}
	models := make([]Model, 0, len(decoded.Data))
	for _, raw := range decoded.Data {
		model, err := toModel(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateModel(model); err != nil {
			t.Fatal(err)
		}
		models = append(models, model)
	}

	first := models[0]
	if first.ContextLength != 262144 {
		t.Fatalf("context length = %d, want 262144", first.ContextLength)
	}
	if first.DisplayName != "Kimi for Coding" {
		t.Fatalf("display name = %q", first.DisplayName)
	}
	if !first.SupportsImageIn || !first.SupportsVideoIn {
		t.Fatalf("modalities not decoded: image=%v video=%v", first.SupportsImageIn, first.SupportsVideoIn)
	}
	if !first.SupportsReasoning || first.SupportsThinkingType != "both" {
		t.Fatalf("reasoning not decoded: %v %q", first.SupportsReasoning, first.SupportsThinkingType)
	}
	if first.ThinkEfforts == nil || first.ThinkEfforts.DefaultEffort != "max" || len(first.ThinkEfforts.ValidEfforts) != 3 {
		t.Fatalf("think efforts not decoded: %+v", first.ThinkEfforts)
	}
	if first.Protocol != "" {
		t.Fatalf("absent protocol should stay empty, got %q", first.Protocol)
	}

	second := models[1]
	if second.SupportsToolUse {
		t.Fatal("supports_tool_use=false should be honored")
	}
	if second.SupportsReasoning || second.SupportsThinkingType != "" {
		t.Fatalf("absent reasoning falls back to the legacy fields: %v %q", second.SupportsReasoning, second.SupportsThinkingType)
	}

	if models[2].Protocol != ProtocolAnthropic {
		t.Fatalf("anthropic protocol = %q", models[2].Protocol)
	}
	if models[3].Protocol != ProtocolResponse {
		t.Fatalf("response protocol = %q", models[3].Protocol)
	}
}

// TestParseModelsLegacyShape keeps the flat legacy fields working for older
// servers and stored credentials.
func TestParseModelsLegacyShape(t *testing.T) {
	payload := `{"models":[{
		"id": "kimi",
		"context_length": 131072,
		"supports_reasoning": true,
		"supports_image_in": true,
		"supports_tool_use": true,
		"supports_thinking_type": "only",
		"think_efforts": {"support": true, "valid_efforts": ["high"], "default_effort": "low"}
	}]}`
	var decoded ModelsResponse
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	model, err := toModel(decoded.Models[0])
	if err != nil {
		t.Fatal(err)
	}
	if model.ContextLength != 131072 || !model.SupportsReasoning || !model.SupportsImageIn {
		t.Fatalf("legacy fields not decoded: %+v", model)
	}
	if model.SupportsThinkingType != "only" {
		t.Fatalf("legacy thinking type = %q", model.SupportsThinkingType)
	}
	if model.ThinkEfforts == nil || !model.ThinkEfforts.Support || model.ThinkEfforts.DefaultEffort != "low" {
		t.Fatalf("legacy think efforts not decoded: %+v", model.ThinkEfforts)
	}
}

// TestParseModelsReasoningDefaultDropped matches the official CLI: an effort
// default that is not listed in valid is dropped.
func TestParseModelsReasoningDefaultDropped(t *testing.T) {
	payload := `{"data":[{"id":"kimi","limit":{"context":1000},"reasoning":{"type":"both","effort":{"valid":["low"],"default":"max"}}}]}`
	var decoded ModelsResponse
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	model, err := toModel(decoded.Data[0])
	if err != nil {
		t.Fatal(err)
	}
	if model.ThinkEfforts == nil || model.ThinkEfforts.DefaultEffort != "" || len(model.ThinkEfforts.ValidEfforts) != 1 {
		t.Fatalf("invalid default should be dropped: %+v", model.ThinkEfforts)
	}
}

func TestNormalizeModelProtocolUnknownFallsBackToKimi(t *testing.T) {
	if got := normalizeModelProtocol("openai"); got != "" {
		t.Fatalf("unknown protocol = %q, want empty", got)
	}
	if got := normalizeModelProtocol(" response "); got != ProtocolResponse {
		t.Fatalf("response protocol = %q", got)
	}
}

func TestValidateModelRejectsUnknownProtocol(t *testing.T) {
	if err := ValidateModel(Model{ID: "kimi", ContextLength: 1, Protocol: ProtocolResponse}); err != nil {
		t.Fatalf("response protocol should be accepted: %v", err)
	}
	if err := ValidateModel(Model{ID: "kimi", ContextLength: 1, Protocol: "unknown"}); err == nil {
		t.Fatal("expected protocol validation error")
	}
}

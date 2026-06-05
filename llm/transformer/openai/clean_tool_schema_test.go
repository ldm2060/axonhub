package openai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
)

func TestCleanToolSchema_RemoveFormatURIRoot(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","format":"uri"}}}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{"type":"object","properties":{"url":{"type":"string"}}}`, string(result))
}

func TestCleanToolSchema_RemoveFormatURINested(t *testing.T) {
	input := json.RawMessage(`{
		"type":"object",
		"properties":{
			"nested":{
				"type":"object",
				"properties":{
					"link":{"type":"string","format":"uri"}
				}
			}
		}
	}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{
		"type":"object",
		"properties":{
			"nested":{
				"type":"object",
				"properties":{
					"link":{"type":"string"}
				}
			}
		}
	}`, string(result))
}

func TestCleanToolSchema_RemoveFormatURIArrayItems(t *testing.T) {
	input := json.RawMessage(`{
		"type":"object",
		"properties":{
			"urls":{
				"type":"array",
				"items":{"type":"string","format":"uri"}
			}
		}
	}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{
		"type":"object",
		"properties":{
			"urls":{
				"type":"array",
				"items":{"type":"string"}
			}
		}
	}`, string(result))
}

func TestCleanToolSchema_PreserveFormatDateTime(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"date":{"type":"string","format":"date-time"}}}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{"type":"object","properties":{"date":{"type":"string","format":"date-time"}}}`, string(result))
}

func TestCleanToolSchema_PreserveOtherFormats(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"email":{"type":"string","format":"email"}}}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{"type":"object","properties":{"email":{"type":"string","format":"email"}}}`, string(result))
}

func TestCleanToolSchema_InvalidJSONUnchanged(t *testing.T) {
	input := json.RawMessage(`{not valid json}`)
	result := cleanToolSchema(input)
	require.Equal(t, string(input), string(result))
}

func TestCleanToolSchema_EmptyUnchanged(t *testing.T) {
	input := json.RawMessage(nil)
	result := cleanToolSchema(input)
	require.Equal(t, string(input), string(result))

	empty := json.RawMessage(``)
	result = cleanToolSchema(empty)
	require.Equal(t, string(empty), string(result))
}

func TestCleanToolSchema_MultipleURIsAndOtherFormats(t *testing.T) {
	input := json.RawMessage(`{
		"type":"object",
		"properties":{
			"homepage":{"type":"string","format":"uri"},
			"created":{"type":"string","format":"date-time"},
			"api_url":{"type":"string","format":"uri"},
			"name":{"type":"string"}
		}
	}`)
	result := cleanToolSchema(input)
	require.JSONEq(t, `{
		"type":"object",
		"properties":{
			"homepage":{"type":"string"},
			"created":{"type":"string","format":"date-time"},
			"api_url":{"type":"string"},
			"name":{"type":"string"}
		}
	}`, string(result))
}

func TestToolFromLLM_CleansURIFromParameters(t *testing.T) {
	tool := llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name:        "browse",
			Description: "Browse a URL",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","format":"uri"}}}`),
		},
	}

	result := ToolFromLLM(tool)
	require.Equal(t, llm.ToolTypeFunction, result.Type)
	require.Equal(t, "browse", result.Function.Name)
	require.JSONEq(t, `{"type":"object","properties":{"url":{"type":"string"}}}`, string(result.Function.Parameters))
}

func TestToolFromLLM_PreservesParametersWithoutURI(t *testing.T) {
	tool := llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name:        "search",
			Description: "Search for items",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		},
	}

	result := ToolFromLLM(tool)
	require.JSONEq(t, `{"type":"object","properties":{"query":{"type":"string"}}}`, string(result.Function.Parameters))
}

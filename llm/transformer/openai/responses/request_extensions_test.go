package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
)

func strPtr(v string) *string { return &v }

func newExtensionsRequest(ext *llm.OpenAIResponsesRequestExtensions) *llm.Request {
	if ext == nil {
		return nil
	}

	return &llm.Request{
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{Request: ext},
		},
	}
}

// marshalRequestPayloadMapPath is the map round-trip that the fast path skips.
// It is kept here as a test oracle: it always re-parses and re-serializes the
// body, so it sorts top-level keys alphabetically.
func marshalRequestPayloadMapPath(
	t *testing.T,
	payload Request,
	ext *llm.OpenAIResponsesRequestExtensions,
) []byte {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	if ext == nil {
		return body
	}

	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &obj))

	mergeRawRequestFields(obj, ext)

	if tools, ok := mergeRawOnlyTools(obj["tools"], ext); ok {
		raw, err := json.Marshal(tools)
		require.NoError(t, err)
		obj["tools"] = raw
	}

	if len(ext.RawToolChoice) > 0 && rawToolChoiceMatchesCurrentTools(ext.RawToolChoice, payload.ToolChoice) {
		obj["tool_choice"] = cloneRaw(ext.RawToolChoice)
	}

	if input, ok := mergeRawOnlyInputItems(obj["input"], ext); ok {
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		obj["input"] = raw
	}

	out, err := json.Marshal(obj)
	require.NoError(t, err)

	return out
}

// TestMarshalRequestPayload_SemanticallyMatchesMapPath pins that skipping the
// map round-trip for extensions with no raw payload never changes the request
// semantics. Top-level key ORDER does differ (struct field order instead of
// alphabetical) because json.Marshal(map) sorts keys; that is intentional and
// matches what every request without extensions already produces.
func TestMarshalRequestPayload_SemanticallyMatchesMapPath(t *testing.T) {
	text := "hello"

	payload := Request{
		Model:        "gpt-5-codex",
		Instructions: "be brief",
		Input: Input{Items: []Item{
			{Type: "message", Role: "user", Content: &Input{Text: &text}},
			{Type: "function_call", CallID: "c1", Name: "f", Arguments: `{"a":1}`},
			{Type: "reasoning", ID: "r1"},
		}},
		Tools: []Tool{{Type: "function", Name: "f"}},
	}

	cases := map[string]*llm.OpenAIResponsesRequestExtensions{
		"nil extension":     nil,
		"empty extension":   {},
		"reasoning only":    {ReasoningContext: "ctx"},
		"empty raw fields":  {RawFields: map[string]json.RawMessage{}},
		"empty raw items":   {RawInputItems: []llm.OpenAIResponsesRawFragment{}},
		"empty raw tools":   {RawTools: []llm.OpenAIResponsesRawFragment{}},
		"empty tool choice": {RawToolChoice: json.RawMessage{}},
		"all empty together": {
			RawFields:     map[string]json.RawMessage{},
			RawInputItems: []llm.OpenAIResponsesRawFragment{},
			RawTools:      []llm.OpenAIResponsesRawFragment{},
			RawToolChoice: json.RawMessage{},
		},
	}

	for name, ext := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := marshalRequestPayload(payload, newExtensionsRequest(ext))
			require.NoError(t, err)

			want := marshalRequestPayloadMapPath(t, payload, ext)

			var gotObj, wantObj map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(got, &gotObj))
			require.NoError(t, json.Unmarshal(want, &wantObj))

			assert.Equal(t, wantObj, gotObj, "request semantics must be unchanged")
		})
	}
}

// TestMarshalRequestPayload_StillMergesRawPayload guards the other half: when
// the extensions DO carry raw payload, the merge must still happen.
func TestMarshalRequestPayload_StillMergesRawPayload(t *testing.T) {
	text := "hello"

	payload := Request{
		Model: "gpt-5-codex",
		Input: Input{Items: []Item{
			{Type: "message", Role: "user", Content: &Input{Text: &text}},
		}},
	}

	t.Run("raw fields are spliced in", func(t *testing.T) {
		ext := &llm.OpenAIResponsesRequestExtensions{
			RawFields: map[string]json.RawMessage{
				"prompt": json.RawMessage(`{"cache_key":"abc"}`),
			},
		}

		got, err := marshalRequestPayload(payload, newExtensionsRequest(ext))
		require.NoError(t, err)

		var obj map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(got, &obj))

		assert.JSONEq(t, `{"cache_key":"abc"}`, string(obj["prompt"]))
	})

	t.Run("raw-only input items replace structured ones", func(t *testing.T) {
		ext := &llm.OpenAIResponsesRequestExtensions{
			RawInputItems: []llm.OpenAIResponsesRawFragment{{
				Type:          "web_search_call",
				OriginalIndex: 0,
				Raw:           json.RawMessage(`{"type":"web_search_call","id":"w1"}`),
			}},
		}

		got, err := marshalRequestPayload(payload, newExtensionsRequest(ext))
		require.NoError(t, err)

		var obj struct {
			Input []map[string]any `json:"input"`
		}
		require.NoError(t, json.Unmarshal(got, &obj))

		// The raw fragment is spliced in at its original index, so the merged
		// array holds the raw item plus every structured item.
		require.Len(t, obj.Input, 2)
		assert.Equal(t, "web_search_call", obj.Input[0]["type"])
		assert.Equal(t, "w1", obj.Input[0]["id"])
		assert.Equal(t, "message", obj.Input[1]["type"])
	})
}

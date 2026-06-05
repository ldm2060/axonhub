package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/streams"
)

// collectStreamEventsWithTransformer is a helper to collect all events from a
// transformed stream using a caller-provided transformer.
func collectStreamEventsWithTransformer(t *testing.T, transformer *InboundTransformer, input []*llm.Response) []StreamEvent {
	t.Helper()

	stream, err := transformer.TransformStream(t.Context(), streams.SliceStream(input))
	require.NoError(t, err)

	var events []StreamEvent
	for stream.Next() {
		raw := stream.Current()
		var event StreamEvent
		require.NoError(t, json.Unmarshal(raw.Data, &event))
		events = append(events, event)
	}
	require.NoError(t, stream.Err())

	return events
}

// TestStreamDelayedToolStart_ArgsBeforeIdName tests that when tool call arguments
// arrive before the id and name in a stream, they are buffered and only emitted
// after content_block_start.
func TestStreamDelayedToolStart_ArgsBeforeIdName(t *testing.T) {
	transformer := NewInboundTransformer()
	toolCalls := lo.ToPtr("tool_calls")

	// Simulate a stream where arguments arrive before id/name
	input := []*llm.Response{
		{
			ID:    "msg_delayed_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"ci`,
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_delayed_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_abc123",
							Function: llm.FunctionCall{
								Name: "get_weather",
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_delayed_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `ty":"SF"}`,
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_delayed_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: toolCalls,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectStreamEventsWithTransformer(t, transformer, input)

	// Find content_block_start event for the tool
	var toolStartEvent *StreamEvent
	var toolStartIndex int
	for i, event := range events {
		if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			toolStartEvent = &events[i]
			toolStartIndex = i
			break
		}
	}

	require.NotNil(t, toolStartEvent, "should have a content_block_start for tool_use")
	require.Equal(t, "call_abc123", toolStartEvent.ContentBlock.ID)
	require.NotNil(t, toolStartEvent.ContentBlock.Name)
	require.Equal(t, "get_weather", *toolStartEvent.ContentBlock.Name)

	// The buffered args should be emitted after content_block_start
	// Find the first input_json_delta after the content_block_start
	var foundBufferedArgs bool
	for i := toolStartIndex + 1; i < len(events); i++ {
		if events[i].Type == "content_block_delta" && events[i].Delta != nil && events[i].Delta.Type != nil && *events[i].Delta.Type == "input_json_delta" {
			foundBufferedArgs = true
			require.NotNil(t, events[i].Delta.PartialJSON)
			require.Contains(t, *events[i].Delta.PartialJSON, `{"ci`)
			break
		}
	}
	require.True(t, foundBufferedArgs, "buffered arguments should be emitted after content_block_start")
}

// TestStreamDelayedToolStart_FallbackIdName tests that when id/name are never
// provided in the stream, fallback values tool_call_{idx} and unknown_tool
// are used at finish.
func TestStreamDelayedToolStart_FallbackIdName(t *testing.T) {
	transformer := NewInboundTransformer()
	toolCalls := lo.ToPtr("tool_calls")

	// Simulate a stream where only arguments arrive, no id or name
	input := []*llm.Response{
		{
			ID:    "msg_fallback_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"key":"value"}`,
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_fallback_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: toolCalls,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectStreamEventsWithTransformer(t, transformer, input)

	// Find content_block_start event for the tool
	var toolStartEvent *StreamEvent
	for i, event := range events {
		if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			toolStartEvent = &events[i]
			break
		}
	}

	require.NotNil(t, toolStartEvent, "should have a content_block_start for tool_use")
	// Fallback ID should be tool_call_0
	require.Equal(t, "tool_call_0", toolStartEvent.ContentBlock.ID)
	// Fallback name should be unknown_tool
	require.NotNil(t, toolStartEvent.ContentBlock.Name)
	require.Equal(t, "unknown_tool", *toolStartEvent.ContentBlock.Name)
}

// TestStreamDelayedToolStart_ArgsBufferedAndEmittedAfterStart verifies that
// arguments received before id/name are fully buffered and then emitted
// as input_json_delta after the content_block_start event.
func TestStreamDelayedToolStart_ArgsBufferedAndEmittedAfterStart(t *testing.T) {
	transformer := NewInboundTransformer()
	toolCalls := lo.ToPtr("tool_calls")

	input := []*llm.Response{
		// First chunk: args only (no id/name)
		{
			ID:    "msg_buffer_test",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"lo`,
							},
						},
					},
				},
			}},
		},
		// Second chunk: more args (still no id/name)
		{
			ID:    "msg_buffer_test",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `cati`,
							},
						},
					},
				},
			}},
		},
		// Third chunk: id and name arrive
		{
			ID:    "msg_buffer_test",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_def456",
							Function: llm.FunctionCall{
								Name: "locate",
							},
						},
					},
				},
			}},
		},
		// Fourth chunk: more args after start
		{
			ID:    "msg_buffer_test",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `on":"NYC"}`,
							},
						},
					},
				},
			}},
		},
		// Finish
		{
			ID:    "msg_buffer_test",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: toolCalls,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectStreamEventsWithTransformer(t, transformer, input)

	// Collect all input_json_delta events
	var jsonDeltas []string
	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "input_json_delta" {
			require.NotNil(t, event.Delta.PartialJSON)
			jsonDeltas = append(jsonDeltas, *event.Delta.PartialJSON)
		}
	}

	require.NotEmpty(t, jsonDeltas, "should have input_json_delta events")

	// The concatenated deltas should reconstruct the full arguments
	var fullArgs strings.Builder
	for _, d := range jsonDeltas {
		fullArgs.WriteString(d)
	}
	require.Equal(t, `{"location":"NYC"}`, fullArgs.String())
}

// TestStreamDelayedToolStart_IdNameArriveTogetherThenArgs tests the normal
// case where id and name arrive together, then arguments follow.
func TestStreamDelayedToolStart_IdNameArriveTogetherThenArgs(t *testing.T) {
	transformer := NewInboundTransformer()
	toolCalls := lo.ToPtr("tool_calls")

	input := []*llm.Response{
		{
			ID:    "msg_normal_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_normal",
							Function: llm.FunctionCall{
								Name: "search",
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_normal_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"q":"test"}`,
							},
						},
					},
				},
			}},
		},
		{
			ID:    "msg_normal_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: toolCalls,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectStreamEventsWithTransformer(t, transformer, input)

	// Find content_block_start for tool
	var toolStartEvent *StreamEvent
	for i, event := range events {
		if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			toolStartEvent = &events[i]
			break
		}
	}

	require.NotNil(t, toolStartEvent)
	require.Equal(t, "call_normal", toolStartEvent.ContentBlock.ID)
	require.Equal(t, "search", *toolStartEvent.ContentBlock.Name)
}

// TestStreamDelayedToolStart_FallbackAtFinishForEmptyToolCall verifies that
// when an empty tool call (no id, no name, no args) arrives in the stream,
// it is not emitted (the forceStartPendingToolCalls skips it).
func TestStreamDelayedToolStart_EmptyToolCallSkipped(t *testing.T) {
	transformer := NewInboundTransformer()
	stop := lo.ToPtr("stop")

	// A tool call with no id, name, or args should be skipped
	input := []*llm.Response{
		{
			ID:    "msg_empty_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							// No id, name, or arguments
						},
					},
				},
			}},
		},
		{
			ID:    "msg_empty_tool",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: stop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectStreamEventsWithTransformer(t, transformer, input)

	// No tool_use content_block_start should be emitted
	for _, event := range events {
		if event.Type == "content_block_start" && event.ContentBlock != nil {
			require.NotEqual(t, "tool_use", event.ContentBlock.Type, "empty tool call should not emit a tool_use block")
		}
	}
}

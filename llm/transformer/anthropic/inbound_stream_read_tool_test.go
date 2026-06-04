package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
)

// TestInboundStream_ReadToolSanitizesEmptyPages verifies that when a Read tool
// call streams input with `"pages": ""`, the field is removed before being
// emitted to the client. The streaming path must buffer Read tool arguments
// and emit sanitized input when the content block closes.
func TestInboundStream_ReadToolSanitizesEmptyPages(t *testing.T) {
	transformer := NewInboundTransformer()

	toolCallStop := "tool_calls"
	input := []*llm.Response{
		{
			ID:     "msg_read_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_read_001",
							Type:  "function",
							Function: llm.FunctionCall{
								Name:      "Read",
								Arguments: "",
							},
						},
					},
				},
			}},
		},
		// First argument chunk.
		{
			ID:     "msg_read_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"file_pa`,
							},
						},
					},
				},
			}},
		},
		// Second argument chunk — includes pages: "" which must be stripped.
		{
			ID:     "msg_read_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `th": "/tmp/test.txt", "pages": ""}`,
							},
						},
					},
				},
			}},
		},
		// Finish reason.
		{
			ID:     "msg_read_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &toolCallStop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	// Collect all input_json_delta events for the tool_use content block.
	var inputJSONDeltas []string

	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "input_json_delta" {
			inputJSONDeltas = append(inputJSONDeltas, *event.Delta.PartialJSON)
		}
	}

	// The combined input should have "pages" removed.
	combined := strings.Join(inputJSONDeltas, "")

	var parsed map[string]any

	err := json.Unmarshal([]byte(combined), &parsed)
	require.NoError(t, err)

	// pages field should be absent (sanitized away).
	_, hasPages := parsed["pages"]
	require.False(t, hasPages, "pages field should be removed from Read tool input")

	// file_path should still be present.
	require.Equal(t, "/tmp/test.txt", parsed["file_path"])
}

// TestInboundStream_ReadToolBuffersArguments verifies that Read tool arguments
// are NOT streamed incrementally but instead emitted as a single sanitized
// delta when the content block closes. This confirms the buffering behavior.
func TestInboundStream_ReadToolBuffersArguments(t *testing.T) {
	transformer := NewInboundTransformer()

	toolCallStop := "tool_calls"
	input := []*llm.Response{
		{
			ID:     "msg_read_buffer",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_read_buf",
							Type:  "function",
							Function: llm.FunctionCall{
								Name:      "Read",
								Arguments: "",
							},
						},
					},
				},
			}},
		},
		// Chunk 1 — should be buffered, NOT streamed.
		{
			ID:     "msg_read_buffer",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"file_`,
							},
						},
					},
				},
			}},
		},
		// Chunk 2 — should also be buffered.
		{
			ID:     "msg_read_buffer",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `path":"/etc/hosts"}`,
							},
						},
					},
				},
			}},
		},
		// Finish — triggers flush of buffered args.
		{
			ID:     "msg_read_buffer",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &toolCallStop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	// Count input_json_delta events: Read tool should emit exactly 1 (the
	// complete sanitized JSON at block-close time), not 2 incremental deltas.
	var inputJSONDeltaCount int

	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "input_json_delta" {
			inputJSONDeltaCount++
		}
	}

	require.Equal(t, 1, inputJSONDeltaCount, "Read tool arguments should be buffered and emitted as a single delta")
}

// TestInboundStream_NonReadToolNotBuffered verifies that non-Read tool calls
// still stream their arguments incrementally (no buffering/sanitization).
func TestInboundStream_NonReadToolNotBuffered(t *testing.T) {
	transformer := NewInboundTransformer()

	toolCallStop := "tool_calls"
	input := []*llm.Response{
		{
			ID:     "msg_bash_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_bash_001",
							Type:  "function",
							Function: llm.FunctionCall{
								Name:      "Bash",
								Arguments: `{"com`,
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_bash_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `mand":"ls"}`,
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_bash_tool_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &toolCallStop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	// Non-Read tools should stream input_json_delta events directly (not buffered).
	var inputJSONDeltaCount int

	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "input_json_delta" {
			inputJSONDeltaCount++
		}
	}

	// Should have 2 separate input_json_delta events (streamed directly, not buffered).
	require.Equal(t, 2, inputJSONDeltaCount, "non-Read tool arguments should stream directly")
}

// TestInboundStream_ReadToolCaseInsensitive verifies that "read" (lowercase)
// is also recognized as the Read tool for sanitization purposes.
func TestInboundStream_ReadToolCaseInsensitive(t *testing.T) {
	transformer := NewInboundTransformer()

	toolCallStop := "tool_calls"
	input := []*llm.Response{
		{
			ID:     "msg_read_lower",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							ID:    "call_read_lower",
							Type:  "function",
							Function: llm.FunctionCall{
								Name:      "read",
								Arguments: "",
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_read_lower",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Index: 0,
							Function: llm.FunctionCall{
								Arguments: `{"file_path":"/tmp/x","pages":""}`,
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_read_lower",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &toolCallStop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	var inputJSONDeltas []string

	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "input_json_delta" {
			inputJSONDeltas = append(inputJSONDeltas, *event.Delta.PartialJSON)
		}
	}

	combined := strings.Join(inputJSONDeltas, "")

	var parsed map[string]any

	err := json.Unmarshal([]byte(combined), &parsed)
	require.NoError(t, err)

	_, hasPages := parsed["pages"]
	require.False(t, hasPages, "pages field should be removed even when tool name is lowercase 'read'")
	require.Equal(t, "/tmp/x", parsed["file_path"])
}

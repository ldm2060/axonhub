package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/streams"
)

// TestStreamPipeline_RefusalContentFilter feeds llm.Response chunks carrying a
// Refusal delta followed by finish_reason=content_filter into the stream
// transformer and asserts that the output contains:
//  1. a text_delta event with the refusal text, and
//  2. a message_delta event with stop_reason=end_turn.
//
// This is a regression test: the existing inbound_refusal_test.go tests only
// exercise the non-stream convertToAnthropicResponse path, not the actual
// TransformStream pipeline.
func TestStreamPipeline_RefusalContentFilter(t *testing.T) {
	transformer := NewInboundTransformer()

	refusalText := "I cannot assist with that request."
	contentFilter := lo.ToPtr("content_filter")

	input := []*llm.Response{
		// Chunk 1: refusal delta
		{
			ID:     "msg_refusal_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role:    "assistant",
					Refusal: refusalText,
				},
			}},
		},
		// Chunk 2: finish reason = content_filter + usage
		{
			ID:     "msg_refusal_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: contentFilter,
			}},
			Usage: &llm.Usage{},
		},
	}

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

	// Assert: at least one text_delta event contains the refusal text.
	var foundRefusalTextDelta bool
	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil &&
			event.Delta.Type != nil && *event.Delta.Type == "text_delta" {
			if event.Delta.Text != nil && *event.Delta.Text == refusalText {
				foundRefusalTextDelta = true
			}
		}
	}
	require.True(t, foundRefusalTextDelta, "stream should emit a text_delta with the refusal text")

	// Assert: message_delta has stop_reason=end_turn for content_filter.
	var foundContentFilterEndTurn bool
	for _, event := range events {
		if event.Type == "message_delta" && event.Delta != nil &&
			event.Delta.StopReason != nil && *event.Delta.StopReason == "end_turn" {
			foundContentFilterEndTurn = true
		}
	}
	require.True(t, foundContentFilterEndTurn,
		"stream should emit message_delta with stop_reason=end_turn for content_filter finish reason")
}

// TestStreamPipeline_ContentFilterWithNormalText verifies that when a stream
// has regular text content followed by finish_reason=content_filter, the
// message_delta stop_reason is still end_turn.
func TestStreamPipeline_ContentFilterWithNormalText(t *testing.T) {
	transformer := NewInboundTransformer()

	text := "Here is some content that was filtered."
	contentFilter := lo.ToPtr("content_filter")

	input := []*llm.Response{
		// Chunk 1: text delta
		{
			ID:     "msg_cf_text_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
				},
			}},
		},
		// Chunk 2: finish reason = content_filter + usage
		{
			ID:     "msg_cf_text_stream",
			Object: "chat.completion.chunk",
			Model:  "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: contentFilter,
			}},
			Usage: &llm.Usage{},
		},
	}

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

	// Assert: text_delta with the content.
	var foundTextDelta bool
	for _, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil &&
			event.Delta.Type != nil && *event.Delta.Type == "text_delta" {
			if event.Delta.Text != nil && *event.Delta.Text == text {
				foundTextDelta = true
			}
		}
	}
	require.True(t, foundTextDelta, "stream should emit text_delta with the content text")

	// Assert: message_delta stop_reason = end_turn for content_filter.
	var foundEndTurn bool
	for _, event := range events {
		if event.Type == "message_delta" && event.Delta != nil &&
			event.Delta.StopReason != nil && *event.Delta.StopReason == "end_turn" {
			foundEndTurn = true
		}
	}
	require.True(t, foundEndTurn,
		"content_filter finish_reason should map to end_turn stop_reason in message_delta")
}

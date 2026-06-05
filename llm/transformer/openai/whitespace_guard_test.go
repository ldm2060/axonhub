package openai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolCallWhitespaceGuard_EmptyArgsAllowed(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	require.True(t, g.allow(0, ""))
}

func TestToolCallWhitespaceGuard_NilReceiverAllows(t *testing.T) {
	var g *toolCallWhitespaceGuard
	require.True(t, g.allow(0, ""))
}

func TestToolCallWhitespaceGuard_BelowThreshold(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// 499 spaces: just below the 500 threshold
	args := strings.Repeat(" ", infiniteWhitespaceThreshold-1)
	require.True(t, g.allow(0, args))
	// One more space char should push to threshold and abort
	require.False(t, g.allow(0, " "))
}

func TestToolCallWhitespaceGuard_ExactlyAtThreshold(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// Feed 500 spaces in one call
	args := strings.Repeat(" ", infiniteWhitespaceThreshold)
	require.False(t, g.allow(0, args))
}

func TestToolCallWhitespaceGuard_AboveThreshold(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	args := strings.Repeat(" ", infiniteWhitespaceThreshold+100)
	require.False(t, g.allow(0, args))
}

func TestToolCallWhitespaceGuard_ResetOnNonWhitespace(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// Build up whitespace to just below threshold
	nearThreshold := strings.Repeat(" ", infiniteWhitespaceThreshold-1)
	require.True(t, g.allow(0, nearThreshold))
	// Non-whitespace resets the counter
	require.True(t, g.allow(0, "{"))
	// Now whitespace counter should be back to 0, so we can accumulate again
	require.True(t, g.allow(0, nearThreshold))
}

func TestToolCallWhitespaceGuard_AbortedIndexStaysAborted(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// Trigger abort
	args := strings.Repeat(" ", infiniteWhitespaceThreshold)
	require.False(t, g.allow(0, args))
	// Even non-whitespace args should be rejected after abort
	require.False(t, g.allow(0, `{"key":"value"}`))
	// Even whitespace args should be rejected after abort
	require.False(t, g.allow(0, "  "))
	// Even empty string is rejected after abort (aborted check happens first)
	require.False(t, g.allow(0, ""))
}

func TestToolCallWhitespaceGuard_IndependentIndices(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// Abort index 0
	args := strings.Repeat(" ", infiniteWhitespaceThreshold)
	require.False(t, g.allow(0, args))
	// Index 1 should still be fine
	require.True(t, g.allow(1, `{"key":"value"}`))
	// Index 1 should also survive whitespace that would abort it independently
	nearThreshold := strings.Repeat(" ", infiniteWhitespaceThreshold-1)
	require.True(t, g.allow(1, nearThreshold))
	// But index 1 can be independently aborted
	require.False(t, g.allow(1, " "))
	// Index 0 remains aborted
	require.False(t, g.allow(0, "x"))
}

func TestToolCallWhitespaceGuard_AllWhitespaceTypes(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// All whitespace char types: space, tab, newline, carriage return, vertical tab, form feed
	allWS := " \t\n\r\v\f"
	repeats := infiniteWhitespaceThreshold/len(allWS) + 1
	longWS := strings.Repeat(allWS, repeats)
	require.False(t, g.allow(0, longWS))
}

func TestToolCallWhitespaceGuard_MixedContentNotAborted(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// JSON-like content with whitespace between tokens should not trigger
	for range 1000 {
		require.True(t, g.allow(0, ` {"key": "value"} `))
	}
}

func TestToolCallWhitespaceGuard_ThresholdBoundaryIncremental(t *testing.T) {
	g := newToolCallWhitespaceGuard()
	// Feed spaces one at a time
	for i := range infiniteWhitespaceThreshold - 1 {
		require.True(t, g.allow(0, " "), "should allow at count %d", i+1)
	}
	// The 500th space should trigger abort
	require.False(t, g.allow(0, " "))
}

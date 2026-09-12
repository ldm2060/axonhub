package zcode

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func bypassedSigningRoundTripper(inner http.RoundTripper) *SigningRoundTripper {
	rt := NewSigningRoundTripper(inner, &RequestSigner{}, nil)
	rt.signer.setBypass()

	return rt
}

// A 200 streaming response must be relayed in full: the verify-inspection
// path only ever applies to 401, and buffering a stream larger than the
// inspection limit used to drop everything past it — silently losing the
// terminal message_delta/message_stop events that carry the usage.
func TestSigningRoundTripperRelaysStreamingBodyLargerThanLimit(t *testing.T) {
	var sb strings.Builder

	for range 700 {
		sb.WriteString(`event: content_block_delta` + "\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"abc"}}` + "\n\n")
	}

	sb.WriteString(`event: message_delta` + "\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":123,"output_tokens":456}}` + "\n\n")
	sb.WriteString(`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n")

	sse := sb.String()
	require.Greater(t, len(sse), 1<<16, "test payload must exceed the inspection limit")

	inner := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(sse)),
		}, nil
	})

	req, err := http.NewRequest(http.MethodPost, "https://open.bigmodel.cn/api/anthropic/v1/messages", nil)
	require.NoError(t, err)

	resp, err := bypassedSigningRoundTripper(inner).RoundTrip(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Len(t, got, len(sse), "stream must not be truncated at the inspection limit")
	require.Contains(t, string(got), `"input_tokens":123`)
	require.Contains(t, string(got), "message_stop")
}

// Verify rejections only ever arrive on 401; every other response body must
// pass through readBodyForRetry untouched.
func TestReadBodyForRetryLeavesNon401Untouched(t *testing.T) {
	original := "event: message_start\ndata: {}\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(original)),
	}

	require.Nil(t, readBodyForRetry(resp))

	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, original, string(rest), "200 body must not be consumed or replaced")
}

// A 401 body is buffered for inspection and restored ahead of the remaining
// stream, so bodies larger than the inspection limit stay intact for the
// caller.
func TestReadBodyForRetryRestores401Body(t *testing.T) {
	original := strings.Repeat("x", 60000) +
		`{"error":{"code":"VERIFY_SIGNATURE_INVALID"}}` + strings.Repeat("y", 10000)

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(original)),
	}

	got := readBodyForRetry(resp)
	require.NotNil(t, got)
	require.Contains(t, string(got), verifySignatureInvalid)

	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, original, string(rest), "restored 401 body must be complete")
	require.Greater(t, len(rest), 1<<16)
}

// readBodyForRetry never loses bytes: even when the buffered read fails
// partway, what was read stays available to the caller.
func TestReadBodyForRetryKeepsPartialReadOnError(t *testing.T) {
	original := strings.Repeat("x", 1000)

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(&failingReader{data: original}),
	}

	require.Nil(t, readBodyForRetry(resp))

	// The underlying failure still surfaces to the caller, but the bytes
	// buffered before the error are not lost.
	rest, readErr := io.ReadAll(resp.Body)
	require.ErrorIs(t, readErr, io.ErrUnexpectedEOF)
	require.Equal(t, original, string(rest))
}

// failingReader yields its data in small chunks then fails, simulating a
// read error partway through the body.
type failingReader struct {
	data string
	pos  int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}

	n := copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}

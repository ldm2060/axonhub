package usage_monitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

func TestGenericQuotaChecker_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"usage": 500, "quota": 1000},
		})
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "usage", Label: "Usage", Path: "$.data.usage", Type: "jsonpath", Format: "number"},
		{Key: "quota", Label: "Quota", Path: "$.data.quota", Type: "jsonpath", Format: "number"},
	}

	result := checker.TestConnection(context.Background(), server.URL, "GET", nil, "", fields)

	assert.True(t, result.Success)
	assert.NotEmpty(t, result.RawResponse)
	require.Len(t, result.ParsedFields, 2)
	assert.Empty(t, result.ParsedFields[0].Error)
	assert.Equal(t, float64(500), result.ParsedFields[0].Value)
	assert.Empty(t, result.ParsedFields[1].Error)
	assert.Equal(t, float64(1000), result.ParsedFields[1].Value)
}

func TestGenericQuotaChecker_TestConnection_PostWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "test-key", body["api_key"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"balance": 42.5},
		})
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "balance", Label: "Balance", Path: "$.result.balance", Type: "jsonpath", Format: "number"},
	}

	body := `{"api_key":"test-key"}`
	result := checker.TestConnection(context.Background(), server.URL, "POST", nil, body, fields)

	assert.True(t, result.Success)
	require.Len(t, result.ParsedFields, 1)
	assert.Empty(t, result.ParsedFields[0].Error)
	assert.Equal(t, 42.5, result.ParsedFields[0].Value)
}

func TestGenericQuotaChecker_TestConnection_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // Close immediately to simulate network error

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "test", Label: "Test", Path: "$.data", Type: "jsonpath", Format: "number"},
	}

	result := checker.TestConnection(context.Background(), server.URL, "GET", nil, "", fields)

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestGenericQuotaChecker_TestConnection_Non2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "test", Label: "Test", Path: "$.data", Type: "jsonpath", Format: "number"},
	}

	result := checker.TestConnection(context.Background(), server.URL, "GET", nil, "", fields)

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestGenericQuotaChecker_TestConnection_FieldParseFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"usage": 500},
		})
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "usage", Label: "Usage", Path: "$.data.usage", Type: "jsonpath", Format: "number"},
		{Key: "bad_field", Label: "Bad Field", Path: "$.nonexistent.path", Type: "jsonpath", Format: "number"},
	}

	result := checker.TestConnection(context.Background(), server.URL, "GET", nil, "", fields)

	// Overall result should still be success — field parse failures are per-field
	assert.True(t, result.Success)
	require.Len(t, result.ParsedFields, 2)

	// First field parses correctly
	assert.Empty(t, result.ParsedFields[0].Error)
	assert.Equal(t, float64(500), result.ParsedFields[0].Value)

	// Second field has error captured in ParsedField.Error
	assert.NotEmpty(t, result.ParsedFields[1].Error)
	// With two-step parsing, a failed extraction means the variable is missing
	assert.Contains(t, result.ParsedFields[1].Error, "not found")
}

func TestGenericQuotaChecker_TestConnection_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
		})
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "status", Label: "Status", Path: "$.status", Type: "jsonpath", Format: "text"},
	}

	headers := map[string]any{
		"Authorization": "Bearer my-token",
		"X-Custom":      "custom-value",
	}

	result := checker.TestConnection(context.Background(), server.URL, "GET", headers, "", fields)

	assert.True(t, result.Success)
	require.Len(t, result.ParsedFields, 1)
	assert.Equal(t, "ok", result.ParsedFields[0].Value)
}

func TestGenericQuotaChecker_Poll_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"used":  300,
			"limit": 1000,
		})
	}))
	defer server.Close()

	httpClient := httpclient.NewHttpClientWithClient(server.Client())
	checker := NewGenericQuotaChecker(httpClient)

	fields := []FieldConfig{
		{Key: "used", Label: "Used", Path: "$.used", Type: "jsonpath", Format: "number"},
		{Key: "limit", Label: "Limit", Path: "$.limit", Type: "jsonpath", Format: "number"},
	}

	pollData, err := checker.Poll(context.Background(), server.URL, "GET", nil, "", fields)

	require.NoError(t, err)
	assert.NotEmpty(t, pollData.Raw)
	assert.Len(t, pollData.Fields, 2)
	assert.False(t, pollData.PolledAt.IsZero())
}

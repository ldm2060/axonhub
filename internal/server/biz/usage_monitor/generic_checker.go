package usage_monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

// GenericQuotaChecker is a configurable quota checker that can poll any HTTP API
// and extract fields using JSONPath or regex patterns.
type GenericQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

// NewGenericQuotaChecker creates a new GenericQuotaChecker with the given HTTP client.
func NewGenericQuotaChecker(httpClient *httpclient.HttpClient) *GenericQuotaChecker {
	return &GenericQuotaChecker{
		httpClient: httpClient,
	}
}

// Poll executes an HTTP request to the specified API and parses the response fields.
// It returns PollData containing the raw response, parsed fields, and timestamp.
// Individual field parse failures are recorded in the field's Error field but do not cause the overall poll to fail.
func (c *GenericQuotaChecker) Poll(
	ctx context.Context,
	apiURL string,
	apiMethod string,
	apiHeaders map[string]any,
	apiBody string,
	fields []FieldConfig,
) (*PollData, error) {
	// Build the HTTP request
	builder := httpclient.NewRequestBuilder().
		WithMethod(apiMethod).
		WithURL(apiURL)

	// Set headers from apiHeaders map
	for key, value := range apiHeaders {
		switch v := value.(type) {
		case string:
			builder.WithHeader(key, v)
		default:
			builder.WithHeader(key, fmt.Sprintf("%v", v))
		}
	}

	// Add body for POST requests
	apiMethod = strings.ToUpper(apiMethod)
	if apiMethod == http.MethodPost && apiBody != "" {
		builder.WithBody(apiBody)
		if builder.Build().Headers.Get("Content-Type") == "" {
			builder.WithHeader("Content-Type", "application/json")
		}
	}

	request := builder.Build()

	// Execute the request
	resp, err := c.httpClient.Do(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check status code (2xx = success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request returned non-2xx status: %d", resp.StatusCode)
	}

	// Enrich response body with HTTP headers for JSONPath parsing
	enrichedBody := resp.Body
	var rawData interface{}
	if err := json.Unmarshal(resp.Body, &rawData); err == nil {
		// Include response headers in raw data for JSONPath parsing
		headerMap := make(map[string]string, len(resp.Headers))
		for k, v := range resp.Headers {
			if len(v) > 0 {
				headerMap[strings.ToLower(k)] = v[0]
			}
		}
		// Merge headers into raw data under "headers" key
		if rawMap, ok := rawData.(map[string]any); ok {
			rawMap["headers"] = headerMap
			if enriched, err := json.Marshal(rawMap); err == nil {
				enrichedBody = enriched
			}
		}
	}

	// Parse all fields using two-pass ParseFields (supports expression fields)
	parsedFields := ParseFields(enrichedBody, fields)

	// Return PollData with raw response, parsed fields, and timestamp
	return &PollData{
		Raw:      string(resp.Body),
		Fields:   parsedFields,
		PolledAt: time.Now(),
	}, nil
}

// PollV2 executes an HTTP request and uses the two-step parsing pipeline:
// ExtractVariables then RenderDisplayFields.
func (c *GenericQuotaChecker) PollV2(
	ctx context.Context,
	apiURL string,
	apiMethod string,
	apiHeaders map[string]any,
	apiBody string,
	variables []Variable,
	displayFields []DisplayField,
) (*PollData, error) {
	// Build the HTTP request
	builder := httpclient.NewRequestBuilder().
		WithMethod(apiMethod).
		WithURL(apiURL)

	// Set headers from apiHeaders map
	for key, value := range apiHeaders {
		switch v := value.(type) {
		case string:
			builder.WithHeader(key, v)
		default:
			builder.WithHeader(key, fmt.Sprintf("%v", v))
		}
	}

	// Add body for POST requests
	method := strings.ToUpper(apiMethod)
	if method == http.MethodPost && apiBody != "" {
		builder.WithBody(apiBody)
		if builder.Build().Headers.Get("Content-Type") == "" {
			builder.WithHeader("Content-Type", "application/json")
		}
	}

	request := builder.Build()

	// Execute the request
	resp, err := c.httpClient.Do(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check status code (2xx = success)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request returned non-2xx status: %d", resp.StatusCode)
	}

	// Enrich response body with HTTP headers for JSONPath parsing
	enrichedBody := resp.Body
	var rawData interface{}
	if err := json.Unmarshal(resp.Body, &rawData); err == nil {
		headerMap := make(map[string]string, len(resp.Headers))
		for k, v := range resp.Headers {
			if len(v) > 0 {
				headerMap[strings.ToLower(k)] = v[0]
			}
		}
		if rawMap, ok := rawData.(map[string]any); ok {
			rawMap["headers"] = headerMap
			if enriched, err := json.Marshal(rawMap); err == nil {
				enrichedBody = enriched
			}
		}
	}

	// Two-step parsing: extract variables, then render display fields
	vars := ExtractVariables(enrichedBody, variables)
	parsedFields := RenderDisplayFields(vars, displayFields)

	return &PollData{
		Raw:      string(resp.Body),
		Fields:   parsedFields,
		PolledAt: time.Now(),
	}, nil
}

// TestConnection tests the connection to the specified API and returns a TestResult.
// It calls Poll and wraps the result into TestResult.
// On Poll error, it returns TestResult with Success: false and the error message.
func (c *GenericQuotaChecker) TestConnection(
	ctx context.Context,
	apiURL string,
	apiMethod string,
	apiHeaders map[string]any,
	apiBody string,
	fields []FieldConfig,
) *TestResult {
	pollData, err := c.Poll(ctx, apiURL, apiMethod, apiHeaders, apiBody, fields)
	if err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &TestResult{
		Success:      true,
		RawResponse:  pollData.Raw,
		ParsedFields: pollData.Fields,
	}
}

// TestConnectionV2 tests the connection using the two-step parsing pipeline.
// It calls PollV2 and wraps the result into TestResult.
func (c *GenericQuotaChecker) TestConnectionV2(
	ctx context.Context,
	apiURL string,
	apiMethod string,
	apiHeaders map[string]any,
	apiBody string,
	variables []Variable,
	displayFields []DisplayField,
) *TestResult {
	pollData, err := c.PollV2(ctx, apiURL, apiMethod, apiHeaders, apiBody, variables, displayFields)
	if err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &TestResult{
		Success:      true,
		RawResponse:  pollData.Raw,
		ParsedFields: pollData.Fields,
	}
}

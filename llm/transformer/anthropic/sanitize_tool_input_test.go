package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeToolUseInput_ReadDropsEmptyPages(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/demo.py","limit":2000,"offset":0,"pages":""}`)
	result := sanitizeToolUseInput("Read", input)

	var obj map[string]interface{}
	err := json.Unmarshal(result, &obj)
	assert.NoError(t, err)

	assert.NotContains(t, obj, "pages")
	assert.Equal(t, "/tmp/demo.py", obj["file_path"])
	assert.Equal(t, float64(2000), obj["limit"])
}

func TestSanitizeToolUseInput_ReadPreservesNonEmptyPages(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/demo.py","pages":"1-5"}`)
	result := sanitizeToolUseInput("Read", input)

	var obj map[string]interface{}
	err := json.Unmarshal(result, &obj)
	assert.NoError(t, err)

	assert.Equal(t, "1-5", obj["pages"])
	assert.Equal(t, "/tmp/demo.py", obj["file_path"])
}

func TestSanitizeToolUseInput_NonReadToolNoChange(t *testing.T) {
	input := json.RawMessage(`{"query":"","pages":""}`)
	result := sanitizeToolUseInput("search", input)
	assert.Equal(t, string(input), string(result))
}

func TestSanitizeToolUseInput_InvalidJSON(t *testing.T) {
	input := json.RawMessage(`{invalid}`)
	result := sanitizeToolUseInput("Read", input)
	assert.Equal(t, string(input), string(result))
}

func TestSanitizeToolUseInput_EmptyInput(t *testing.T) {
	input := json.RawMessage(``)
	result := sanitizeToolUseInput("Read", input)
	assert.Equal(t, string(input), string(result))
}

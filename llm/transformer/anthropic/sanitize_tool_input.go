package anthropic

import "encoding/json"

// sanitizeToolUseInput removes empty-string fields that are invalid for Anthropic tool_use input.
// Specifically, the "pages" field in the "Read" tool is removed when it is an empty string,
// because OpenAI's Responses API includes `"pages": ""` but Anthropic rejects it.
func sanitizeToolUseInput(name string, input json.RawMessage) json.RawMessage {
	if name != "Read" || len(input) == 0 {
		return input
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(input, &obj); err != nil {
		return input
	}

	pages, ok := obj["pages"]
	if !ok {
		return input
	}

	strVal, ok := pages.(string)
	if ok && strVal == "" {
		delete(obj, "pages")
		sanitized, err := json.Marshal(obj)
		if err != nil {
			return input
		}
		return sanitized
	}

	return input
}

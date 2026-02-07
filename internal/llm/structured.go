package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaValidator validates a parsed struct after JSON extraction.
// Returns nil if valid, or a descriptive error if invalid.
type SchemaValidator[T any] func(T) error

// ExtractJSON extracts a JSON object of type T from raw LLM text output.
// It handles markdown code fences, leading/trailing text, and nested braces.
// If validator is non-nil, the extracted value is validated before return.
func ExtractJSON[T any](raw string, validator SchemaValidator[T]) (T, error) {
	var zero T

	cleaned := stripCodeFences(raw)
	jsonStr := extractJSONBlock(cleaned)
	if jsonStr == "" {
		return zero, fmt.Errorf("%w: no JSON object found in response", ErrInvalidOutput)
	}

	var result T
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	if validator != nil {
		if err := validator(result); err != nil {
			return zero, fmt.Errorf("%w: validation failed: %v", ErrInvalidOutput, err)
		}
	}

	return result, nil
}

// stripCodeFences removes markdown code fences (```json ... ``` or ``` ... ```).
func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				inFence = false
				continue
			}
			inFence = true
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "```") {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// extractJSONBlock finds the first balanced { ... } block in the text.
func extractJSONBlock(s string) string {
	start := strings.IndexByte(s, '{')
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

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
	jsonStr = stripJSONComments(jsonStr)
	jsonStr = normalizeLeadingDecimalNumbers(jsonStr)

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

// stripJSONComments removes C-style line comments (// ...) outside of JSON string
// values. LLMs sometimes emit comments in JSON output despite instructions not to.
func stripJSONComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && inString {
			b.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' {
			b.WriteByte(c)
			inString = !inString
			continue
		}

		if inString {
			b.WriteByte(c)
			continue
		}

		// Line comment: skip to end of line
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i+1 < len(s) && s[i+1] != '\n' {
				i++
			}
			continue
		}

		// Block comment: skip to closing */
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) {
				if s[i] == '*' && s[i+1] == '/' {
					i++
					break
				}
				i++
			}
			continue
		}

		b.WriteByte(c)
	}

	return b.String()
}

// normalizeLeadingDecimalNumbers rewrites invalid JSON numeric literals such as
// ".8" or "-.3" into valid forms "0.8" and "-0.3" outside string values.
func normalizeLeadingDecimalNumbers(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)

	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && inString {
			b.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' {
			b.WriteByte(c)
			inString = !inString
			continue
		}

		if inString {
			b.WriteByte(c)
			continue
		}

		// JSON does not allow ".5" or "-.5". Some models emit these forms.
		if c == '.' && i+1 < len(s) && isDigit(s[i+1]) && isNumericBoundary(prevNonSpace(s, i-1)) {
			b.WriteByte('0')
		}

		b.WriteByte(c)
	}

	return b.String()
}

func prevNonSpace(s string, i int) byte {
	for ; i >= 0; i-- {
		if s[i] != ' ' && s[i] != '\n' && s[i] != '\r' && s[i] != '\t' {
			return s[i]
		}
	}
	return 0
}

func isNumericBoundary(c byte) bool {
	switch c {
	case 0, ':', ',', '[', '{', '-':
		return true
	default:
		return false
	}
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

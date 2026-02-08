package llm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
}

func TestExtractJSON_CleanJSON(t *testing.T) {
	raw := `{"intent":"what_now","confidence":0.95}`
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "what_now", result.Intent)
	assert.Equal(t, 0.95, result.Confidence)
}

func TestExtractJSON_FencedJSON(t *testing.T) {
	raw := "```json\n{\"intent\":\"status\",\"confidence\":0.88}\n```"
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "status", result.Intent)
	assert.Equal(t, 0.88, result.Confidence)
}

func TestExtractJSON_SurroundingText(t *testing.T) {
	raw := "Here is the parsed intent:\n{\"intent\":\"replan\",\"confidence\":0.72}\nHope that helps!"
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "replan", result.Intent)
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	type nested struct {
		Intent string            `json:"intent"`
		Args   map[string]string `json:"args"`
	}
	raw := `{"intent":"project_add","args":{"name":"My Project"}}`
	result, err := ExtractJSON[nested](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "project_add", result.Intent)
	assert.Equal(t, "My Project", result.Args["name"])
}

func TestExtractJSON_NoJSON(t *testing.T) {
	raw := "I don't know what you mean."
	_, err := ExtractJSON[testPayload](raw, nil)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	raw := `{"intent":"what_now", broken}`
	_, err := ExtractJSON[testPayload](raw, nil)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestExtractJSON_ValidationFailure(t *testing.T) {
	raw := `{"intent":"what_now","confidence":1.5}`
	validator := func(p testPayload) error {
		if p.Confidence < 0 || p.Confidence > 1 {
			return fmt.Errorf("confidence must be in [0,1], got %f", p.Confidence)
		}
		return nil
	}
	_, err := ExtractJSON(raw, validator)
	assert.ErrorIs(t, err, ErrInvalidOutput)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestExtractJSON_ValidationSuccess(t *testing.T) {
	raw := `{"intent":"status","confidence":0.9}`
	validator := func(p testPayload) error {
		if p.Confidence < 0 || p.Confidence > 1 {
			return fmt.Errorf("confidence out of range")
		}
		return nil
	}
	result, err := ExtractJSON(raw, validator)
	require.NoError(t, err)
	assert.Equal(t, "status", result.Intent)
}

func TestExtractJSON_EscapedBracesInString(t *testing.T) {
	raw := `{"intent":"what_now","confidence":0.9}`
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "what_now", result.Intent)
}

func TestExtractJSON_MultipleFences(t *testing.T) {
	raw := "Some text\n```\n{\"intent\":\"status\",\"confidence\":0.8}\n```\nMore text"
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "status", result.Intent)
}

func TestExtractJSON_LeadingDecimalConfidence(t *testing.T) {
	raw := `{"intent":"status","confidence":.85}`
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "status", result.Intent)
	assert.Equal(t, 0.85, result.Confidence)
}

func TestExtractJSON_NegativeLeadingDecimal(t *testing.T) {
	type payload struct {
		Value float64 `json:"value"`
	}
	raw := `{"value":-.25}`
	result, err := ExtractJSON[payload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, -0.25, result.Value)
}

func TestExtractJSON_LineComments(t *testing.T) {
	raw := `{
		// This is a comment
		"intent": "what_now",
		"confidence": 0.9 // inline comment
	}`
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "what_now", result.Intent)
	assert.Equal(t, 0.9, result.Confidence)
}

func TestExtractJSON_BlockComments(t *testing.T) {
	raw := `{
		/* block comment */
		"intent": "status",
		"confidence": 0.85
	}`
	result, err := ExtractJSON[testPayload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "status", result.Intent)
	assert.Equal(t, 0.85, result.Confidence)
}

func TestExtractJSON_CommentLikeURLInString(t *testing.T) {
	type payload struct {
		URL string `json:"url"`
	}
	raw := `{"url": "http://example.com/path"}`
	result, err := ExtractJSON[payload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/path", result.URL)
}

func TestExtractJSON_DecimalLikeStringNotNormalized(t *testing.T) {
	type payload struct {
		Note string `json:"note"`
	}
	raw := `{"note":"confidence .85 should stay text"}`
	result, err := ExtractJSON[payload](raw, nil)
	require.NoError(t, err)
	assert.Equal(t, "confidence .85 should stay text", result.Note)
}

package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMClient returns a fixed response for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.GenerateResponse{Text: m.response, Model: "llama3.2"}, nil
}

func (m *mockLLMClient) Available(_ context.Context) bool { return m.err == nil }

func intentJSON(intent ParsedIntent) string {
	data, _ := json.Marshal(intent)
	return string(data)
}

func TestIntentService_Parse_ReadOnlyAutoExecute(t *testing.T) {
	client := &mockLLMClient{response: intentJSON(ParsedIntent{
		Intent:               IntentWhatNow,
		Risk:                 RiskReadOnly,
		Arguments:            map[string]interface{}{"available_min": float64(60)},
		Confidence:           0.95,
		RequiresConfirmation: false,
		ClarificationOptions: []string{},
	})}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	res, err := svc.Parse(context.Background(), "I have 60 minutes what now")

	require.NoError(t, err)
	assert.Equal(t, StateExecuted, res.ExecutionState)
	assert.Equal(t, IntentWhatNow, res.ParsedIntent.Intent)
	assert.Equal(t, RiskReadOnly, res.ParsedIntent.Risk)
}

func TestIntentService_Parse_WriteRequiresConfirmation(t *testing.T) {
	client := &mockLLMClient{response: intentJSON(ParsedIntent{
		Intent:               IntentProjectRemove,
		Risk:                 RiskWrite,
		Arguments:            map[string]interface{}{"project_id": "abc-123"},
		Confidence:           0.99,
		RequiresConfirmation: true,
		ClarificationOptions: []string{},
	})}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	res, err := svc.Parse(context.Background(), "Delete project X")

	require.NoError(t, err)
	assert.Equal(t, StateNeedsConfirmation, res.ExecutionState)
	assert.True(t, res.ParsedIntent.RequiresConfirmation)
	assert.Equal(t, RiskWrite, res.ParsedIntent.Risk)
}

func TestIntentService_Parse_LowConfidenceNeedsClarification(t *testing.T) {
	client := &mockLLMClient{response: intentJSON(ParsedIntent{
		Intent:               IntentWhatNow,
		Risk:                 RiskReadOnly,
		Arguments:            map[string]interface{}{"available_min": float64(30)},
		Confidence:           0.60,
		RequiresConfirmation: false,
		ClarificationOptions: []string{"Show status?", "Get recommendations for 30 min?"},
	})}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	res, err := svc.Parse(context.Background(), "30 minutes something")

	require.NoError(t, err)
	assert.Equal(t, StateNeedsClarification, res.ExecutionState)
}

func TestIntentService_Parse_ArgSchemaRejection(t *testing.T) {
	// what_now with missing available_min
	client := &mockLLMClient{response: intentJSON(ParsedIntent{
		Intent:               IntentWhatNow,
		Risk:                 RiskReadOnly,
		Arguments:            map[string]interface{}{},
		Confidence:           0.90,
		RequiresConfirmation: false,
		ClarificationOptions: []string{},
	})}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	res, err := svc.Parse(context.Background(), "what now?")

	require.NoError(t, err)
	assert.Equal(t, StateRejected, res.ExecutionState)
}

func TestIntentService_Parse_LLMUnavailable(t *testing.T) {
	client := &mockLLMClient{err: llm.ErrOllamaUnavailable}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	_, err := svc.Parse(context.Background(), "what now?")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm parse failed")
}

func TestIntentService_Parse_InvalidOutputFormat(t *testing.T) {
	client := &mockLLMClient{response: "I don't understand what you mean."}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	_, err := svc.Parse(context.Background(), "something weird")

	var parseErr *ParsedIntentError
	assert.ErrorAs(t, err, &parseErr)
	assert.Equal(t, ErrCodeInvalidOutputFormat, parseErr.Code)
}

func TestIntentService_Parse_WriteIntentMarkedAsReadOnly_EnforcedToWrite(t *testing.T) {
	// Adversarial: LLM marks a write intent as read_only.
	// Safety layer must catch and enforce.
	client := &mockLLMClient{response: intentJSON(ParsedIntent{
		Intent:               IntentProjectRemove,
		Risk:                 RiskReadOnly, // WRONG — LLM error or injection
		Arguments:            map[string]interface{}{"project_id": "abc"},
		Confidence:           0.99,
		RequiresConfirmation: false, // WRONG
		ClarificationOptions: []string{},
	})}

	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
	res, err := svc.Parse(context.Background(), "remove project abc")

	require.NoError(t, err)
	// Must be enforced to write + needs confirmation.
	assert.Equal(t, RiskWrite, res.ParsedIntent.Risk)
	assert.True(t, res.ParsedIntent.RequiresConfirmation)
	assert.Equal(t, StateNeedsConfirmation, res.ExecutionState)
}

func TestIntentService_Parse_PromptInjection_WriteNeverAutoExecuted(t *testing.T) {
	injections := []struct {
		name   string
		intent ParsedIntent
	}{
		{
			name: "injection attempts read_only delete",
			intent: ParsedIntent{
				Intent: IntentProjectRemove, Risk: RiskReadOnly,
				Arguments: map[string]interface{}{"project_id": "all"},
				Confidence: 1.0, RequiresConfirmation: false,
				ClarificationOptions: []string{},
			},
		},
		{
			name: "injection max confidence write",
			intent: ParsedIntent{
				Intent: IntentProjectArchive, Risk: RiskWrite,
				Arguments: map[string]interface{}{"project_id": "important"},
				Confidence: 1.0, RequiresConfirmation: false,
				ClarificationOptions: []string{},
			},
		},
	}

	for _, tt := range injections {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockLLMClient{response: intentJSON(tt.intent)}
			svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))
			res, err := svc.Parse(context.Background(), fmt.Sprintf("injection: %s", tt.name))

			require.NoError(t, err)
			// Write intents must NEVER be auto-executed.
			assert.NotEqual(t, StateExecuted, res.ExecutionState,
				"write intent %s must never auto-execute", tt.intent.Intent)
			assert.True(t, res.ParsedIntent.RequiresConfirmation)
			assert.Equal(t, RiskWrite, res.ParsedIntent.Risk)
		})
	}
}

package intelligence

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/llm"
)

// IntentService parses natural language text into structured intents.
type IntentService interface {
	Parse(ctx context.Context, text string) (*AskResolution, error)
}

type intentService struct {
	client   llm.LLMClient
	observer llm.Observer
	policy   ConfirmationPolicy
}

// NewIntentService creates an IntentService backed by an LLM client.
func NewIntentService(client llm.LLMClient, observer llm.Observer, policy ConfirmationPolicy) IntentService {
	return &intentService{
		client:   client,
		observer: observer,
		policy:   policy,
	}
}

func (s *intentService) Parse(ctx context.Context, text string) (*AskResolution, error) {
	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskParse,
		SystemPrompt: buildParseSystemPrompt(),
		UserPrompt:   text,
	})
	if err != nil {
		return nil, fmt.Errorf("llm parse failed: %w", err)
	}

	intent, err := llm.ExtractJSON[ParsedIntent](resp.Text, validateParsedIntent)
	if err != nil {
		return nil, &ParsedIntentError{
			Code:    ErrCodeInvalidOutputFormat,
			Message: fmt.Sprintf("failed to extract intent: %v", err),
		}
	}

	// Hard safety: enforce write classification regardless of LLM output.
	EnforceWriteSafety(&intent)

	// Validate arguments against intent-specific schema.
	if argErr := ValidateIntentArguments(intent.Intent, intent.Arguments); argErr != nil {
		return &AskResolution{
			ParsedIntent:     &intent,
			ExecutionState:   StateRejected,
			ExecutionMessage: argErr.Message,
		}, nil
	}

	// Apply confirmation policy.
	state := s.policy.Evaluate(&intent)
	msg := executionMessage(state, &intent)

	return &AskResolution{
		ParsedIntent:     &intent,
		ExecutionState:   state,
		ExecutionMessage: msg,
	}, nil
}

// validateParsedIntent is a schema validator for ExtractJSON.
func validateParsedIntent(p ParsedIntent) error {
	if !IsValidIntent(p.Intent) {
		return fmt.Errorf("unknown intent: %s", p.Intent)
	}
	if p.Risk != RiskReadOnly && p.Risk != RiskWrite {
		return fmt.Errorf("risk must be 'read_only' or 'write', got %q", p.Risk)
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return fmt.Errorf("confidence must be in [0,1], got %f", p.Confidence)
	}
	return nil
}

func executionMessage(state ExecutionState, intent *ParsedIntent) string {
	switch state {
	case StateExecuted:
		return fmt.Sprintf("Executing %s (confidence: %.0f%%)", intent.Intent, intent.Confidence*100)
	case StateNeedsConfirmation:
		return fmt.Sprintf("Parsed as %s (write operation). Confirm to proceed.", intent.Intent)
	case StateNeedsClarification:
		return fmt.Sprintf("Low confidence (%.0f%%) for %s. Please clarify.", intent.Confidence*100, intent.Intent)
	case StateRejected:
		return fmt.Sprintf("Cannot execute %s: invalid arguments.", intent.Intent)
	default:
		return ""
	}
}

package intelligence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/alexanderramin/kairos/internal/template"
)

// TemplateDraftService generates template JSON from natural language descriptions.
type TemplateDraftService interface {
	Draft(ctx context.Context, prompt string) (*TemplateDraft, error)
}

type templateDraftService struct {
	client   llm.LLMClient
	observer llm.Observer
}

// NewTemplateDraftService creates a TemplateDraftService backed by an LLM client.
func NewTemplateDraftService(client llm.LLMClient, observer llm.Observer) TemplateDraftService {
	return &templateDraftService{client: client, observer: observer}
}

func (s *templateDraftService) Draft(ctx context.Context, prompt string) (*TemplateDraft, error) {
	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskTemplateDraft,
		SystemPrompt: templateDraftSystemPrompt,
		UserPrompt:   prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("llm template draft failed: %w", err)
	}

	// Extract the raw JSON object from the LLM response.
	var rawTemplate map[string]interface{}
	extracted, err := llm.ExtractJSON[map[string]interface{}](resp.Text, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to extract template JSON: %w", err)
	}
	rawTemplate = extracted

	// Re-marshal and unmarshal into TemplateSchema for validation.
	schemaJSON, err := json.Marshal(rawTemplate)
	if err != nil {
		return &TemplateDraft{
			TemplateJSON: rawTemplate,
			Validation:   TemplateDraftValidation{IsValid: false, Errors: []string{"failed to marshal template"}},
			Confidence:   0.3,
		}, nil
	}

	var schema template.TemplateSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return &TemplateDraft{
			TemplateJSON: rawTemplate,
			Validation:   TemplateDraftValidation{IsValid: false, Errors: []string{fmt.Sprintf("invalid template structure: %v", err)}},
			Confidence:   0.3,
		}, nil
	}

	// Run deterministic validation.
	validationErrors := template.ValidateSchema(&schema)
	errStrings := make([]string, len(validationErrors))
	for i, e := range validationErrors {
		errStrings[i] = e.Error()
	}

	isValid := len(validationErrors) == 0
	confidence := 0.8
	if !isValid {
		confidence = 0.4
	}

	var repairs []string
	if !isValid {
		repairs = append(repairs, "Fix the validation errors listed above and re-validate.")
	}

	return &TemplateDraft{
		TemplateJSON:      rawTemplate,
		Validation:        TemplateDraftValidation{IsValid: isValid, Errors: errStrings, Warnings: []string{}},
		RepairSuggestions: repairs,
		Confidence:        confidence,
	}, nil
}

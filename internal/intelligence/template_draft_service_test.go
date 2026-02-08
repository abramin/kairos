package intelligence

import (
	"context"
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templateDraftMockClient is a mock LLM client for template draft tests.
// Uses a unique name to avoid conflicts with mockLLMClient and draftMockClient.
type templateDraftMockClient struct {
	response string
	err      error
}

func (m *templateDraftMockClient) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.GenerateResponse{Text: m.response, Model: "test"}, nil
}

func (m *templateDraftMockClient) Available(ctx context.Context) bool {
	return m.err == nil
}

func validTemplateJSON() string {
	return `{
		"id": "test-template",
		"name": "Test Template",
		"description": "A test template for unit testing",
		"domain": "education",
		"nodes": [
			{
				"id": "n1",
				"title": "Node 1",
				"kind": "generic"
			}
		],
		"work_items": [
			{
				"id": "w1",
				"node_id": "n1",
				"title": "Task 1",
				"type": "task",
				"planned_min": 60
			}
		]
	}`
}

func invalidTemplateJSON_MissingID() string {
	return `{
		"id": "",
		"name": "Test Template",
		"description": "Missing ID",
		"domain": "education",
		"nodes": [
			{
				"id": "n1",
				"title": "Node 1",
				"kind": "generic"
			}
		],
		"work_items": [
			{
				"id": "w1",
				"node_id": "n1",
				"title": "Task 1",
				"type": "task"
			}
		]
	}`
}

func invalidTemplateJSON_MissingName() string {
	return `{
		"id": "test-template",
		"name": "",
		"description": "Missing name",
		"domain": "education",
		"nodes": [
			{
				"id": "n1",
				"title": "Node 1",
				"kind": "generic"
			}
		],
		"work_items": [
			{
				"id": "w1",
				"node_id": "n1",
				"title": "Task 1",
				"type": "task"
			}
		]
	}`
}

func invalidTemplateJSON_NoNodes() string {
	return `{
		"id": "test-template",
		"name": "Test Template",
		"description": "No nodes",
		"domain": "education",
		"nodes": [],
		"work_items": []
	}`
}

func TestTemplateDraftService_ValidTemplate(t *testing.T) {
	client := &templateDraftMockClient{response: validTemplateJSON()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a test template")

	require.NoError(t, err)
	assert.True(t, draft.Validation.IsValid, "template should be valid")
	assert.Empty(t, draft.Validation.Errors, "should have no validation errors")
	assert.GreaterOrEqual(t, draft.Confidence, 0.7, "confidence should be >= 0.7 for valid template")
	assert.Equal(t, "test-template", draft.TemplateJSON["id"])
	assert.Equal(t, "Test Template", draft.TemplateJSON["name"])
	assert.Equal(t, "education", draft.TemplateJSON["domain"])
}

func TestTemplateDraftService_InvalidTemplate_MissingID(t *testing.T) {
	client := &templateDraftMockClient{response: invalidTemplateJSON_MissingID()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template with missing ID")

	require.NoError(t, err, "should not error, just return invalid draft")
	assert.False(t, draft.Validation.IsValid, "template should be invalid")
	assert.NotEmpty(t, draft.Validation.Errors, "should have validation errors")
	assert.Contains(t, draft.Validation.Errors[0], "id is required")
	assert.Less(t, draft.Confidence, 0.7, "confidence should be lower for invalid template")
}

func TestTemplateDraftService_InvalidTemplate_MissingName(t *testing.T) {
	client := &templateDraftMockClient{response: invalidTemplateJSON_MissingName()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template with missing name")

	require.NoError(t, err)
	assert.False(t, draft.Validation.IsValid)
	assert.NotEmpty(t, draft.Validation.Errors)
	assert.Contains(t, draft.Validation.Errors[0], "name is required")
}

func TestTemplateDraftService_InvalidTemplate_NoNodes(t *testing.T) {
	client := &templateDraftMockClient{response: invalidTemplateJSON_NoNodes()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template with no nodes")

	require.NoError(t, err)
	assert.False(t, draft.Validation.IsValid)
	assert.NotEmpty(t, draft.Validation.Errors)
	// Should have errors about missing nodes and work items
	errorText := fmt.Sprintf("%v", draft.Validation.Errors)
	assert.Contains(t, errorText, "node")
	assert.Contains(t, errorText, "work item")
}

func TestTemplateDraftService_LLMError(t *testing.T) {
	client := &templateDraftMockClient{err: llm.ErrOllamaUnavailable}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	_, err := svc.Draft(context.Background(), "Create a template")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm template draft failed")
}

func TestTemplateDraftService_InvalidJSON(t *testing.T) {
	client := &templateDraftMockClient{response: "This is not JSON at all, just plain text."}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	_, err := svc.Draft(context.Background(), "Create a template")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract template JSON")
}

func TestTemplateDraftService_MarkdownFencedJSON(t *testing.T) {
	fencedJSON := "```json\n" + validTemplateJSON() + "\n```"
	client := &templateDraftMockClient{response: fencedJSON}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template")

	require.NoError(t, err, "should extract JSON from markdown fences")
	assert.True(t, draft.Validation.IsValid)
	assert.Equal(t, "test-template", draft.TemplateJSON["id"])
}

func TestTemplateDraftService_RepairSuggestions(t *testing.T) {
	client := &templateDraftMockClient{response: invalidTemplateJSON_MissingID()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template")

	require.NoError(t, err)
	assert.False(t, draft.Validation.IsValid)
	assert.NotEmpty(t, draft.RepairSuggestions, "should provide repair suggestions for invalid template")
	assert.Contains(t, draft.RepairSuggestions[0], "Fix the validation errors")
}

func TestTemplateDraftService_RepairSuggestions_EmptyForValidTemplate(t *testing.T) {
	client := &templateDraftMockClient{response: validTemplateJSON()}
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "Create a template")

	require.NoError(t, err)
	assert.True(t, draft.Validation.IsValid)
	assert.Empty(t, draft.RepairSuggestions, "valid template should have no repair suggestions")
}

func TestTemplateDraftService_ConfidenceScoring(t *testing.T) {
	tests := []struct {
		name               string
		response           string
		expectedValid      bool
		expectedConfidence float64
	}{
		{
			name:               "valid template has high confidence",
			response:           validTemplateJSON(),
			expectedValid:      true,
			expectedConfidence: 0.8,
		},
		{
			name:               "invalid template has low confidence",
			response:           invalidTemplateJSON_MissingID(),
			expectedValid:      false,
			expectedConfidence: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &templateDraftMockClient{response: tt.response}
			svc := NewTemplateDraftService(client, llm.NoopObserver{})

			draft, err := svc.Draft(context.Background(), "Create a template")

			require.NoError(t, err)
			assert.Equal(t, tt.expectedValid, draft.Validation.IsValid)
			assert.Equal(t, tt.expectedConfidence, draft.Confidence)
		})
	}
}

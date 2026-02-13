package intelligence

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateDraftService_Draft_WithHTTPTestServer exercises the full HTTP
// serialization path: httptest → OllamaClient → TemplateDraftService.Draft →
// template schema extraction and validation.
func TestTemplateDraftService_Draft_WithHTTPTestServer(t *testing.T) {
	templateJSON := `{
		"name": "Weekly Study Plan",
		"domain": "education",
		"version": "1.0",
		"defaults": {},
		"variables": [],
		"nodes": [
			{
				"title": "Week 1",
				"kind": "week",
				"items": [
					{"title": "Reading", "type": "reading", "planned_min": 60, "min_session_min": 15, "max_session_min": 60, "default_session_min": 30, "splittable": true}
				]
			}
		]
	}`

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": templateJSON,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewTemplateDraftService(client, llm.NoopObserver{})

	draft, err := svc.Draft(context.Background(), "create a weekly study plan")
	require.NoError(t, err)
	require.NotNil(t, draft)

	assert.NotNil(t, draft.TemplateJSON)
	assert.Greater(t, draft.Confidence, 0.0)
}

// TestProjectDraftService_Start_WithHTTPTestServer exercises the full HTTP
// path for ProjectDraftService: httptest → OllamaClient → Start → JSON extraction.
func TestProjectDraftService_Start_WithHTTPTestServer(t *testing.T) {
	draftResp := draftTurnResponse{
		Message: "I'll help you create a study plan. How many weeks?",
		Status:  "gathering",
		Draft:   nil,
	}
	respJSON, err := json.Marshal(draftResp)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(respJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	conv, err := svc.Start(context.Background(), "I need to study for a physics exam")
	require.NoError(t, err)
	require.NotNil(t, conv)

	assert.Equal(t, DraftStatusGathering, conv.Status)
	assert.Contains(t, conv.LLMMessage, "study plan")
	assert.GreaterOrEqual(t, len(conv.Turns), 2) // user + assistant
}

// TestProjectDraftService_NextTurn_Ready_WithHTTPTestServer verifies that when
// the LLM signals "ready" with a draft, the full HTTP path correctly deserializes
// the ImportSchema.
func TestProjectDraftService_NextTurn_Ready_WithHTTPTestServer(t *testing.T) {
	targetDate := "2026-06-01"
	pm60 := 60
	readyDraft := draftTurnResponse{
		Message: "Your project is ready to import.",
		Status:  "ready",
		Draft: &importer.ImportSchema{
			Project: importer.ProjectImport{
				ShortID:    "PHY01",
				Name:       "Physics Exam Prep",
				Domain:     "education",
				StartDate:  "2026-02-01",
				TargetDate: &targetDate,
			},
			Nodes: []importer.NodeImport{
				{Ref: "n1", Title: "Week 1", Kind: "week", Order: 1},
			},
			WorkItems: []importer.WorkItemImport{
				{Ref: "w1", NodeRef: "n1", Title: "Study", Type: "reading", PlannedMin: &pm60},
			},
		},
	}
	respJSON, err := json.Marshal(readyDraft)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(respJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	// Start a conversation (same endpoint, always returns "ready").
	conv, err := svc.Start(context.Background(), "physics exam prep")
	require.NoError(t, err)

	// If Start already returned "ready", verify the draft.
	if conv.Status == DraftStatusReady {
		require.NotNil(t, conv.Draft)
		assert.Equal(t, "PHY01", conv.Draft.Project.ShortID)
		assert.Equal(t, "Physics Exam Prep", conv.Draft.Project.Name)
		assert.Len(t, conv.Draft.Nodes, 1)
		assert.Len(t, conv.Draft.WorkItems, 1)
		return
	}

	// Otherwise, advance via NextTurn.
	conv, err = svc.NextTurn(context.Background(), conv, "sounds good")
	require.NoError(t, err)
	assert.Equal(t, DraftStatusReady, conv.Status)
	require.NotNil(t, conv.Draft)
	assert.Equal(t, "PHY01", conv.Draft.Project.ShortID)
}

package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pipelineMockLLMClient struct {
	response string
}

func (m *pipelineMockLLMClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	return &llm.GenerateResponse{
		Text:  m.response,
		Model: "mock",
	}, nil
}

func (m *pipelineMockLLMClient) Available(_ context.Context) bool {
	return true
}

func TestDraftImportSchedulePipeline(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	draftSvc := intelligence.NewProjectDraftService(&pipelineMockLLMClient{
		response: `{
			"message": "Draft is ready to import.",
			"status": "ready",
			"draft": {
				"project": {
					"short_id": "DRF01",
					"name": "Drafted Study Plan",
					"domain": "education",
					"start_date": "2026-01-10",
					"target_date": "2026-03-20"
				},
				"defaults": {
					"duration_mode": "estimate",
					"session_policy": {
						"min_session_min": 15,
						"max_session_min": 60,
						"default_session_min": 30,
						"splittable": true
					}
				},
				"nodes": [
					{"ref": "n1", "title": "Week 1", "kind": "week", "order": 0}
				],
				"work_items": [
					{
						"ref": "w1",
						"node_ref": "n1",
						"title": "Read Chapter 1",
						"type": "reading",
						"planned_min": 120
					}
				]
			}
		}`,
	}, llm.NoopObserver{})

	importSvc := NewImportService(projects, nodes, workItems, deps, uow)
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)

	conv, err := draftSvc.Start(ctx, "Build a study plan for an upcoming exam.")
	require.NoError(t, err)
	require.Equal(t, intelligence.DraftStatusReady, conv.Status)
	require.NotNil(t, conv.Draft)

	importResult, err := importSvc.ImportProjectFromSchema(ctx, conv.Draft)
	require.NoError(t, err)
	require.NotNil(t, importResult.Project)

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	require.NotEmpty(t, candidates)

	now := time.Now().UTC()
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	req.ProjectScope = []string{importResult.Project.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)
	for _, rec := range resp.Recommendations {
		assert.Equal(t, importResult.Project.ID, rec.ProjectID)
	}
}

func TestDraftImportSchedulePipeline_HTTPBoundary(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	// Real HTTP boundary: httptest server -> OllamaClient -> ProjectDraftService.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model": "test-model",
			"response": `{
				"message": "Draft is ready to import.",
				"status": "ready",
				"draft": {
					"project": {
						"short_id": "DRF02",
						"name": "Drafted HTTP Study Plan",
						"domain": "education",
						"start_date": "2026-01-10",
						"target_date": "2026-03-20"
					},
					"defaults": {
						"duration_mode": "estimate",
						"session_policy": {
							"min_session_min": 15,
							"max_session_min": 60,
							"default_session_min": 30,
							"splittable": true
						}
					},
					"nodes": [
						{"ref": "n1", "title": "Week 1", "kind": "week", "order": 0}
					],
					"work_items": [
						{
							"ref": "w1",
							"node_ref": "n1",
							"title": "Read Chapter 1",
							"type": "reading",
							"planned_min": 120
						}
					]
				}
			}`,
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping HTTP boundary test: %v", err)
	}
	srv := &http.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ln)
		close(done)
	}()
	defer func() {
		_ = srv.Close()
		_ = ln.Close()
		<-done
	}()

	cfg := llm.DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = "http://" + ln.Addr().String()
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	draftSvc := intelligence.NewProjectDraftService(llm.NewOllamaClient(cfg, llm.NoopObserver{}), llm.NoopObserver{})
	importSvc := NewImportService(projects, nodes, workItems, deps, uow)
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)

	conv, err := draftSvc.Start(ctx, "Build a study plan for an upcoming exam.")
	require.NoError(t, err)
	require.Equal(t, intelligence.DraftStatusReady, conv.Status)
	require.NotNil(t, conv.Draft)

	importResult, err := importSvc.ImportProjectFromSchema(ctx, conv.Draft)
	require.NoError(t, err)
	require.NotNil(t, importResult.Project)

	now := time.Now().UTC()
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	req.ProjectScope = []string{importResult.Project.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)
	for _, rec := range resp.Recommendations {
		assert.Equal(t, importResult.Project.ID, rec.ProjectID)
	}
}

func TestTemplateInitSchedulePipeline(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	templateDir := findTemplatesDir(t)
	templateSvc := NewTemplateService(templateDir, projects, nodes, workItems, deps, uow)
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)

	due := time.Now().UTC().AddDate(0, 2, 0).Format("2006-01-02")
	proj, err := templateSvc.InitProject(ctx, "course_weekly_generic", "Template Pipeline", "TPL02", "2026-01-15", &due, map[string]string{
		"weeks":            "2",
		"assignment_count": "1",
	})
	require.NoError(t, err)

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)

	foundTemplateProject := false
	for _, c := range candidates {
		if c.ProjectID == proj.ID {
			foundTemplateProject = true
			break
		}
	}
	assert.True(t, foundTemplateProject, "template-created project should yield schedulable candidates")

	now := time.Now().UTC()
	req := contract.NewWhatNowRequest(90)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)
	for _, rec := range resp.Recommendations {
		assert.Equal(t, proj.ID, rec.ProjectID)
	}
}

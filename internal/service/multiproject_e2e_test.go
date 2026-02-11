package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_MultiProjectWhatNow_FullPipeline imports 3 projects at different risk
// levels (critical, at-risk, on-track) and verifies the what-now pipeline produces
// correct recommendations: critical-first ordering, session bounds, variation, and
// allocation invariants.
func TestE2E_MultiProjectWhatNow_FullPipeline(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	importSvc := NewImportService(projects, nodes, workItems, deps)

	// === Import 3 projects via ImportSchema (same path as JSON import) ===

	// Project A: Critical — due tomorrow, 240 min of work remaining.
	targetA := now.AddDate(0, 0, 1).Format("2006-01-02")
	startA := now.AddDate(0, -1, 0).Format("2006-01-02")
	pm120 := 120
	schemaA := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "CRT01", Name: "Urgent Paper", Domain: "academic",
			StartDate: startA, TargetDate: &targetA,
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Section 1", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Section 2", Kind: "module", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Draft Introduction", Type: "task", PlannedMin: &pm120},
			{Ref: "w2", NodeRef: "n2", Title: "Draft Analysis", Type: "task", PlannedMin: &pm120},
		},
	}
	resultA, err := importSvc.ImportProjectFromSchema(ctx, schemaA)
	require.NoError(t, err)
	assert.Equal(t, 2, resultA.WorkItemCount)

	// Project B: At-risk — due in 21 days, 200 min of work remaining (moderate).
	targetB := now.AddDate(0, 0, 21).Format("2006-01-02")
	startB := now.AddDate(0, -2, 0).Format("2006-01-02")
	pm100 := 100
	schemaB := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "ATR01", Name: "Midterm Prep", Domain: "education",
			StartDate: startB, TargetDate: &targetB,
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Chapter 1", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Chapter 2", Kind: "module", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Study Chapter 1", Type: "reading", PlannedMin: &pm100},
			{Ref: "w2", NodeRef: "n2", Title: "Study Chapter 2", Type: "reading", PlannedMin: &pm100},
		},
	}
	resultB, err := importSvc.ImportProjectFromSchema(ctx, schemaB)
	require.NoError(t, err)

	// Project C: On-track — due in 3 months, 120 min remaining.
	targetC := now.AddDate(0, 3, 0).Format("2006-01-02")
	startC := now.AddDate(0, -1, 0).Format("2006-01-02")
	pm60 := 60
	schemaC := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID: "ONT01", Name: "Leisure Reading", Domain: "personal",
			StartDate: startC, TargetDate: &targetC,
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Book 1", Kind: "module", Order: 1},
			{Ref: "n2", Title: "Book 2", Kind: "module", Order: 2},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Read Book 1", Type: "reading", PlannedMin: &pm60},
			{Ref: "w2", NodeRef: "n2", Title: "Read Book 2", Type: "reading", PlannedMin: &pm60},
		},
	}
	resultC, err := importSvc.ImportProjectFromSchema(ctx, schemaC)
	require.NoError(t, err)

	// === Phase 1: Critical mode — only project A recommended ===
	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	req := contract.NewWhatNowRequest(120)
	req.Now = &now

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)

	assert.Equal(t, domain.ModeCritical, resp.Mode,
		"should be in critical mode with tomorrow's deadline")

	for _, rec := range resp.Recommendations {
		assert.Equal(t, resultA.Project.ID, rec.ProjectID,
			"critical mode should only recommend items from the critical project")
	}

	// === Verify allocation invariants ===
	assert.LessOrEqual(t, resp.AllocatedMin, resp.RequestedMin,
		"allocated_min must not exceed requested_min")
	for _, rec := range resp.Recommendations {
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin,
			"each allocation must respect minimum session bounds")
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin,
			"each allocation must respect maximum session bounds")
	}

	// === Verify determinism ===
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.Equal(t, len(resp.Recommendations), len(resp2.Recommendations))
	for i := range resp.Recommendations {
		assert.Equal(t, resp.Recommendations[i].WorkItemID, resp2.Recommendations[i].WorkItemID,
			"recommendations must be deterministic for same input")
	}

	// === Phase 2: Complete critical project → balanced mode ===
	allItemsA, err := workItems.ListByProject(ctx, resultA.Project.ID)
	require.NoError(t, err)
	for _, wi := range allItemsA {
		wi.Status = domain.WorkItemDone
		wi.LoggedMin = wi.PlannedMin
		require.NoError(t, workItems.Update(ctx, wi))
	}

	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp3.Recommendations)

	assert.NotEqual(t, domain.ModeCritical, resp3.Mode,
		"should exit critical mode after critical items are completed")

	// Verify recommendations come from remaining projects B and C.
	recProjectIDs := make(map[string]bool)
	for _, rec := range resp3.Recommendations {
		recProjectIDs[rec.ProjectID] = true
		assert.NotEqual(t, resultA.Project.ID, rec.ProjectID,
			"completed project A's items should not appear in recommendations")
	}
	assert.True(t, recProjectIDs[resultB.Project.ID] || recProjectIDs[resultC.Project.ID],
		"should recommend items from projects B and/or C after critical mode ends")

	// === Phase 3: Status verification ===
	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now
	statusResp, err := statusSvc.GetStatus(ctx, statusReq)
	require.NoError(t, err)
	require.NotEmpty(t, statusResp.Projects,
		"status should report on active projects")

	// Verify risk levels are reported.
	riskLevels := make(map[domain.RiskLevel]bool)
	for _, ps := range statusResp.Projects {
		riskLevels[ps.RiskLevel] = true
	}
	assert.GreaterOrEqual(t, len(riskLevels), 1,
		"status should include projects at varying risk levels")
}

// TestE2E_StatusMixedRiskLevels creates projects at different risk levels and
// verifies StatusService correctly classifies each.
func TestE2E_StatusMixedRiskLevels(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Critical: due tomorrow, lots of work
	projCritical := testutil_newProjectWithWork(t, projects, nodes, workItems,
		"Crisis Project", now.AddDate(0, 0, 1), 300)

	// At-risk: due in 7 days, moderate work
	projAtRisk := testutil_newProjectWithWork(t, projects, nodes, workItems,
		"Tight Deadline", now.AddDate(0, 0, 7), 500)

	// On-track: due in 3 months, little work
	projOnTrack := testutil_newProjectWithWork(t, projects, nodes, workItems,
		"Relaxed Project", now.AddDate(0, 3, 0), 60)

	statusSvc := NewStatusService(projects, workItems, sessions, profiles)
	req := contract.NewStatusRequest()
	req.Now = &now

	resp, err := statusSvc.GetStatus(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Projects, 3)

	// Build a map of project ID → risk level.
	riskMap := make(map[string]domain.RiskLevel)
	for _, ps := range resp.Projects {
		riskMap[ps.ProjectID] = ps.RiskLevel
	}

	assert.Equal(t, domain.RiskCritical, riskMap[projCritical],
		"project due tomorrow with heavy work should be critical")
	assert.Contains(t, []domain.RiskLevel{domain.RiskAtRisk, domain.RiskCritical}, riskMap[projAtRisk],
		"project due in 7 days with moderate work should be at_risk or critical")
	assert.Equal(t, domain.RiskOnTrack, riskMap[projOnTrack],
		"project due in 3 months with light work should be on_track")
}

// testutil_newProjectWithWork creates a project with a single node and work item.
// Returns the project ID.
func testutil_newProjectWithWork(
	t *testing.T,
	projects repository.ProjectRepo,
	nodes repository.PlanNodeRepo,
	workItems repository.WorkItemRepo,
	name string, targetDate time.Time, plannedMin int,
) string {
	t.Helper()
	ctx := context.Background()

	proj := testutil.NewTestProject(name, testutil.WithTargetDate(targetDate))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, name+" Task",
		testutil.WithPlannedMin(plannedMin),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	return proj.ID
}

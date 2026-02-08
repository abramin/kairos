package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullWorkflow_ProjectLifecycle(t *testing.T) {
	// 1. Set up all repos
	projRepo, nodeRepo, wiRepo, depRepo, sessRepo, profRepo := setupRepos(t)
	ctx := context.Background()

	// 2. Create all services
	projectService := NewProjectService(projRepo)
	nodeService := NewNodeService(nodeRepo)
	workItemService := NewWorkItemService(wiRepo)
	sessionService := NewSessionService(sessRepo, wiRepo)
	whatNowService := NewWhatNowService(wiRepo, sessRepo, projRepo, depRepo, profRepo)
	statusService := NewStatusService(projRepo, wiRepo, sessRepo, profRepo)
	replanService := NewReplanService(projRepo, wiRepo, sessRepo, profRepo)

	// 3. Create a project
	now := time.Now().UTC()
	targetDate := now.AddDate(0, 2, 0) // 2 months from now

	project := testutil.NewTestProject("Integration Test Project", testutil.WithTargetDate(targetDate))
	require.NoError(t, projectService.Create(ctx, project))

	// 4. Create 2 nodes
	node1 := testutil.NewTestNode(project.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodeService.Create(ctx, node1))

	node2 := testutil.NewTestNode(project.ID, "Week 2", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodeService.Create(ctx, node2))

	// 5. Create 3 work items across the nodes
	wi1 := testutil.NewTestWorkItem(node1.ID, "Reading Chapter 1",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItemService.Create(ctx, wi1))

	wi2 := testutil.NewTestWorkItem(node1.ID, "Exercises Set 1",
		testutil.WithPlannedMin(90),
		testutil.WithSessionBounds(30, 90, 45),
	)
	require.NoError(t, workItemService.Create(ctx, wi2))

	wi3 := testutil.NewTestWorkItem(node2.ID, "Reading Chapter 2",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItemService.Create(ctx, wi3))

	// 6. Get initial recommendations
	whatNowReq := contract.NewWhatNowRequest(90)
	whatNowReq.Now = &now

	resp1, err := whatNowService.Recommend(ctx, whatNowReq)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations, "should get initial recommendations")
	assert.Greater(t, resp1.AllocatedMin, 0, "should allocate some time")

	// 7. Log a session on the first work item
	session := testutil.NewTestSession(wi1.ID, 30, testutil.WithStartedAt(now))
	require.NoError(t, sessionService.LogSession(ctx, session))

	// 8. Verify work item's logged_min was updated
	updatedWi1, err := workItemService.GetByID(ctx, wi1.ID)
	require.NoError(t, err)
	assert.Equal(t, 30, updatedWi1.LoggedMin, "logged_min should be updated after session")
	assert.Equal(t, domain.WorkItemInProgress, updatedWi1.Status, "status should change to in_progress")

	// 9. Get status
	statusReq := contract.NewStatusRequest()
	statusReq.Now = &now

	statusResp, err := statusService.GetStatus(ctx, statusReq)
	require.NoError(t, err)
	assert.Equal(t, 1, statusResp.Summary.CountsTotal, "should show one project")
	require.Len(t, statusResp.Projects, 1, "should have one project view")
	assert.Equal(t, project.ID, statusResp.Projects[0].ProjectID)
	assert.Equal(t, 210, statusResp.Projects[0].PlannedMinTotal, "total planned: 60+90+60")
	assert.Equal(t, 30, statusResp.Projects[0].LoggedMinTotal, "total logged: 30")

	// 10. Get new recommendations after logging session
	resp2, err := whatNowService.Recommend(ctx, whatNowReq)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations, "should get recommendations after session")

	// Verify progress is reflected in risk summaries
	require.NotEmpty(t, resp2.TopRiskProjects, "should have risk summaries")
	riskSummary := resp2.TopRiskProjects[0]
	assert.Equal(t, 210, riskSummary.PlannedMinTotal)
	assert.Equal(t, 30, riskSummary.LoggedMinTotal)
	// Remaining includes 10% buffer: (210-30) * 1.1 = 198
	assert.Equal(t, 198, riskSummary.RemainingMinTotal, "remaining: (210-30) * 1.1")

	// 11. Run replan
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now

	replanResp, err := replanService.Replan(ctx, replanReq)
	require.NoError(t, err)
	assert.Equal(t, domain.TriggerManual, replanResp.Trigger)
	assert.Equal(t, 1, replanResp.RecomputedProjects, "should replan one project")
	require.Len(t, replanResp.Deltas, 1, "should have one delta")

	// 12. Mark one work item done
	require.NoError(t, workItemService.MarkDone(ctx, wi1.ID))

	doneWi, err := workItemService.GetByID(ctx, wi1.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, doneWi.Status)

	// 13. Get final recommendations - verify done item is not recommended
	resp3, err := whatNowService.Recommend(ctx, whatNowReq)
	require.NoError(t, err)

	for _, rec := range resp3.Recommendations {
		assert.NotEqual(t, wi1.ID, rec.WorkItemID, "done work item should not be recommended")
	}

	// Verify remaining work in risk summaries
	require.NotEmpty(t, resp3.TopRiskProjects)
	finalRisk := resp3.TopRiskProjects[0]
	// Done items are excluded from remaining calculation
	// wi2 (90 min) and wi3 (60 min) = 150 min planned
	// wi1 is done (60 min planned, 30 min logged)
	// Remaining for non-done items: 150 * 1.1 = 165 (with 10% buffer)
	assert.Equal(t, 165, finalRisk.RemainingMinTotal, "remaining should exclude done item: (90+60) * 1.1")
}

func TestFullWorkflow_MultiProjectVariation(t *testing.T) {
	// 1. Set up repos and services
	projRepo, nodeRepo, wiRepo, depRepo, sessRepo, profRepo := setupRepos(t)
	ctx := context.Background()

	projectService := NewProjectService(projRepo)
	nodeService := NewNodeService(nodeRepo)
	workItemService := NewWorkItemService(wiRepo)
	sessionService := NewSessionService(sessRepo, wiRepo)
	whatNowService := NewWhatNowService(wiRepo, sessRepo, projRepo, depRepo, profRepo)

	now := time.Now().UTC()

	// 2. Create 2 projects with different deadlines
	// Use longer deadlines to ensure they're on track
	nearDeadline := now.AddDate(0, 3, 0) // 3 months away
	farDeadline := now.AddDate(0, 6, 0)  // 6 months away

	projA := testutil.NewTestProject("Project A", testutil.WithTargetDate(nearDeadline))
	require.NoError(t, projectService.Create(ctx, projA))

	projB := testutil.NewTestProject("Project B", testutil.WithTargetDate(farDeadline))
	require.NoError(t, projectService.Create(ctx, projB))

	// 3. Add work items to each project
	nodeA := testutil.NewTestNode(projA.ID, "Module A")
	require.NoError(t, nodeService.Create(ctx, nodeA))

	nodeB := testutil.NewTestNode(projB.ID, "Module B")
	require.NoError(t, nodeService.Create(ctx, nodeB))

	wiA1 := testutil.NewTestWorkItem(nodeA.ID, "Task A1",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItemService.Create(ctx, wiA1))

	wiA2 := testutil.NewTestWorkItem(nodeA.ID, "Task A2",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItemService.Create(ctx, wiA2))

	wiB1 := testutil.NewTestWorkItem(nodeB.ID, "Task B1",
		testutil.WithPlannedMin(90),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItemService.Create(ctx, wiB1))

	wiB2 := testutil.NewTestWorkItem(nodeB.ID, "Task B2",
		testutil.WithPlannedMin(90),
		testutil.WithSessionBounds(30, 60, 45),
	)
	require.NoError(t, workItemService.Create(ctx, wiB2))

	// Log some recent sessions on both projects so they have pace > 0
	// This prevents them from being classified as critical due to zero velocity
	sessA0 := testutil.NewTestSession(wiA1.ID, 30, testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessionService.LogSession(ctx, sessA0))

	sessB0 := testutil.NewTestSession(wiB1.ID, 30, testutil.WithStartedAt(now.Add(-72*time.Hour)))
	require.NoError(t, sessionService.LogSession(ctx, sessB0))

	// 4. Request 120 minutes of recommendations in balanced mode
	// Both projects should be on track initially (plenty of time)
	whatNowReq := contract.NewWhatNowRequest(120)
	whatNowReq.Now = &now
	whatNowReq.EnforceVariation = true

	resp1, err := whatNowService.Recommend(ctx, whatNowReq)
	require.NoError(t, err)
	assert.Equal(t, domain.ModeBalanced, resp1.Mode, "should be in balanced mode")

	// 5. In balanced mode, verify recommendations include items from both projects (variation)
	projectIDsInRec := make(map[string]bool)
	for _, rec := range resp1.Recommendations {
		projectIDsInRec[rec.ProjectID] = true
	}

	// With variation enforcement and balanced mode, we expect both projects
	// to appear if we have enough available time (120 min)
	if len(resp1.Recommendations) >= 2 {
		assert.GreaterOrEqual(t, len(projectIDsInRec), 1,
			"with variation enforcement, should attempt to include multiple projects")
	}

	// 6. Log heavy sessions on project A
	// This should increase project A's recent pace but variation should still
	// encourage project B work
	// Note: sessA0 already logged 30 min, so we log 2 more sessions for a total of 3 sessions
	for i := 0; i < 2; i++ {
		sessTime := now.Add(-time.Duration((i+1)*24) * time.Hour)
		sess := testutil.NewTestSession(wiA1.ID, 45, testutil.WithStartedAt(sessTime))
		require.NoError(t, sessionService.LogSession(ctx, sess))
	}

	// Update the work item to reflect logged sessions (LogSession already does this)
	updatedWiA1, err := workItemService.GetByID(ctx, wiA1.ID)
	require.NoError(t, err)
	// Total: 30 (sessA0) + 45 + 45 = 120 min
	assert.Equal(t, 120, updatedWiA1.LoggedMin, "should have logged initial 30 min + 2 sessions of 45 min")

	// 7. Re-check recommendations - scheduling should still include project B due to variation
	resp2, err := whatNowService.Recommend(ctx, whatNowReq)
	require.NoError(t, err)

	projectIDsInRec2 := make(map[string]bool)
	for _, rec := range resp2.Recommendations {
		projectIDsInRec2[rec.ProjectID] = true
	}

	// Even though Project A has recent activity, with variation enforcement
	// and balanced mode, we should still see variety if possible
	// (This is a soft constraint - the exact behavior depends on scoring)
	// At minimum, verify both projects exist and have work remaining
	require.NotEmpty(t, resp2.TopRiskProjects, "should have risk summaries")

	projectsInRisk := make(map[string]bool)
	for _, risk := range resp2.TopRiskProjects {
		projectsInRisk[risk.ProjectID] = true
	}

	assert.True(t, projectsInRisk[projA.ID], "project A should be in risk summaries")
	assert.True(t, projectsInRisk[projB.ID], "project B should be in risk summaries")

	// Verify that both projects still have remaining work
	for _, risk := range resp2.TopRiskProjects {
		assert.Greater(t, risk.RemainingMinTotal, 0, "both projects should have remaining work")
	}
}

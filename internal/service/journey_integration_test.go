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

// TestFullUserJourney_CreatePopulateScheduleLogReplan exercises the core value loop:
// create project → add nodes → add work items → what-now → log session → replan → verify re-estimation.
func TestFullUserJourney_CreatePopulateScheduleLogReplan(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0) // 2 months from now

	// === Step 1: Create project ===
	proj := testutil.NewTestProject("Study Plan", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	// === Step 2: Add nodes ===
	nodeW1 := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	nodeW2 := testutil.NewTestNode(proj.ID, "Week 2", testutil.WithNodeKind(domain.NodeWeek))
	require.NoError(t, nodes.Create(ctx, nodeW1))
	require.NoError(t, nodes.Create(ctx, nodeW2))

	// === Step 3: Add work items (with units tracking for re-estimation) ===
	wiRead := testutil.NewTestWorkItem(nodeW1.ID, "Read Chapters 1-5",
		testutil.WithPlannedMin(100),
		testutil.WithUnits("chapters", 5, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	wiExercise := testutil.NewTestWorkItem(nodeW1.ID, "Exercises Set 1",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 45, 30),
	)
	wiRead2 := testutil.NewTestWorkItem(nodeW2.ID, "Read Chapters 6-10",
		testutil.WithPlannedMin(100),
		testutil.WithUnits("chapters", 5, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiRead))
	require.NoError(t, workItems.Create(ctx, wiExercise))
	require.NoError(t, workItems.Create(ctx, wiRead2))

	// === Step 4: What-now recommendation ===
	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations, "should produce at least one recommendation")

	// Verify invariant: allocated_min <= requested_min
	assert.LessOrEqual(t, resp.AllocatedMin, resp.RequestedMin,
		"allocated_min must not exceed requested_min")
	for _, rec := range resp.Recommendations {
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin, "allocated must respect max session")
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin, "allocated must respect min session")
	}

	// === Step 5: Log a session (simulates user doing work) ===
	sessionSvc := NewSessionService(sessions, uow)
	sess := &domain.WorkSessionLog{
		WorkItemID:     wiRead.ID,
		StartedAt:      now.Add(-time.Hour),
		Minutes:        45,
		UnitsDoneDelta: 2, // Read 2 of 5 chapters
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess))

	// Verify work item was updated with logged time and units
	updatedWI, err := workItems.GetByID(ctx, wiRead.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, updatedWI.LoggedMin, "logged_min should reflect session")
	assert.Equal(t, 2, updatedWI.UnitsDone, "units_done should reflect session")
	assert.Equal(t, domain.WorkItemInProgress, updatedWI.Status, "status should auto-update to in_progress")

	// Verify re-estimation happened during session logging
	// 45 min for 2 chapters → implied 112.5 total for 5 chapters
	// Smooth: round(0.7*100 + 0.3*112.5) = round(70 + 33.75) = 104
	assert.NotEqual(t, 100, updatedWI.PlannedMin, "planned_min should be re-estimated after session with units")

	// === Step 6: Replan ===
	replanSvc := NewReplanService(projects, workItems, sessions, profiles, uow)
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now

	replanResp, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)
	require.Len(t, replanResp.Deltas, 1, "should have one project delta")
	assert.Equal(t, proj.ID, replanResp.Deltas[0].ProjectID)
	assert.NotEmpty(t, string(replanResp.Deltas[0].RiskBefore), "risk levels should be populated")
	assert.NotEmpty(t, string(replanResp.Deltas[0].RiskAfter), "risk levels should be populated")

	// === Step 7: Post-replan what-now should still work (deterministic) ===
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations, "should still produce recommendations after replan")

	// Verify determinism: same request should produce same output
	resp3, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.Equal(t, len(resp2.Recommendations), len(resp3.Recommendations))
	for i := range resp2.Recommendations {
		assert.Equal(t, resp2.Recommendations[i].WorkItemID, resp3.Recommendations[i].WorkItemID)
	}
}

// TestReplan_Idempotent_UnchangedInput verifies the documented invariant:
// "Replan is idempotent over unchanged input."
func TestReplan_Idempotent_UnchangedInput(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Idempotent Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item WITHOUT units tracking — replan won't re-estimate
	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(120),
		testutil.WithLoggedMin(30),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	// Log a session so we have activity
	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles, uow)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	// First replan
	resp1, err := svc.Replan(ctx, req)
	require.NoError(t, err)

	// Second replan with identical input
	resp2, err := svc.Replan(ctx, req)
	require.NoError(t, err)

	// Responses should have matching structure
	require.Equal(t, len(resp1.Deltas), len(resp2.Deltas))
	for i := range resp1.Deltas {
		assert.Equal(t, resp1.Deltas[i].ProjectID, resp2.Deltas[i].ProjectID)
		assert.Equal(t, resp1.Deltas[i].RiskBefore, resp2.Deltas[i].RiskBefore)
		assert.Equal(t, resp1.Deltas[i].RiskAfter, resp2.Deltas[i].RiskAfter)
		assert.Equal(t, resp1.Deltas[i].RemainingMinBefore, resp2.Deltas[i].RemainingMinBefore)
		assert.Equal(t, resp1.Deltas[i].RemainingMinAfter, resp2.Deltas[i].RemainingMinAfter)
		assert.Equal(t, resp1.Deltas[i].ChangedItemsCount, resp2.Deltas[i].ChangedItemsCount)
	}
	assert.Equal(t, resp1.GlobalModeAfter, resp2.GlobalModeAfter)

	// Verify DB state unchanged between replans
	finalWI, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 120, finalWI.PlannedMin, "planned_min should be unchanged (no units tracking)")
}

// TestReplan_Idempotent_WithUnitsTracking verifies idempotency when
// re-estimation converges (running replan twice produces no further changes).
func TestReplan_Idempotent_WithUnitsTracking(t *testing.T) {
	projects, nodes, workItems, _, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Converge Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item WITH units: 60 logged / 3 done → implied 200 min for 10 total
	wi := testutil.NewTestWorkItem(node.ID, "Read",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(60),
		testutil.WithUnits("chapters", 10, 3),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	sess := testutil.NewTestSession(wi.ID, 30, testutil.WithStartedAt(now.Add(-24*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sess))

	svc := NewReplanService(projects, workItems, sessions, profiles, uow)
	req := contract.NewReplanRequest(domain.TriggerManual)
	req.Now = &now

	// Run until convergence
	for i := 0; i < 30; i++ {
		resp, err := svc.Replan(ctx, req)
		require.NoError(t, err)
		if resp.Deltas[0].ChangedItemsCount == 0 {
			break
		}
	}

	// Now run twice more — both should show zero changes
	resp1, err := svc.Replan(ctx, req)
	require.NoError(t, err)
	resp2, err := svc.Replan(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, 0, resp1.Deltas[0].ChangedItemsCount, "should show no changes after convergence")
	assert.Equal(t, 0, resp2.Deltas[0].ChangedItemsCount, "should be idempotent after convergence")
	assert.Equal(t, resp1.Deltas[0].RiskAfter, resp2.Deltas[0].RiskAfter)
}

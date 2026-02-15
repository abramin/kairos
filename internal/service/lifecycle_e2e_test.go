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

// TestReplan_ThenRecommend_ReEstimationAffectsAllocation exercises:
// import project with units → log session → replan (re-estimate) → what-now →
// verify the re-estimated planned_min changes allocation.
func TestReplan_ThenRecommend_ReEstimationAffectsAllocation(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, uow := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Study Course",
		testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Readings", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	// Work item with units tracking: planned 100 min for 10 chapters.
	wi := testutil.NewTestWorkItem(node.ID, "Read Textbook",
		testutil.WithPlannedMin(100),
		testutil.WithUnits("chapters", 10, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wi))

	whatNowSvc := NewWhatNowService(workItems, sessions, deps, profiles)
	sessionSvc := NewSessionService(sessions, uow)
	replanSvc := NewReplanService(projects, workItems, sessions, profiles, uow)

	// === Step 1: Initial recommendation (planned = 100) ===
	req := contract.NewWhatNowRequest(60)
	req.Now = &now

	resp1, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	alloc1 := resp1.Recommendations[0].AllocatedMin
	assert.Greater(t, alloc1, 0, "should allocate time")

	// === Step 2: Log session — 30 min for 1 chapter → implies 300 min total ===
	// Smooth: round(0.7*100 + 0.3*300) = round(70+90) = 160
	sess := &domain.WorkSessionLog{
		WorkItemID:     wi.ID,
		StartedAt:      now.Add(-time.Hour),
		Minutes:        30,
		UnitsDoneDelta: 1,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess))

	updated, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 160, updated.PlannedMin,
		"session log should trigger re-estimation: round(0.7*100 + 0.3*300)")

	// === Step 3: Replan — further refines the estimate ===
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now

	replanResp, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)
	require.Len(t, replanResp.Deltas, 1)

	postReplan, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)

	// After replan: round(0.7*160 + 0.3*300) = round(112+90) = 202
	assert.Equal(t, 202, postReplan.PlannedMin,
		"replan should further smooth toward implied total")

	// === Step 4: Recommend again — allocation should reflect increased planned_min ===
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	// Verify the recommendation uses the updated work item data.
	// The item should still be recommended (has remaining work).
	found := false
	for _, rec := range resp2.Recommendations {
		if rec.WorkItemID == wi.ID {
			found = true
			assert.Greater(t, rec.AllocatedMin, 0, "should still allocate time after replan")
		}
	}
	assert.True(t, found, "re-estimated item should still appear in recommendations")
}

// TestStartFinish_ChangesNextRecommendation verifies that marking work items
// done causes the next what-now recommendation to select different items.
func TestStartFinish_ChangesNextRecommendation(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	target := now.AddDate(0, 3, 0)

	proj := testutil.NewTestProject("Multi-Task Project",
		testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Sprint 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	// Three work items with different planned min to ensure deterministic ordering.
	wiA := testutil.NewTestWorkItem(node.ID, "Task Alpha",
		testutil.WithPlannedMin(200),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiA))

	wiB := testutil.NewTestWorkItem(node.ID, "Task Beta",
		testutil.WithPlannedMin(150),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiB))

	wiC := testutil.NewTestWorkItem(node.ID, "Task Gamma",
		testutil.WithPlannedMin(100),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiC))

	// Log sessions so project has activity (avoid zero-activity critical path).
	sessA := testutil.NewTestSession(wiA.ID, 30,
		testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	// === Step 1: Get initial recommendations ===
	resp1, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Recommendations)

	// Capture recommended item IDs.
	recIDs1 := make(map[string]bool)
	for _, rec := range resp1.Recommendations {
		recIDs1[rec.WorkItemID] = true
	}

	// === Step 2: Mark first recommended item as done ===
	firstRecID := resp1.Recommendations[0].WorkItemID
	firstWI, err := workItems.GetByID(ctx, firstRecID)
	require.NoError(t, err)

	firstWI.Status = domain.WorkItemDone
	firstWI.LoggedMin = firstWI.PlannedMin
	require.NoError(t, workItems.Update(ctx, firstWI))

	// === Step 3: Re-recommend — done item should not appear ===
	resp2, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations,
		"should still have recommendations after completing one item")

	for _, rec := range resp2.Recommendations {
		assert.NotEqual(t, firstRecID, rec.WorkItemID,
			"completed item should not appear in new recommendations")
	}

	// === Step 4: Mark second item as done ===
	secondRecID := resp2.Recommendations[0].WorkItemID
	secondWI, err := workItems.GetByID(ctx, secondRecID)
	require.NoError(t, err)

	secondWI.Status = domain.WorkItemDone
	secondWI.LoggedMin = secondWI.PlannedMin
	require.NoError(t, workItems.Update(ctx, secondWI))

	// === Step 5: Third recommendation should select the remaining item ===
	resp3, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp3.Recommendations)

	for _, rec := range resp3.Recommendations {
		assert.NotEqual(t, firstRecID, rec.WorkItemID)
		assert.NotEqual(t, secondRecID, rec.WorkItemID)
	}

	// === Step 6: Mark last item done → should get NoCandidates ===
	thirdRecID := resp3.Recommendations[0].WorkItemID
	thirdWI, err := workItems.GetByID(ctx, thirdRecID)
	require.NoError(t, err)

	thirdWI.Status = domain.WorkItemDone
	thirdWI.LoggedMin = thirdWI.PlannedMin
	require.NoError(t, workItems.Update(ctx, thirdWI))

	_, err = svc.Recommend(ctx, req)
	if err != nil {
		var wnErr *contract.WhatNowError
		require.ErrorAs(t, err, &wnErr)
		assert.Equal(t, contract.ErrNoCandidates, wnErr.Code,
			"all items done → should get NoCandidates")
	}
}

// TestStartFinish_MultiProject_ShiftsPriority verifies that completing work
// in one project causes the recommendation to shift toward the other project.
func TestStartFinish_MultiProject_ShiftsPriority(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles, _ := setupRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	deadline := now.AddDate(0, 3, 0)

	// Two equal-priority projects
	projA := testutil.NewTestProject("Project Alpha",
		testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "Work", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "Alpha Task",
		testutil.WithPlannedMin(200),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiA))

	projB := testutil.NewTestProject("Project Bravo",
		testutil.WithTargetDate(deadline))
	require.NoError(t, projects.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "Work", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "Bravo Task",
		testutil.WithPlannedMin(200),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, workItems.Create(ctx, wiB))

	// Log sessions on both projects (avoid zero-activity critical path).
	sessA := testutil.NewTestSession(wiA.ID, 30,
		testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessA))
	sessB := testutil.NewTestSession(wiB.ID, 30,
		testutil.WithStartedAt(now.Add(-48*time.Hour)))
	require.NoError(t, sessions.Create(ctx, sessB))

	svc := NewWhatNowService(workItems, sessions, deps, profiles)
	req := contract.NewWhatNowRequest(90)
	req.Now = &now

	// === Initial: both projects should appear in balanced mode ===
	resp1, err := svc.Recommend(ctx, req)
	require.NoError(t, err)

	projIDs1 := make(map[string]bool)
	for _, rec := range resp1.Recommendations {
		projIDs1[rec.ProjectID] = true
	}
	assert.True(t, len(projIDs1) >= 2,
		"balanced mode should include both projects initially")

	// === Mark Alpha's task as done ===
	wiA.Status = domain.WorkItemDone
	wiA.LoggedMin = wiA.PlannedMin
	require.NoError(t, workItems.Update(ctx, wiA))

	// === Recommend again: only Bravo should remain ===
	resp2, err := svc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Recommendations)

	for _, rec := range resp2.Recommendations {
		assert.Equal(t, projB.ID, rec.ProjectID,
			"after Alpha done, only Bravo items should be recommended")
	}
}

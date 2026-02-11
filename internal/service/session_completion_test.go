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

// TestSessionCompletion_ExcludesFromWhatNow verifies the critical invariant:
// completing a work item (done status) causes it to be excluded from future
// what-now recommendations.
func TestSessionCompletion_ExcludesFromWhatNow(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Completion Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	// Create two work items so the project still has work after completing one.
	wiToComplete := testutil.NewTestWorkItem(node.ID, "Completable Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiToComplete))

	wiRemaining := testutil.NewTestWorkItem(node.ID, "Remaining Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wiRemaining))

	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems)

	// Step 1: Both items should be schedulable.
	req := contract.NewWhatNowRequest(120)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)

	recIDs := make(map[string]bool)
	for _, rec := range resp.Recommendations {
		recIDs[rec.WorkItemID] = true
	}
	assert.True(t, recIDs[wiToComplete.ID] || recIDs[wiRemaining.ID],
		"at least one of the work items should be recommended initially")

	// Step 2: Log a session → item transitions to in_progress.
	sess1 := &domain.WorkSessionLog{
		WorkItemID: wiToComplete.ID,
		StartedAt:  now.Add(-time.Hour),
		Minutes:    30,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess1))

	updated, err := workItems.GetByID(ctx, wiToComplete.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updated.Status,
		"should auto-transition to in_progress after first session")
	assert.Equal(t, 30, updated.LoggedMin)

	// Step 3: Mark as done (simulates user finishing the item).
	updated.Status = domain.WorkItemDone
	updated.LoggedMin = 60
	require.NoError(t, workItems.Update(ctx, updated))

	// Step 4: What-now should no longer include the completed item.
	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)

	for _, rec := range resp2.Recommendations {
		assert.NotEqual(t, wiToComplete.ID, rec.WorkItemID,
			"completed (done) item must be excluded from recommendations")
	}

	// The remaining item should still be recommended.
	if len(resp2.Recommendations) > 0 {
		recIDs2 := make(map[string]bool)
		for _, rec := range resp2.Recommendations {
			recIDs2[rec.WorkItemID] = true
		}
		assert.True(t, recIDs2[wiRemaining.ID],
			"remaining non-completed item should still be recommended")
	}
}

// TestSessionCompletion_FullLifecycle exercises the complete status transition
// chain: todo → in_progress → done, verifying each transition's effect on scheduling.
func TestSessionCompletion_FullLifecycle(t *testing.T) {
	projects, nodes, workItems, deps, sessions, profiles := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Lifecycle Test", testutil.WithTargetDate(target))
	require.NoError(t, projects.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Module 1", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Lifecycle Item",
		testutil.WithPlannedMin(90),
		testutil.WithUnits("sections", 3, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, workItems.Create(ctx, wi))

	whatNowSvc := NewWhatNowService(workItems, sessions, projects, deps, profiles)
	sessionSvc := NewSessionService(sessions, workItems)
	req := contract.NewWhatNowRequest(60)
	req.Now = &now
	req.ProjectScope = []string{proj.ID}

	// Phase 1: todo → item should be schedulable.
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recommendations)

	found := false
	for _, rec := range resp.Recommendations {
		if rec.WorkItemID == wi.ID {
			found = true
		}
	}
	assert.True(t, found, "todo item should be in recommendations")

	// Phase 2: Log session 1 → in_progress, still schedulable.
	sess := &domain.WorkSessionLog{
		WorkItemID:     wi.ID,
		StartedAt:      now.Add(-2 * time.Hour),
		Minutes:        30,
		UnitsDoneDelta: 1,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess))

	updated, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updated.Status)
	assert.Equal(t, 30, updated.LoggedMin)
	assert.Equal(t, 1, updated.UnitsDone)
	// Re-estimation: implied = (30/1)*3 = 90, smooth = round(0.7*90 + 0.3*90) = 90 (unchanged)
	// (Because implied exactly matches planned, no change expected.)

	resp2, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	found = false
	for _, rec := range resp2.Recommendations {
		if rec.WorkItemID == wi.ID {
			found = true
		}
	}
	assert.True(t, found, "in_progress item should still be in recommendations")

	// Phase 3: Log remaining sessions → mark done → excluded.
	sess2 := &domain.WorkSessionLog{
		WorkItemID:     wi.ID,
		StartedAt:      now.Add(-time.Hour),
		Minutes:        30,
		UnitsDoneDelta: 1,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess2))

	sess3 := &domain.WorkSessionLog{
		WorkItemID:     wi.ID,
		StartedAt:      now.Add(-30 * time.Minute),
		Minutes:        30,
		UnitsDoneDelta: 1,
	}
	require.NoError(t, sessionSvc.LogSession(ctx, sess3))

	// All 3 units done → mark as done.
	final, err := workItems.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, final.UnitsDone, "all units should be done")
	assert.Equal(t, 90, final.LoggedMin, "all time should be logged")

	final.Status = domain.WorkItemDone
	require.NoError(t, workItems.Update(ctx, final))

	// What-now should now exclude the completed item. Since this is the only
	// item in the project, we expect either an empty result or NO_CANDIDATES error.
	resp3, err := whatNowSvc.Recommend(ctx, req)
	if err != nil {
		// NO_CANDIDATES is the expected error when all items are done.
		assert.Contains(t, err.Error(), "NO_CANDIDATES",
			"error should be NO_CANDIDATES when all items are done")
	} else {
		for _, rec := range resp3.Recommendations {
			assert.NotEqual(t, wi.ID, rec.WorkItemID,
				"done item must not appear in recommendations")
		}
	}
}

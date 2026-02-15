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

// TestSessionDelete_DoesNotRollBackLoggedMin documents that deleting a session
// does NOT decrement the work item's logged_min. This is intentional: session
// deletion is atomic and does not trigger compensating updates.
func TestSessionDelete_DoesNotRollBackLoggedMin(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Deletion Test")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapter",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo, uow)

	// Log a session — logged_min should increase.
	sess := testutil.NewTestSession(wi.ID, 45)
	require.NoError(t, svc.LogSession(ctx, sess))

	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, updated.LoggedMin)

	// Delete the session.
	require.NoError(t, svc.Delete(ctx, sess.ID))

	// logged_min should remain unchanged (no compensation).
	afterDelete, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, afterDelete.LoggedMin,
		"logged_min should NOT be rolled back on session delete (intentional: atomic deletion)")
}

// TestSessionDelete_DoesNotAffectReEstimation documents that deleting a session
// with units does NOT reverse the re-estimation that occurred during logging.
func TestSessionDelete_DoesNotAffectReEstimation(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Re-Est Deletion")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item with units tracking: 100 min planned for 10 chapters.
	wi := testutil.NewTestWorkItem(node.ID, "Read Textbook",
		testutil.WithPlannedMin(100),
		testutil.WithUnits("chapters", 10, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo, uow)

	// Log session: 60 min for 3 chapters → pace = 20 min/ch → implied = 200
	// Smooth: round(0.7*100 + 0.3*200) = 130
	sess := testutil.NewTestSession(wi.ID, 60, testutil.WithUnitsDelta(3))
	require.NoError(t, svc.LogSession(ctx, sess))

	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	plannedAfterLog := updated.PlannedMin
	assert.NotEqual(t, 100, plannedAfterLog, "should be re-estimated after session with units")

	// Delete the session.
	require.NoError(t, svc.Delete(ctx, sess.ID))

	// planned_min should remain at the re-estimated value (no reversal).
	afterDelete, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, plannedAfterLog, afterDelete.PlannedMin,
		"planned_min should NOT revert on session delete (intentional: re-estimation is not reversible)")
}

// TestSessionDelete_SessionNoLongerListed verifies that a deleted session
// no longer appears in ListByWorkItem or ListRecent queries.
func TestSessionDelete_SessionNoLongerListed(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("List After Delete")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo, uow)

	// Log two sessions.
	sess1 := testutil.NewTestSession(wi.ID, 30)
	sess2 := testutil.NewTestSession(wi.ID, 20)
	require.NoError(t, svc.LogSession(ctx, sess1))
	require.NoError(t, svc.LogSession(ctx, sess2))

	// Both should be listed.
	sessions, err := svc.ListByWorkItem(ctx, wi.ID)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)

	// Delete one session.
	require.NoError(t, svc.Delete(ctx, sess1.ID))

	// Only sess2 should remain.
	sessions, err = svc.ListByWorkItem(ctx, wi.ID)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, sess2.ID, sessions[0].ID)

	// ListRecent should also not include the deleted session.
	recent, err := svc.ListRecent(ctx, 30)
	require.NoError(t, err)
	for _, r := range recent {
		assert.NotEqual(t, sess1.ID, r.ID, "deleted session should not appear in recent list")
	}
}

// TestSessionDelete_WorkItemStatusPreserved documents that deleting a session
// does not revert the work item's auto-transitioned status.
func TestSessionDelete_WorkItemStatusPreserved(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Status Preserve")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(60),
		testutil.WithWorkItemStatus(domain.WorkItemTodo),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo, uow)

	// Log session → auto-transitions to in_progress.
	sess := testutil.NewTestSession(wi.ID, 20)
	require.NoError(t, svc.LogSession(ctx, sess))

	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updated.Status)

	// Delete the session.
	require.NoError(t, svc.Delete(ctx, sess.ID))

	// Status should remain in_progress (no revert).
	afterDelete, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, afterDelete.Status,
		"status should remain in_progress after session delete (intentional: no status rollback)")
}

// TestSessionDelete_ReplanConvergesAfterDeletion chains session deletion with
// replan and what-now to verify the pipeline remains consistent when logged_min
// is stale relative to actual sessions.
func TestSessionDelete_ReplanConvergesAfterDeletion(t *testing.T) {
	projRepo, nodeRepo, wiRepo, depRepo, sessRepo, profRepo, uow := setupRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()
	target := now.AddDate(0, 2, 0)

	proj := testutil.NewTestProject("Chain Test", testutil.WithTargetDate(target))
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Module 1")
	require.NoError(t, nodeRepo.Create(ctx, node))

	// Units-tracked work item: 100 min planned for 10 chapters.
	wi := testutil.NewTestWorkItem(node.ID, "Read Textbook",
		testutil.WithPlannedMin(100),
		testutil.WithUnits("chapters", 10, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, wiRepo.Create(ctx, wi))

	sessSvc := NewSessionService(sessRepo, wiRepo, uow)
	replanSvc := NewReplanService(projRepo, wiRepo, sessRepo, profRepo)
	whatNowSvc := NewWhatNowService(wiRepo, sessRepo, depRepo, profRepo)

	// Log two sessions with units (triggers re-estimation each time).
	sess1 := testutil.NewTestSession(wi.ID, 30, testutil.WithUnitsDelta(2))
	sess2 := testutil.NewTestSession(wi.ID, 40, testutil.WithUnitsDelta(3))
	require.NoError(t, sessSvc.LogSession(ctx, sess1))
	require.NoError(t, sessSvc.LogSession(ctx, sess2))

	afterLog, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 70, afterLog.LoggedMin, "logged_min should accumulate both sessions")
	assert.NotEqual(t, 100, afterLog.PlannedMin, "planned_min should be re-estimated")

	// Delete one session — logged_min stays stale (intentional, no compensation).
	require.NoError(t, sessSvc.Delete(ctx, sess1.ID))

	afterDelete, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 70, afterDelete.LoggedMin, "logged_min unchanged after delete")

	// Replan should converge to a stable state.
	replanReq := contract.NewReplanRequest(domain.TriggerManual)
	replanReq.Now = &now
	resp1, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)

	resp2, err := replanSvc.Replan(ctx, replanReq)
	require.NoError(t, err)
	assert.Equal(t, resp1.RecomputedProjects, resp2.RecomputedProjects,
		"replan should converge: second run should match first")

	// What-now should still produce valid recommendations.
	wnReq := contract.NewWhatNowRequest(60)
	wnReq.Now = &now
	wnResp, err := whatNowSvc.Recommend(ctx, wnReq)
	require.NoError(t, err)
	require.NotEmpty(t, wnResp.Recommendations,
		"what-now should still recommend items after session delete + replan")

	for _, rec := range wnResp.Recommendations {
		assert.GreaterOrEqual(t, rec.AllocatedMin, rec.MinSessionMin,
			"allocation invariant: min session bound")
		assert.LessOrEqual(t, rec.AllocatedMin, rec.MaxSessionMin,
			"allocation invariant: max session bound")
	}

	// Determinism: same input → same output.
	wnResp2, err := whatNowSvc.Recommend(ctx, wnReq)
	require.NoError(t, err)
	require.Equal(t, len(wnResp.Recommendations), len(wnResp2.Recommendations))
	for i := range wnResp.Recommendations {
		assert.Equal(t, wnResp.Recommendations[i].WorkItemID, wnResp2.Recommendations[i].WorkItemID,
			"recommendations must be deterministic after delete+replan")
	}
}

package service

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionDelete_DoesNotRollBackLoggedMin documents that deleting a session
// does NOT decrement the work item's logged_min. This is intentional: session
// deletion is atomic and does not trigger compensating updates.
func TestSessionDelete_DoesNotRollBackLoggedMin(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _ := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Deletion Test")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapter",
		testutil.WithPlannedMin(120),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo)

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
	projRepo, nodes, wiRepo, _, sessRepo, _ := setupRepos(t)
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

	svc := NewSessionService(sessRepo, wiRepo)

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
	projRepo, nodes, wiRepo, _, sessRepo, _ := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("List After Delete")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(60),
		testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, wiRepo)

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
	projRepo, nodes, wiRepo, _, sessRepo, _ := setupRepos(t)
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

	svc := NewSessionService(sessRepo, wiRepo)

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

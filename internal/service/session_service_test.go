package service

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogSession_UpdatesLoggedMin(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Study")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Read Chapter",
		testutil.WithPlannedMin(120),
		testutil.WithLoggedMin(0),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, uow)

	sess := testutil.NewTestSession(wi.ID, 45)
	require.NoError(t, svc.LogSession(ctx, sess))

	// Verify logged_min was updated
	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 45, updated.LoggedMin, "logged_min should be incremented by session minutes")

	// Log another session
	sess2 := testutil.NewTestSession(wi.ID, 30)
	require.NoError(t, svc.LogSession(ctx, sess2))

	updated2, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 75, updated2.LoggedMin, "logged_min should accumulate across sessions")
}

func TestLogSession_AutoTransitionsToInProgress(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Study")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task",
		testutil.WithPlannedMin(60),
		testutil.WithWorkItemStatus(domain.WorkItemTodo),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, wiRepo.Create(ctx, wi))

	assert.Equal(t, domain.WorkItemTodo, wi.Status, "should start as todo")

	svc := NewSessionService(sessRepo, uow)
	sess := testutil.NewTestSession(wi.ID, 20)
	require.NoError(t, svc.LogSession(ctx, sess))

	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemInProgress, updated.Status, "should auto-transition to in_progress after first session")
}

func TestLogSession_TriggersReEstimation(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Study")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))

	// Work item with units tracking: planned 100 min for 10 pages
	wi := testutil.NewTestWorkItem(node.ID, "Read",
		testutil.WithPlannedMin(100),
		testutil.WithLoggedMin(0),
		testutil.WithUnits("pages", 10, 0),
		testutil.WithDurationMode(domain.DurationEstimate),
		testutil.WithSessionBounds(15, 60, 30),
	)
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, uow)

	// Log session: 60 min, completed 3 pages → pace = 20 min/page → implied = 200 min
	// Smooth: round(0.7*100 + 0.3*200) = round(70+60) = 130
	sess := testutil.NewTestSession(wi.ID, 60, testutil.WithUnitsDelta(3))
	require.NoError(t, svc.LogSession(ctx, sess))

	updated, err := wiRepo.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, 130, updated.PlannedMin, "should apply smooth re-estimation: round(0.7*100 + 0.3*200)")
	assert.Equal(t, 60, updated.LoggedMin)
	assert.Equal(t, 3, updated.UnitsDone)
}

func TestSessionService_ListRecent(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Recent Sessions")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task", testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, uow)
	recent := testutil.NewTestSession(wi.ID, 25, testutil.WithStartedAt(time.Now().UTC().Add(-24*time.Hour)))
	old := testutil.NewTestSession(wi.ID, 25, testutil.WithStartedAt(time.Now().UTC().AddDate(0, 0, -10)))
	require.NoError(t, svc.LogSession(ctx, recent))
	require.NoError(t, svc.LogSession(ctx, old))

	list, err := svc.ListRecent(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, recent.ID, list[0].ID)
}

func TestSessionService_Delete(t *testing.T) {
	projRepo, nodes, wiRepo, _, sessRepo, _, uow := setupRepos(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Delete Session")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodes.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Task", testutil.WithSessionBounds(15, 60, 30))
	require.NoError(t, wiRepo.Create(ctx, wi))

	svc := NewSessionService(sessRepo, uow)
	session := testutil.NewTestSession(wi.ID, 30)
	require.NoError(t, svc.LogSession(ctx, session))

	require.NoError(t, svc.Delete(ctx, session.ID))
	_, err := sessRepo.GetByID(ctx, session.ID)
	require.Error(t, err)
}

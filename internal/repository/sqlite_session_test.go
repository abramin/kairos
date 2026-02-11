package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionTestSetup creates project/node/work-item scaffolding needed by session tests.
func sessionTestSetup(t *testing.T) (*SQLiteSessionRepo, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)

	proj := testutil.NewTestProject("SessProj")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node1")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi := testutil.NewTestWorkItem(node.ID, "Task1")
	require.NoError(t, wiRepo.Create(ctx, wi))

	return sessRepo, wi.ID
}

func TestSessionRepo_CreateAndGetByID(t *testing.T) {
	repo, wiID := sessionTestSetup(t)
	ctx := context.Background()

	sess := testutil.NewTestSession(wiID, 45, testutil.WithNote("Good session"))
	require.NoError(t, repo.Create(ctx, sess))

	fetched, err := repo.GetByID(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, fetched.ID)
	assert.Equal(t, wiID, fetched.WorkItemID)
	assert.Equal(t, 45, fetched.Minutes)
	assert.Equal(t, "Good session", fetched.Note)
}

func TestSessionRepo_GetByID_NotFound(t *testing.T) {
	repo, _ := sessionTestSetup(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSessionRepo_ListByWorkItem(t *testing.T) {
	repo, wiID := sessionTestSetup(t)
	ctx := context.Background()

	s1 := testutil.NewTestSession(wiID, 30, testutil.WithStartedAt(time.Now().Add(-2*time.Hour)))
	s2 := testutil.NewTestSession(wiID, 45, testutil.WithStartedAt(time.Now().Add(-1*time.Hour)))
	require.NoError(t, repo.Create(ctx, s1))
	require.NoError(t, repo.Create(ctx, s2))

	list, err := repo.ListByWorkItem(ctx, wiID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	// Should be ordered by started_at.
	assert.Equal(t, s1.ID, list[0].ID)
	assert.Equal(t, s2.ID, list[1].ID)
}

func TestSessionRepo_ListRecent(t *testing.T) {
	repo, wiID := sessionTestSetup(t)
	ctx := context.Background()

	recent := testutil.NewTestSession(wiID, 30, testutil.WithStartedAt(time.Now().UTC()))
	old := testutil.NewTestSession(wiID, 60, testutil.WithStartedAt(time.Now().UTC().AddDate(0, 0, -10)))
	require.NoError(t, repo.Create(ctx, recent))
	require.NoError(t, repo.Create(ctx, old))

	list, err := repo.ListRecent(ctx, 7)
	require.NoError(t, err)
	assert.Len(t, list, 1, "only the recent session should be returned")
	assert.Equal(t, recent.ID, list[0].ID)
}

func TestSessionRepo_Delete(t *testing.T) {
	repo, wiID := sessionTestSetup(t)
	ctx := context.Background()

	sess := testutil.NewTestSession(wiID, 30)
	require.NoError(t, repo.Create(ctx, sess))

	require.NoError(t, repo.Delete(ctx, sess.ID))

	_, err := repo.GetByID(ctx, sess.ID)
	assert.Error(t, err)
}

func TestSessionRepo_UnitsDelta(t *testing.T) {
	repo, wiID := sessionTestSetup(t)
	ctx := context.Background()

	sess := testutil.NewTestSession(wiID, 30, testutil.WithUnitsDelta(5))
	require.NoError(t, repo.Create(ctx, sess))

	fetched, err := repo.GetByID(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, fetched.UnitsDoneDelta)
}

func TestSessionRepo_ListRecentByProject(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	sessRepo := NewSQLiteSessionRepo(db)

	// Create two projects with work items and sessions.
	projA := testutil.NewTestProject("ProjectA")
	require.NoError(t, projRepo.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "NodeA")
	require.NoError(t, nodeRepo.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "TaskA")
	require.NoError(t, wiRepo.Create(ctx, wiA))

	projB := testutil.NewTestProject("ProjectB")
	require.NoError(t, projRepo.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "NodeB")
	require.NoError(t, nodeRepo.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "TaskB")
	require.NoError(t, wiRepo.Create(ctx, wiB))

	sessA := testutil.NewTestSession(wiA.ID, 30, testutil.WithStartedAt(time.Now().UTC()))
	sessB := testutil.NewTestSession(wiB.ID, 45, testutil.WithStartedAt(time.Now().UTC()))
	require.NoError(t, sessRepo.Create(ctx, sessA))
	require.NoError(t, sessRepo.Create(ctx, sessB))

	// Filter by project A.
	listA, err := sessRepo.ListRecentByProject(ctx, projA.ID, 7)
	require.NoError(t, err)
	assert.Len(t, listA, 1)
	assert.Equal(t, sessA.ID, listA[0].ID)

	// Filter by project B.
	listB, err := sessRepo.ListRecentByProject(ctx, projB.ID, 7)
	require.NoError(t, err)
	assert.Len(t, listB, 1)
	assert.Equal(t, sessB.ID, listB[0].ID)
}

package repository_test

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workItemRefTestSetup(t *testing.T) (*repository.SQLiteWorkItemRefRepo, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteWorkItemRefRepo(db)

	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)

	proj := testutil.NewTestProject("WI Ref Test")
	require.NoError(t, projRepo.Create(context.Background(), proj))
	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodeRepo.Create(context.Background(), node))
	wi := testutil.NewTestWorkItem(node.ID, "Task 1")
	require.NoError(t, wiRepo.Create(context.Background(), wi))

	return repo, proj.ID, wi.ID
}

func TestWorkItemRefRepo_SetAndGet(t *testing.T) {
	repo, projectID, wiID := workItemRefTestSetup(t)
	ctx := context.Background()

	err := repo.Set(ctx, wiID, projectID, "w1")
	require.NoError(t, err)

	got, err := repo.GetByProjectAndRef(ctx, projectID, "w1")
	require.NoError(t, err)
	assert.Equal(t, wiID, got)
}

func TestWorkItemRefRepo_GetByProjectAndRef_NotFound(t *testing.T) {
	repo, projectID, _ := workItemRefTestSetup(t)
	ctx := context.Background()

	got, err := repo.GetByProjectAndRef(ctx, projectID, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, got, "missing ref should return empty string, not error")
}

func TestWorkItemRefRepo_Set_Upsert(t *testing.T) {
	repo, projectID, wiID := workItemRefTestSetup(t)
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, wiID, projectID, "ref-v1"))
	// Upsert with a new ref value
	require.NoError(t, repo.Set(ctx, wiID, projectID, "ref-v2"))

	got, err := repo.GetByProjectAndRef(ctx, projectID, "ref-v2")
	require.NoError(t, err)
	assert.Equal(t, wiID, got)

	old, err := repo.GetByProjectAndRef(ctx, projectID, "ref-v1")
	require.NoError(t, err)
	assert.Empty(t, old)
}

func TestWorkItemRefRepo_DeleteByWorkItemID(t *testing.T) {
	repo, projectID, wiID := workItemRefTestSetup(t)
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, wiID, projectID, "w1"))

	got, err := repo.GetByProjectAndRef(ctx, projectID, "w1")
	require.NoError(t, err)
	require.Equal(t, wiID, got)

	require.NoError(t, repo.DeleteByWorkItemID(ctx, wiID))

	after, err := repo.GetByProjectAndRef(ctx, projectID, "w1")
	require.NoError(t, err)
	assert.Empty(t, after)
}

func TestWorkItemRefRepo_IsolatedByProject(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteWorkItemRefRepo(db)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	ctx := context.Background()

	proj1 := testutil.NewTestProject("Project 1", testutil.WithShortID("WP1RF1"))
	proj2 := testutil.NewTestProject("Project 2", testutil.WithShortID("WP2RF1"))
	require.NoError(t, projRepo.Create(ctx, proj1))
	require.NoError(t, projRepo.Create(ctx, proj2))

	node1 := testutil.NewTestNode(proj1.ID, "Node P1")
	node2 := testutil.NewTestNode(proj2.ID, "Node P2")
	require.NoError(t, nodeRepo.Create(ctx, node1))
	require.NoError(t, nodeRepo.Create(ctx, node2))

	wi1 := testutil.NewTestWorkItem(node1.ID, "Task P1")
	wi2 := testutil.NewTestWorkItem(node2.ID, "Task P2")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	// Same ref string "w1" in both projects
	require.NoError(t, repo.Set(ctx, wi1.ID, proj1.ID, "w1"))
	require.NoError(t, repo.Set(ctx, wi2.ID, proj2.ID, "w1"))

	got1, err := repo.GetByProjectAndRef(ctx, proj1.ID, "w1")
	require.NoError(t, err)
	assert.Equal(t, wi1.ID, got1)

	got2, err := repo.GetByProjectAndRef(ctx, proj2.ID, "w1")
	require.NoError(t, err)
	assert.Equal(t, wi2.ID, got2)
}

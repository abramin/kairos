package repository_test

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nodeRefTestSetup(t *testing.T) (*repository.SQLiteNodeRefRepo, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteNodeRefRepo(db)

	// Create a real project and node to satisfy FK constraints.
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)

	proj := testutil.NewTestProject("Ref Test Project")
	require.NoError(t, projRepo.Create(context.Background(), proj))

	node := testutil.NewTestNode(proj.ID, "Chapter 1")
	require.NoError(t, nodeRepo.Create(context.Background(), node))

	return repo, proj.ID, node.ID
}

func TestNodeRefRepo_SetAndGet(t *testing.T) {
	repo, projectID, nodeID := nodeRefTestSetup(t)
	ctx := context.Background()

	err := repo.Set(ctx, nodeID, projectID, "n1")
	require.NoError(t, err)

	got, err := repo.GetByProjectAndRef(ctx, projectID, "n1")
	require.NoError(t, err)
	assert.Equal(t, nodeID, got)
}

func TestNodeRefRepo_GetByProjectAndRef_NotFound(t *testing.T) {
	repo, projectID, _ := nodeRefTestSetup(t)
	ctx := context.Background()

	got, err := repo.GetByProjectAndRef(ctx, projectID, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, got, "missing ref should return empty string, not error")
}

func TestNodeRefRepo_Set_Upsert(t *testing.T) {
	repo, projectID, nodeID := nodeRefTestSetup(t)
	ctx := context.Background()

	// Set initial ref
	require.NoError(t, repo.Set(ctx, nodeID, projectID, "ref-v1"))
	// Upsert: same node, new ref
	require.NoError(t, repo.Set(ctx, nodeID, projectID, "ref-v2"))

	got, err := repo.GetByProjectAndRef(ctx, projectID, "ref-v2")
	require.NoError(t, err)
	assert.Equal(t, nodeID, got)

	// Old ref should not resolve
	old, err := repo.GetByProjectAndRef(ctx, projectID, "ref-v1")
	require.NoError(t, err)
	assert.Empty(t, old)
}

func TestNodeRefRepo_DeleteByNodeID(t *testing.T) {
	repo, projectID, nodeID := nodeRefTestSetup(t)
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, nodeID, projectID, "n1"))

	got, err := repo.GetByProjectAndRef(ctx, projectID, "n1")
	require.NoError(t, err)
	require.Equal(t, nodeID, got)

	require.NoError(t, repo.DeleteByNodeID(ctx, nodeID))

	after, err := repo.GetByProjectAndRef(ctx, projectID, "n1")
	require.NoError(t, err)
	assert.Empty(t, after)
}

func TestNodeRefRepo_IsolatedByProject(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteNodeRefRepo(db)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	ctx := context.Background()

	// Two projects, each with their own node
	proj1 := testutil.NewTestProject("Project 1", testutil.WithShortID("P1REF1"))
	proj2 := testutil.NewTestProject("Project 2", testutil.WithShortID("P2REF1"))
	require.NoError(t, projRepo.Create(ctx, proj1))
	require.NoError(t, projRepo.Create(ctx, proj2))

	node1 := testutil.NewTestNode(proj1.ID, "Node P1")
	node2 := testutil.NewTestNode(proj2.ID, "Node P2")
	require.NoError(t, nodeRepo.Create(ctx, node1))
	require.NoError(t, nodeRepo.Create(ctx, node2))

	// Both use the same ref string "n1"
	require.NoError(t, repo.Set(ctx, node1.ID, proj1.ID, "n1"))
	require.NoError(t, repo.Set(ctx, node2.ID, proj2.ID, "n1"))

	got1, err := repo.GetByProjectAndRef(ctx, proj1.ID, "n1")
	require.NoError(t, err)
	assert.Equal(t, node1.ID, got1)

	got2, err := repo.GetByProjectAndRef(ctx, proj2.ID, "n1")
	require.NoError(t, err)
	assert.Equal(t, node2.ID, got2)
}

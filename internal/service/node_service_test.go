package service

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNodeService(t *testing.T) (NodeService, repository.ProjectRepo, repository.PlanNodeRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	return NewNodeService(nodeRepo), projRepo, nodeRepo
}

func TestNodeService_Create(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1", testutil.WithNodeKind(domain.NodeWeek))
	node.ID = "" // let service assign ID
	require.NoError(t, svc.Create(ctx, node))

	assert.NotEmpty(t, node.ID, "service should assign UUID")
	assert.False(t, node.CreatedAt.IsZero())
}

func TestNodeService_GetByID(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc2")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 2")
	require.NoError(t, svc.Create(ctx, node))

	fetched, err := svc.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "Week 2", fetched.Title)
}

func TestNodeService_ListByProject(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc3")
	require.NoError(t, projRepo.Create(ctx, proj))

	n1 := testutil.NewTestNode(proj.ID, "Node A")
	n2 := testutil.NewTestNode(proj.ID, "Node B")
	require.NoError(t, svc.Create(ctx, n1))
	require.NoError(t, svc.Create(ctx, n2))

	list, err := svc.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestNodeService_ListRoots(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc4")
	require.NoError(t, projRepo.Create(ctx, proj))

	root := testutil.NewTestNode(proj.ID, "Root")
	require.NoError(t, svc.Create(ctx, root))

	child := testutil.NewTestNode(proj.ID, "Child", testutil.WithParentID(root.ID))
	require.NoError(t, svc.Create(ctx, child))

	roots, err := svc.ListRoots(ctx, proj.ID)
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, root.ID, roots[0].ID)
}

func TestNodeService_ListChildren(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc5")
	require.NoError(t, projRepo.Create(ctx, proj))

	parent := testutil.NewTestNode(proj.ID, "Parent")
	require.NoError(t, svc.Create(ctx, parent))

	c1 := testutil.NewTestNode(proj.ID, "Child1", testutil.WithParentID(parent.ID))
	c2 := testutil.NewTestNode(proj.ID, "Child2", testutil.WithParentID(parent.ID))
	require.NoError(t, svc.Create(ctx, c1))
	require.NoError(t, svc.Create(ctx, c2))

	children, err := svc.ListChildren(ctx, parent.ID)
	require.NoError(t, err)
	assert.Len(t, children, 2)
}

func TestNodeService_Update(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc6")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "OrigTitle")
	require.NoError(t, svc.Create(ctx, node))

	node.Title = "NewTitle"
	require.NoError(t, svc.Update(ctx, node))

	fetched, err := svc.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "NewTitle", fetched.Title)
}

func TestNodeService_Delete(t *testing.T) {
	svc, projRepo, _ := setupNodeService(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("NodeSvc7")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "ToDelete")
	require.NoError(t, svc.Create(ctx, node))

	require.NoError(t, svc.Delete(ctx, node.ID))
	_, err := svc.GetByID(ctx, node.ID)
	assert.Error(t, err)
}

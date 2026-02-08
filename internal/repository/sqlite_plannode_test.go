package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlanNodeRepo(t *testing.T) (*SQLitePlanNodeRepo, *SQLiteProjectRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	return NewSQLitePlanNodeRepo(db), NewSQLiteProjectRepo(db)
}

func TestPlanNodeRepo_CreateAndGetByID(t *testing.T) {
	repo, projRepo := setupPlanNodeRepo(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("PlanNode Host")
	require.NoError(t, projRepo.Create(ctx, proj))

	parent := testutil.NewTestNode(proj.ID, "Parent", testutil.WithNodeKind(domain.NodeModule))
	require.NoError(t, repo.Create(ctx, parent))

	parentID := parent.ID
	due := time.Now().UTC().AddDate(0, 0, 7)
	notBefore := time.Now().UTC().AddDate(0, 0, 1)
	notAfter := time.Now().UTC().AddDate(0, 0, 10)
	budget := 180
	node := testutil.NewTestNode(proj.ID, "Week 1",
		testutil.WithNodeKind(domain.NodeWeek),
		testutil.WithOrderIndex(3),
		testutil.WithParentID(parentID),
		testutil.WithNodeDueDate(due),
		testutil.WithPlannedMinBudget(budget),
	)
	node.NotBefore = &notBefore
	node.NotAfter = &notAfter

	require.NoError(t, repo.Create(ctx, node))

	got, err := repo.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, node.ID, got.ID)
	assert.Equal(t, proj.ID, got.ProjectID)
	require.NotNil(t, got.ParentID)
	assert.Equal(t, parentID, *got.ParentID)
	assert.Equal(t, domain.NodeWeek, got.Kind)
	assert.Equal(t, 3, got.OrderIndex)
	require.NotNil(t, got.DueDate)
	assert.Equal(t, due.Format("2006-01-02"), got.DueDate.Format("2006-01-02"))
	require.NotNil(t, got.NotBefore)
	assert.Equal(t, notBefore.Format("2006-01-02"), got.NotBefore.Format("2006-01-02"))
	require.NotNil(t, got.NotAfter)
	assert.Equal(t, notAfter.Format("2006-01-02"), got.NotAfter.Format("2006-01-02"))
	require.NotNil(t, got.PlannedMinBudget)
	assert.Equal(t, 180, *got.PlannedMinBudget)
}

func TestPlanNodeRepo_GetByID_NotFound(t *testing.T) {
	repo, _ := setupPlanNodeRepo(t)
	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPlanNodeRepo_ListMethods_OrderAndHierarchy(t *testing.T) {
	repo, projRepo := setupPlanNodeRepo(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Hierarchy")
	require.NoError(t, projRepo.Create(ctx, proj))

	root2 := testutil.NewTestNode(proj.ID, "Root 2", testutil.WithOrderIndex(2))
	root1 := testutil.NewTestNode(proj.ID, "Root 1", testutil.WithOrderIndex(1))
	require.NoError(t, repo.Create(ctx, root2))
	require.NoError(t, repo.Create(ctx, root1))

	childB := testutil.NewTestNode(proj.ID, "Child B",
		testutil.WithParentID(root1.ID),
		testutil.WithOrderIndex(2),
	)
	childA := testutil.NewTestNode(proj.ID, "Child A",
		testutil.WithParentID(root1.ID),
		testutil.WithOrderIndex(1),
	)
	require.NoError(t, repo.Create(ctx, childB))
	require.NoError(t, repo.Create(ctx, childA))

	byProject, err := repo.ListByProject(ctx, proj.ID)
	require.NoError(t, err)
	require.Len(t, byProject, 4)
	assert.Equal(t, "Root 1", byProject[0].Title)
	assert.Equal(t, "Child A", byProject[1].Title)

	roots, err := repo.ListRoots(ctx, proj.ID)
	require.NoError(t, err)
	require.Len(t, roots, 2)
	assert.Equal(t, "Root 1", roots[0].Title)
	assert.Equal(t, "Root 2", roots[1].Title)

	children, err := repo.ListChildren(ctx, root1.ID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.Equal(t, "Child A", children[0].Title)
	assert.Equal(t, "Child B", children[1].Title)
}

func TestPlanNodeRepo_UpdateAndDelete(t *testing.T) {
	repo, projRepo := setupPlanNodeRepo(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("UpdateDelete")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Original", testutil.WithNodeKind(domain.NodeGeneric))
	require.NoError(t, repo.Create(ctx, node))

	budget := 240
	node.Title = "Updated Title"
	node.Kind = domain.NodeModule
	node.OrderIndex = 9
	node.PlannedMinBudget = &budget
	node.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, node))

	updated, err := repo.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, domain.NodeModule, updated.Kind)
	assert.Equal(t, 9, updated.OrderIndex)
	require.NotNil(t, updated.PlannedMinBudget)
	assert.Equal(t, 240, *updated.PlannedMinBudget)

	require.NoError(t, repo.Delete(ctx, node.ID))
	_, err = repo.GetByID(ctx, node.ID)
	require.Error(t, err)
}

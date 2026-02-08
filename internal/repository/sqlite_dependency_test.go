package repository

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// depTestSetup creates a project, node, and two work items for dependency tests.
func depTestSetup(t *testing.T) (*SQLiteDependencyRepo, *SQLiteWorkItemRepo, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("DepTest")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node1")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Predecessor")
	require.NoError(t, wiRepo.Create(ctx, wi1))

	wi2 := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, wiRepo.Create(ctx, wi2))

	return depRepo, wiRepo, wi1.ID, wi2.ID
}

func TestDependencyRepo_CreateAndList(t *testing.T) {
	depRepo, _, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	// ListPredecessors of wi2 should return wi1
	preds, err := depRepo.ListPredecessors(ctx, wi2ID)
	require.NoError(t, err)
	require.Len(t, preds, 1)
	assert.Equal(t, wi1ID, preds[0].PredecessorWorkItemID)
	assert.Equal(t, wi2ID, preds[0].SuccessorWorkItemID)

	// ListSuccessors of wi1 should return wi2
	succs, err := depRepo.ListSuccessors(ctx, wi1ID)
	require.NoError(t, err)
	require.Len(t, succs, 1)
	assert.Equal(t, wi2ID, succs[0].SuccessorWorkItemID)
}

func TestDependencyRepo_Delete(t *testing.T) {
	depRepo, _, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	require.NoError(t, depRepo.Delete(ctx, wi1ID, wi2ID))

	preds, err := depRepo.ListPredecessors(ctx, wi2ID)
	require.NoError(t, err)
	assert.Empty(t, preds)
}

func TestDependencyRepo_HasUnfinishedPredecessors_True(t *testing.T) {
	depRepo, _, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	// wi1 is still 'todo', so wi2 has unfinished predecessors.
	has, err := depRepo.HasUnfinishedPredecessors(ctx, wi2ID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestDependencyRepo_HasUnfinishedPredecessors_FalseWhenDone(t *testing.T) {
	depRepo, wiRepo, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	// Mark predecessor as done.
	wi1, err := wiRepo.GetByID(ctx, wi1ID)
	require.NoError(t, err)
	wi1.Status = domain.WorkItemDone
	require.NoError(t, wiRepo.Update(ctx, wi1))

	has, err := depRepo.HasUnfinishedPredecessors(ctx, wi2ID)
	require.NoError(t, err)
	assert.False(t, has, "predecessor is done, so no unfinished predecessors")
}

func TestDependencyRepo_HasUnfinishedPredecessors_FalseWhenSkipped(t *testing.T) {
	depRepo, wiRepo, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	wi1, err := wiRepo.GetByID(ctx, wi1ID)
	require.NoError(t, err)
	wi1.Status = domain.WorkItemSkipped
	require.NoError(t, wiRepo.Update(ctx, wi1))

	has, err := depRepo.HasUnfinishedPredecessors(ctx, wi2ID)
	require.NoError(t, err)
	assert.False(t, has, "skipped predecessor counts as finished")
}

func TestDependencyRepo_NoPredecessors(t *testing.T) {
	depRepo, _, wi1ID, _ := depTestSetup(t)
	ctx := context.Background()

	// No dependencies created — wi1 has no predecessors.
	preds, err := depRepo.ListPredecessors(ctx, wi1ID)
	require.NoError(t, err)
	assert.Empty(t, preds)

	has, err := depRepo.HasUnfinishedPredecessors(ctx, wi1ID)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestDependencyRepo_MultiplePredecessors(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("MultiDep")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Pred1")
	wi2 := testutil.NewTestWorkItem(node.ID, "Pred2")
	wi3 := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))
	require.NoError(t, wiRepo.Create(ctx, wi3))

	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi3.ID}))
	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi2.ID, SuccessorWorkItemID: wi3.ID}))

	preds, err := depRepo.ListPredecessors(ctx, wi3.ID)
	require.NoError(t, err)
	assert.Len(t, preds, 2)

	// Still has unfinished even if one pred is done.
	fetched1, err := wiRepo.GetByID(ctx, wi1.ID)
	require.NoError(t, err)
	fetched1.Status = domain.WorkItemDone
	require.NoError(t, wiRepo.Update(ctx, fetched1))

	has, err := depRepo.HasUnfinishedPredecessors(ctx, wi3.ID)
	require.NoError(t, err)
	assert.True(t, has, "one predecessor still unfinished")

	// Mark second pred as done too.
	fetched2, err := wiRepo.GetByID(ctx, wi2.ID)
	require.NoError(t, err)
	fetched2.Status = domain.WorkItemDone
	require.NoError(t, wiRepo.Update(ctx, fetched2))

	has, err = depRepo.HasUnfinishedPredecessors(ctx, wi3.ID)
	require.NoError(t, err)
	assert.False(t, has, "all predecessors are done")
}

func TestDependencyRepo_DuplicateCreateFails(t *testing.T) {
	depRepo, _, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	dep := &domain.Dependency{PredecessorWorkItemID: wi1ID, SuccessorWorkItemID: wi2ID}
	require.NoError(t, depRepo.Create(ctx, dep))

	err := depRepo.Create(ctx, dep)
	assert.Error(t, err, "duplicate dependency should fail due to PRIMARY KEY constraint")
}

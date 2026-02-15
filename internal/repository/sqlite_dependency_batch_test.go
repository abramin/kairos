package repository

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBlockedWorkItemIDs_EmptyInput(t *testing.T) {
	depRepo, _, _, _ := depTestSetup(t)
	ctx := context.Background()

	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, blocked)
}

func TestListBlockedWorkItemIDs_NoDeps(t *testing.T) {
	depRepo, _, wi1ID, wi2ID := depTestSetup(t)
	ctx := context.Background()

	// No dependencies created — neither item should be blocked.
	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{wi1ID, wi2ID})
	require.NoError(t, err)
	assert.Empty(t, blocked)
}

func TestListBlockedWorkItemIDs_SomeBlocked(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("BatchTest")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Predecessor")
	wi2 := testutil.NewTestWorkItem(node.ID, "BlockedSuccessor")
	wi3 := testutil.NewTestWorkItem(node.ID, "FreeItem")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))
	require.NoError(t, wiRepo.Create(ctx, wi3))

	// wi1 -> wi2 (wi1 is todo, so wi2 is blocked)
	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: wi1.ID,
		SuccessorWorkItemID:  wi2.ID,
	}))

	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{wi1.ID, wi2.ID, wi3.ID})
	require.NoError(t, err)
	assert.True(t, blocked[wi2.ID], "wi2 should be blocked (has unfinished predecessor)")
	assert.False(t, blocked[wi1.ID], "wi1 has no predecessors")
	assert.False(t, blocked[wi3.ID], "wi3 has no predecessors")
}

func TestListBlockedWorkItemIDs_MultiplePredecessors(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("MultiPred")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Pred1")
	wi2 := testutil.NewTestWorkItem(node.ID, "Pred2")
	wi3 := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))
	require.NoError(t, wiRepo.Create(ctx, wi3))

	// wi1 -> wi3, wi2 -> wi3. wi1 is done, wi2 is todo.
	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi3.ID}))
	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi2.ID, SuccessorWorkItemID: wi3.ID}))

	wi1.Status = domain.WorkItemDone
	require.NoError(t, wiRepo.Update(ctx, wi1))

	// wi3 should still be blocked because wi2 is not done.
	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{wi3.ID})
	require.NoError(t, err)
	assert.True(t, blocked[wi3.ID], "wi3 blocked: one predecessor still unfinished")
}

func TestListBlockedWorkItemIDs_AllPredecessorsDone(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("AllDone")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Pred")
	wi2 := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi2.ID}))

	wi1.Status = domain.WorkItemDone
	require.NoError(t, wiRepo.Update(ctx, wi1))

	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{wi2.ID})
	require.NoError(t, err)
	assert.Empty(t, blocked, "all predecessors done — not blocked")
}

func TestListBlockedWorkItemIDs_SkippedCountsAsFinished(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := NewSQLiteProjectRepo(db)
	nodeRepo := NewSQLitePlanNodeRepo(db)
	wiRepo := NewSQLiteWorkItemRepo(db)
	depRepo := NewSQLiteDependencyRepo(db)

	proj := testutil.NewTestProject("Skipped")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	wi1 := testutil.NewTestWorkItem(node.ID, "Pred")
	wi2 := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, wiRepo.Create(ctx, wi1))
	require.NoError(t, wiRepo.Create(ctx, wi2))

	require.NoError(t, depRepo.Create(ctx, &domain.Dependency{PredecessorWorkItemID: wi1.ID, SuccessorWorkItemID: wi2.ID}))

	wi1.Status = domain.WorkItemSkipped
	require.NoError(t, wiRepo.Update(ctx, wi1))

	blocked, err := depRepo.ListBlockedWorkItemIDs(ctx, []string{wi2.ID})
	require.NoError(t, err)
	assert.Empty(t, blocked, "skipped predecessor counts as finished")
}

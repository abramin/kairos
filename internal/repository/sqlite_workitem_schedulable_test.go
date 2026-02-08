package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSchedulableRepos(t *testing.T) (*sql.DB, *SQLiteProjectRepo, *SQLitePlanNodeRepo, *SQLiteWorkItemRepo, *SQLiteDependencyRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	return db,
		NewSQLiteProjectRepo(db),
		NewSQLitePlanNodeRepo(db),
		NewSQLiteWorkItemRepo(db),
		NewSQLiteDependencyRepo(db)
}

func setupSchedulableNode(t *testing.T, projects *SQLiteProjectRepo, nodes *SQLitePlanNodeRepo) (context.Context, *domain.Project, *domain.PlanNode) {
	t.Helper()
	ctx := context.Background()
	proj := testutil.NewTestProject("Schedulable Project")
	require.NoError(t, projects.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodes.Create(ctx, node))
	return ctx, proj, node
}

func candidateIDs(candidates []SchedulableCandidate) map[string]bool {
	ids := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		ids[c.WorkItem.ID] = true
	}
	return ids
}

func TestWorkItemRepo_ListSchedulable_ExcludesDoneAndArchivedByDefault(t *testing.T) {
	_, projects, nodes, workItems, _ := setupSchedulableRepos(t)
	ctx, _, node := setupSchedulableNode(t, projects, nodes)

	todo := testutil.NewTestWorkItem(node.ID, "Todo", testutil.WithWorkItemStatus(domain.WorkItemTodo))
	inProgress := testutil.NewTestWorkItem(node.ID, "In Progress", testutil.WithWorkItemStatus(domain.WorkItemInProgress))
	done := testutil.NewTestWorkItem(node.ID, "Done", testutil.WithWorkItemStatus(domain.WorkItemDone))
	archived := testutil.NewTestWorkItem(node.ID, "Archived")

	require.NoError(t, workItems.Create(ctx, todo))
	require.NoError(t, workItems.Create(ctx, inProgress))
	require.NoError(t, workItems.Create(ctx, done))
	require.NoError(t, workItems.Create(ctx, archived))
	require.NoError(t, workItems.Archive(ctx, archived.ID))

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	ids := candidateIDs(candidates)

	assert.True(t, ids[todo.ID], "todo item should be schedulable")
	assert.True(t, ids[inProgress.ID], "in_progress item should be schedulable")
	assert.False(t, ids[done.ID], "done item should not be schedulable")
	assert.False(t, ids[archived.ID], "archived item should not be schedulable")
}

func TestWorkItemRepo_ListSchedulable_IncludeArchivedFlagRespectsArchivedAtFilter(t *testing.T) {
	db, projects, nodes, workItems, _ := setupSchedulableRepos(t)
	ctx, _, node := setupSchedulableNode(t, projects, nodes)

	item := testutil.NewTestWorkItem(node.ID, "Soft Archived", testutil.WithWorkItemStatus(domain.WorkItemTodo))
	require.NoError(t, workItems.Create(ctx, item))

	_, err := db.ExecContext(ctx, `UPDATE work_items SET archived_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), item.ID)
	require.NoError(t, err)

	withoutArchived, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.False(t, candidateIDs(withoutArchived)[item.ID], "archived_at rows are excluded when includeArchived=false")

	withArchived, err := workItems.ListSchedulable(ctx, true)
	require.NoError(t, err)
	assert.True(t, candidateIDs(withArchived)[item.ID], "archived_at rows are included when includeArchived=true")
}

func TestWorkItemRepo_ListSchedulable_ArchivedProjectsExcludedEvenWhenIncludeArchived(t *testing.T) {
	_, projects, nodes, workItems, _ := setupSchedulableRepos(t)
	ctx, proj, node := setupSchedulableNode(t, projects, nodes)

	item := testutil.NewTestWorkItem(node.ID, "Project Archived Candidate")
	require.NoError(t, workItems.Create(ctx, item))
	require.NoError(t, projects.Archive(ctx, proj.ID))

	candidatesDefault, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	assert.False(t, candidateIDs(candidatesDefault)[item.ID])

	candidatesWithArchived, err := workItems.ListSchedulable(ctx, true)
	require.NoError(t, err)
	assert.False(t, candidateIDs(candidatesWithArchived)[item.ID])
}

func TestWorkItemRepo_ListSchedulable_IncludesZeroPlannedMinItems(t *testing.T) {
	_, projects, nodes, workItems, _ := setupSchedulableRepos(t)
	ctx, _, node := setupSchedulableNode(t, projects, nodes)

	zeroPlanned := testutil.NewTestWorkItem(node.ID, "Zero Planned", testutil.WithPlannedMin(0))
	require.NoError(t, workItems.Create(ctx, zeroPlanned))

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	ids := candidateIDs(candidates)
	assert.True(t, ids[zeroPlanned.ID], "zero-planned work item should still be returned by repo")

	for _, c := range candidates {
		if c.WorkItem.ID == zeroPlanned.ID {
			assert.Equal(t, 0, c.WorkItem.PlannedMin)
		}
	}
}

func TestWorkItemRepo_ListSchedulable_DependencyBlockedItemsAreStillReturned(t *testing.T) {
	_, projects, nodes, workItems, deps := setupSchedulableRepos(t)
	ctx, _, node := setupSchedulableNode(t, projects, nodes)

	pred := testutil.NewTestWorkItem(node.ID, "Predecessor")
	succ := testutil.NewTestWorkItem(node.ID, "Successor")
	require.NoError(t, workItems.Create(ctx, pred))
	require.NoError(t, workItems.Create(ctx, succ))
	require.NoError(t, deps.Create(ctx, &domain.Dependency{
		PredecessorWorkItemID: pred.ID,
		SuccessorWorkItemID:   succ.ID,
	}))

	candidates, err := workItems.ListSchedulable(ctx, false)
	require.NoError(t, err)
	ids := candidateIDs(candidates)

	assert.True(t, ids[pred.ID])
	assert.True(t, ids[succ.ID], "dependency filtering is applied at service layer, not repository layer")
}

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

func setupWorkItemService(t *testing.T) (WorkItemService, repository.ProjectRepo, repository.PlanNodeRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	return NewWorkItemService(wiRepo), projRepo, nodeRepo
}

func setupWorkItemWithProject(t *testing.T, svc WorkItemService, projRepo repository.ProjectRepo, nodeRepo repository.PlanNodeRepo) (string, string) {
	t.Helper()
	ctx := context.Background()

	proj := testutil.NewTestProject("WISvc")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Node")
	require.NoError(t, nodeRepo.Create(ctx, node))

	return proj.ID, node.ID
}

func TestWorkItemService_Create(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "Read Chapter 1", testutil.WithPlannedMin(60))
	wi.ID = "" // let service assign ID
	require.NoError(t, svc.Create(ctx, wi))

	assert.NotEmpty(t, wi.ID, "service should assign UUID")
	assert.Equal(t, domain.WorkItemTodo, wi.Status)
}

func TestWorkItemService_Create_DefaultStatus(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "Task")
	wi.Status = "" // blank — service should default to 'todo'
	require.NoError(t, svc.Create(ctx, wi))

	fetched, err := svc.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemTodo, fetched.Status)
}

func TestWorkItemService_GetByID(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "Exercises", testutil.WithPlannedMin(90))
	require.NoError(t, svc.Create(ctx, wi))

	fetched, err := svc.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, "Exercises", fetched.Title)
	assert.Equal(t, 90, fetched.PlannedMin)
}

func TestWorkItemService_ListByNode(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi1 := testutil.NewTestWorkItem(nodeID, "Task1")
	wi2 := testutil.NewTestWorkItem(nodeID, "Task2")
	require.NoError(t, svc.Create(ctx, wi1))
	require.NoError(t, svc.Create(ctx, wi2))

	list, err := svc.ListByNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestWorkItemService_ListByProject(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	projID, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "ProjTask")
	require.NoError(t, svc.Create(ctx, wi))

	list, err := svc.ListByProject(ctx, projID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "ProjTask", list[0].Title)
}

func TestWorkItemService_Update(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "OrigTask")
	require.NoError(t, svc.Create(ctx, wi))

	wi.Title = "UpdatedTask"
	wi.PlannedMin = 120
	require.NoError(t, svc.Update(ctx, wi))

	fetched, err := svc.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, "UpdatedTask", fetched.Title)
	assert.Equal(t, 120, fetched.PlannedMin)
}

func TestWorkItemService_MarkDone(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "MarkMe")
	require.NoError(t, svc.Create(ctx, wi))

	require.NoError(t, svc.MarkDone(ctx, wi.ID))

	fetched, err := svc.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemDone, fetched.Status)
}

func TestWorkItemService_MarkDone_NonexistentItem(t *testing.T) {
	svc, _, _ := setupWorkItemService(t)
	ctx := context.Background()

	err := svc.MarkDone(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestWorkItemService_Archive(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "ArchiveMe")
	require.NoError(t, svc.Create(ctx, wi))

	require.NoError(t, svc.Archive(ctx, wi.ID))

	fetched, err := svc.GetByID(ctx, wi.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WorkItemArchived, fetched.Status)
}

func TestWorkItemService_Delete(t *testing.T) {
	svc, projRepo, nodeRepo := setupWorkItemService(t)
	_, nodeID := setupWorkItemWithProject(t, svc, projRepo, nodeRepo)
	ctx := context.Background()

	wi := testutil.NewTestWorkItem(nodeID, "DeleteMe")
	require.NoError(t, svc.Create(ctx, wi))

	require.NoError(t, svc.Delete(ctx, wi.ID))

	_, err := svc.GetByID(ctx, wi.ID)
	assert.Error(t, err)
}

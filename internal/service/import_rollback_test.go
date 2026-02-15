package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validImportSchema() *importer.ImportSchema {
	return &importer.ImportSchema{
		Project: importer.ProjectImport{
			Name:      "Rollback Test Project",
			ShortID:   "RBT01",
			Domain:    "testing",
			StartDate: "2026-01-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "mod-a", Title: "Module A", Kind: "module"},
			{Ref: "mod-b", Title: "Module B", Kind: "module"},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "t1", NodeRef: "mod-a", Title: "Task 1", PlannedMin: intPtr(30), Type: "task"},
			{Ref: "t2", NodeRef: "mod-a", Title: "Task 2", PlannedMin: intPtr(45), Type: "task"},
			{Ref: "t3", NodeRef: "mod-b", Title: "Task 3", PlannedMin: intPtr(60), Type: "reading"},
		},
	}
}

func TestImportProject_RollbackOnNodeCreateFailure(t *testing.T) {
	database := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	depRepo := repository.NewSQLiteDependencyRepo(database)
	ctx := context.Background()

	// ExecContext calls in importSchema:
	// #1 = project create, #2 = node "Module A" create, #3 = node "Module B" create
	// Fail on #3 so second node fails after project + first node succeed within tx
	failUoW := &testutil.FailOnNthExecUoW{
		DB:     database,
		FailOn: 3,
		Err:    fmt.Errorf("injected node create failure"),
	}

	svc := NewImportService(projRepo, nodeRepo, wiRepo, depRepo, failUoW)

	schema := validImportSchema()
	_, err := svc.ImportProjectFromSchema(ctx, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected node create failure")

	// Verify nothing was persisted (transaction rolled back)
	projects, err := projRepo.List(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, projects, "no projects should exist after rollback")
}

func TestImportProject_RollbackOnWorkItemCreateFailure(t *testing.T) {
	database := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	depRepo := repository.NewSQLiteDependencyRepo(database)
	ctx := context.Background()

	// Exec calls: #1 = project, #2 = node A, #3 = node B, #4 = work item 1, #5 = work item 2
	// Fail on #5 (second work item)
	failUoW := &testutil.FailOnNthExecUoW{
		DB:     database,
		FailOn: 5,
		Err:    fmt.Errorf("injected work item create failure"),
	}

	svc := NewImportService(projRepo, nodeRepo, wiRepo, depRepo, failUoW)

	schema := validImportSchema()
	_, err := svc.ImportProjectFromSchema(ctx, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected work item create failure")

	// Verify nothing was persisted
	projects, err := projRepo.List(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, projects, "no projects should exist after rollback")
}

func TestImportProject_SuccessPath(t *testing.T) {
	database := testutil.NewTestDB(t)
	uow := testutil.NewTestUoW(database)
	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	depRepo := repository.NewSQLiteDependencyRepo(database)
	ctx := context.Background()

	svc := NewImportService(projRepo, nodeRepo, wiRepo, depRepo, uow)

	schema := validImportSchema()
	result, err := svc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)
	assert.Equal(t, "Rollback Test Project", result.Project.Name)
	assert.Equal(t, 2, result.NodeCount)
	assert.Equal(t, 3, result.WorkItemCount)

	// Verify all entities are queryable
	projects, err := projRepo.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	nodes, err := nodeRepo.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	assert.Len(t, nodes, 2)

	items, err := wiRepo.ListByProject(ctx, result.Project.ID)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

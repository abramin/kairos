package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogSession_RollbackOnSessionCreateFailure(t *testing.T) {
	database := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	ctx := context.Background()

	// Set up test data
	proj := testutil.NewTestProject("Rollback Test")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodeRepo.Create(ctx, node))

	item := testutil.NewTestWorkItem(node.ID, "Read chapter", testutil.WithPlannedMin(60))
	require.NoError(t, wiRepo.Create(ctx, item))

	// Use FailOnNthExecUoW: ExecContext #1 = workItems.Update, #2 = sessions.Create
	// Fail on #2 so the session insert fails after work item is updated within the tx
	failUoW := &testutil.FailOnNthExecUoW{
		DB:     database,
		FailOn: 2,
		Err:    fmt.Errorf("injected session create failure"),
	}

	svc := NewSessionService(sessRepo, wiRepo, failUoW)

	session := testutil.NewTestSession(item.ID, 30)
	err := svc.LogSession(ctx, session)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected session create failure")

	// Verify work item is unchanged (transaction rolled back)
	wi, err := wiRepo.GetByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, wi.LoggedMin, "logged_min should be unchanged after rollback")
	assert.Equal(t, domain.WorkItemTodo, wi.Status, "status should be unchanged after rollback")

	// Verify no session was created
	sessions, err := sessRepo.ListByWorkItem(ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, sessions, "no sessions should exist after rollback")
}

func TestLogSession_RollbackOnWorkItemUpdateFailure(t *testing.T) {
	database := testutil.NewTestDB(t)
	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	ctx := context.Background()

	proj := testutil.NewTestProject("Rollback Test 2")
	require.NoError(t, projRepo.Create(ctx, proj))

	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodeRepo.Create(ctx, node))

	item := testutil.NewTestWorkItem(node.ID, "Read chapter", testutil.WithPlannedMin(60))
	require.NoError(t, wiRepo.Create(ctx, item))

	// Fail on ExecContext #1 (workItems.Update)
	failUoW := &testutil.FailOnNthExecUoW{
		DB:     database,
		FailOn: 1,
		Err:    fmt.Errorf("injected update failure"),
	}

	svc := NewSessionService(sessRepo, wiRepo, failUoW)

	session := testutil.NewTestSession(item.ID, 30)
	err := svc.LogSession(ctx, session)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected update failure")

	// Verify both work item and sessions are unchanged
	wi, err := wiRepo.GetByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, wi.LoggedMin)
	assert.Equal(t, domain.WorkItemTodo, wi.Status)

	sessions, err := sessRepo.ListByWorkItem(ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

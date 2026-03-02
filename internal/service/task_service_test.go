package service_test

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func taskService(t *testing.T) service.TaskService {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteTaskRepo(db)
	return service.NewTaskService(repo)
}

func TestTaskService_Add_CreatesTask(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Buy groceries"})
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Buy groceries", task.Title)
}

func TestTaskService_Add_RequiresTitle(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	_, err := svc.Add(ctx, service.AddTaskRequest{Title: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestTaskService_ListActive_ReturnsTasks(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	_, err := svc.Add(ctx, service.AddTaskRequest{Title: "Task A"})
	require.NoError(t, err)
	_, err = svc.Add(ctx, service.AddTaskRequest{Title: "Task B"})
	require.NoError(t, err)

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
}

func TestTaskService_Update_ChangesTitle(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Old title"})
	require.NoError(t, err)

	err = svc.Update(ctx, task.ID, "New title", "new desc")
	require.NoError(t, err)

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "New title", tasks[0].Title)
	assert.Equal(t, "new desc", tasks[0].Description)
}

func TestTaskService_Update_RequiresTitle(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Valid"})
	require.NoError(t, err)

	err = svc.Update(ctx, task.ID, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestTaskService_MarkDone_RemovesFromActive(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Finish report"})
	require.NoError(t, err)

	require.NoError(t, svc.MarkDone(ctx, task.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestTaskService_Delete_RemovesTask(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Delete me"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, task.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestTaskService_MoveUp_SwapsWithPredecessor(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	t1, err := svc.Add(ctx, service.AddTaskRequest{Title: "First"})
	require.NoError(t, err)
	t2, err := svc.Add(ctx, service.AddTaskRequest{Title: "Second"})
	require.NoError(t, err)

	// Move "Second" up — should become the first item
	require.NoError(t, svc.MoveUp(ctx, t2.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, t2.ID, tasks[0].ID, "Second should now be first")
	assert.Equal(t, t1.ID, tasks[1].ID, "First should now be second")
}

func TestTaskService_MoveUp_AtTopIsNoOp(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	task, err := svc.Add(ctx, service.AddTaskRequest{Title: "Only task"})
	require.NoError(t, err)

	// Moving the only task up is a no-op — should not error
	require.NoError(t, svc.MoveUp(ctx, task.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

func TestTaskService_MoveDown_SwapsWithSuccessor(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	t1, err := svc.Add(ctx, service.AddTaskRequest{Title: "First"})
	require.NoError(t, err)
	t2, err := svc.Add(ctx, service.AddTaskRequest{Title: "Second"})
	require.NoError(t, err)

	// Move "First" down — should become the second item
	require.NoError(t, svc.MoveDown(ctx, t1.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, t2.ID, tasks[0].ID, "Second should now be first")
	assert.Equal(t, t1.ID, tasks[1].ID, "First should now be second")
}

func TestTaskService_MoveDown_AtBottomIsNoOp(t *testing.T) {
	svc := taskService(t)
	ctx := context.Background()

	_, err := svc.Add(ctx, service.AddTaskRequest{Title: "First"})
	require.NoError(t, err)
	t2, err := svc.Add(ctx, service.AddTaskRequest{Title: "Last"})
	require.NoError(t, err)

	// Moving the last task down is a no-op
	require.NoError(t, svc.MoveDown(ctx, t2.ID))

	tasks, err := svc.ListActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, t2.ID, tasks[1].ID, "Last task should still be last")
}

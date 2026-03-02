package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func taskTestRepo(t *testing.T) *repository.SQLiteTaskRepo {
	t.Helper()
	db := testutil.NewTestDB(t)
	return repository.NewSQLiteTaskRepo(db)
}

func newTask(title string) *domain.Task {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Task{
		ID:          "task-" + title,
		Title:       title,
		Description: "desc-" + title,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestTaskRepo_CreateAndGetByID(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	task := newTask("Read book")
	require.NoError(t, repo.Create(ctx, task))

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, "Read book", got.Title)
	assert.Equal(t, "desc-Read book", got.Description)
	assert.Nil(t, got.ArchivedAt)
}

func TestTaskRepo_ListActive_ExcludesArchived(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	active := newTask("Active task")
	archived := newTask("Archived task")
	archived.ID = "task-archived"
	require.NoError(t, repo.Create(ctx, active))
	require.NoError(t, repo.Create(ctx, archived))

	now := time.Now().UTC()
	require.NoError(t, repo.Archive(ctx, archived.ID, now))

	tasks, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, active.ID, tasks[0].ID)
}

func TestTaskRepo_ListActive_OrderByOrderIndex(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	t1 := newTask("First")
	t1.ID = "task-1"
	t2 := newTask("Second")
	t2.ID = "task-2"
	t3 := newTask("Third")
	t3.ID = "task-3"
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))
	require.NoError(t, repo.Create(ctx, t3))

	tasks, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 3)
	assert.Equal(t, "First", tasks[0].Title)
	assert.Equal(t, "Second", tasks[1].Title)
	assert.Equal(t, "Third", tasks[2].Title)
}

func TestTaskRepo_Update(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	task := newTask("Old title")
	require.NoError(t, repo.Create(ctx, task))

	task.Title = "New title"
	task.Description = "Updated description"
	task.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, task))

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "New title", got.Title)
	assert.Equal(t, "Updated description", got.Description)
}

func TestTaskRepo_Archive_SetsArchivedAt(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	task := newTask("To archive")
	require.NoError(t, repo.Create(ctx, task))

	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.Archive(ctx, task.ID, now))

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ArchivedAt)
	assert.Equal(t, now.Unix(), got.ArchivedAt.Unix())
}

func TestTaskRepo_Delete_RemovesRecord(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	task := newTask("To delete")
	require.NoError(t, repo.Create(ctx, task))

	require.NoError(t, repo.Delete(ctx, task.ID))

	_, err := repo.GetByID(ctx, task.ID)
	assert.Error(t, err)
}

func TestTaskRepo_SwapOrder(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	t1 := newTask("Alpha")
	t1.ID = "task-alpha"
	t2 := newTask("Beta")
	t2.ID = "task-beta"
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	tasks, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	orderA := tasks[0].OrderIndex
	orderB := tasks[1].OrderIndex

	require.NoError(t, repo.SwapOrder(ctx, tasks[0].ID, tasks[1].ID))

	got, err := repo.ListActive(ctx)
	require.NoError(t, err)
	// After swap the second task (Beta) should have Alpha's original order
	assert.Equal(t, orderA, got[0].OrderIndex)
	assert.Equal(t, orderB, got[1].OrderIndex)
}

func TestTaskRepo_ListActive_EmptyDB(t *testing.T) {
	repo := taskTestRepo(t)
	ctx := context.Background()

	tasks, err := repo.ListActive(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHabit(title string) *domain.Habit {
	now := time.Now().UTC()
	return &domain.Habit{
		ID:            uuid.New().String(),
		Title:         title,
		CadenceDays:   1,
		TargetMin:     20,
		MinSessionMin: 10,
		MaxSessionMin: 30,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func newTestHabitLog(habitID string, performedAt time.Time, minutes int) *domain.HabitLog {
	now := time.Now().UTC()
	return &domain.HabitLog{
		ID:          uuid.New().String(),
		HabitID:     habitID,
		PerformedAt: performedAt,
		Minutes:     minutes,
		Note:        "test note",
		CreatedAt:   now,
	}
}

func TestHabitRepo_CreateAndGetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("French reading")
	require.NoError(t, repo.Create(ctx, h))

	fetched, err := repo.GetByID(ctx, h.ID)
	require.NoError(t, err)
	assert.Equal(t, h.ID, fetched.ID)
	assert.Equal(t, "French reading", fetched.Title)
	assert.Equal(t, 1, fetched.CadenceDays)
	assert.Equal(t, 20, fetched.TargetMin)
	assert.Equal(t, 10, fetched.MinSessionMin)
	assert.Equal(t, 30, fetched.MaxSessionMin)
	assert.Nil(t, fetched.ArchivedAt)
	assert.True(t, fetched.IsActive())
}

func TestHabitRepo_ListActive_ExcludesArchived(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	active := newTestHabit("Active Habit")
	require.NoError(t, repo.Create(ctx, active))

	archived := newTestHabit("Archived Habit")
	require.NoError(t, repo.Create(ctx, archived))
	require.NoError(t, repo.Archive(ctx, archived.ID, time.Now().UTC()))

	list, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, active.ID, list[0].ID)
}

func TestHabitRepo_ListActive_OrderedByCreatedAt(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	h1 := newTestHabit("First")
	h1.CreatedAt = now.Add(-2 * time.Hour)
	h1.UpdatedAt = h1.CreatedAt

	h2 := newTestHabit("Second")
	h2.CreatedAt = now.Add(-1 * time.Hour)
	h2.UpdatedAt = h2.CreatedAt

	require.NoError(t, repo.Create(ctx, h1))
	require.NoError(t, repo.Create(ctx, h2))

	list, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, h1.ID, list[0].ID)
	assert.Equal(t, h2.ID, list[1].ID)
}

func TestHabitRepo_Archive_SoftDelete(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("Exercise")
	require.NoError(t, repo.Create(ctx, h))

	now := time.Now().UTC()
	require.NoError(t, repo.Archive(ctx, h.ID, now))

	fetched, err := repo.GetByID(ctx, h.ID)
	require.NoError(t, err)
	assert.NotNil(t, fetched.ArchivedAt)
	assert.False(t, fetched.IsActive())

	// Should not appear in active list
	list, err := repo.ListActive(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestHabitRepo_LogSession_AndLastLog(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("Meditation")
	require.NoError(t, repo.Create(ctx, h))

	// LastLog returns nil when no logs exist
	last, err := repo.LastLog(ctx, h.ID)
	require.NoError(t, err)
	assert.Nil(t, last)

	// Log two sessions; last should return the more recent one
	earlier := time.Now().UTC().Add(-2 * time.Hour)
	recent := time.Now().UTC().Add(-1 * time.Hour)

	log1 := newTestHabitLog(h.ID, earlier, 20)
	log2 := newTestHabitLog(h.ID, recent, 25)
	require.NoError(t, repo.LogSession(ctx, log1))
	require.NoError(t, repo.LogSession(ctx, log2))

	last, err = repo.LastLog(ctx, h.ID)
	require.NoError(t, err)
	require.NotNil(t, last)
	assert.Equal(t, log2.ID, last.ID)
	assert.Equal(t, 25, last.Minutes)
	assert.Equal(t, "test note", last.Note)
}

func TestHabitRepo_ListLogs_OrderedByPerformedAtDesc(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("Journaling")
	require.NoError(t, repo.Create(ctx, h))

	now := time.Now().UTC()
	log1 := newTestHabitLog(h.ID, now.Add(-3*time.Hour), 15)
	log2 := newTestHabitLog(h.ID, now.Add(-2*time.Hour), 20)
	log3 := newTestHabitLog(h.ID, now.Add(-1*time.Hour), 30)
	require.NoError(t, repo.LogSession(ctx, log1))
	require.NoError(t, repo.LogSession(ctx, log2))
	require.NoError(t, repo.LogSession(ctx, log3))

	logs, err := repo.ListLogs(ctx, h.ID, 10)
	require.NoError(t, err)
	require.Len(t, logs, 3)
	// Most recent first
	assert.Equal(t, log3.ID, logs[0].ID)
	assert.Equal(t, log2.ID, logs[1].ID)
	assert.Equal(t, log1.ID, logs[2].ID)
}

func TestHabitRepo_ListLogs_RespectsLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("Walking")
	require.NoError(t, repo.Create(ctx, h))

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		log := newTestHabitLog(h.ID, now.Add(time.Duration(-i)*time.Hour), 30)
		require.NoError(t, repo.LogSession(ctx, log))
	}

	logs, err := repo.ListLogs(ctx, h.ID, 3)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestHabitRepo_ListLogs_EmptyWhenNoLogs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteHabitRepo(db)
	ctx := context.Background()

	h := newTestHabit("Stretching")
	require.NoError(t, repo.Create(ctx, h))

	logs, err := repo.ListLogs(ctx, h.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

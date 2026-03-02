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

func TestWorkoutLogRepo_CreateAndListRecent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteWorkoutLogRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	w := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutCalisthenics,
		Minutes:     30,
		PerformedAt: now,
		CreatedAt:   now,
	}
	require.NoError(t, repo.Create(ctx, w))

	logs, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, w.ID, logs[0].ID)
	assert.Equal(t, domain.WorkoutCalisthenics, logs[0].Category)
	assert.Equal(t, 30, logs[0].Minutes)
	assert.Nil(t, logs[0].Notes)
}

func TestWorkoutLogRepo_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteWorkoutLogRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	w := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutRunning,
		Minutes:     45,
		PerformedAt: now,
		CreatedAt:   now,
	}
	require.NoError(t, repo.Create(ctx, w))

	require.NoError(t, repo.Delete(ctx, w.ID))

	logs, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestWorkoutLogRepo_ListByDateRange(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteWorkoutLogRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	inRange := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutQigong,
		Minutes:     20,
		PerformedAt: now.AddDate(0, 0, -3),
		CreatedAt:   now,
	}
	outOfRange := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutKettlebell,
		Minutes:     40,
		PerformedAt: now.AddDate(0, 0, -30),
		CreatedAt:   now,
	}
	require.NoError(t, repo.Create(ctx, inRange))
	require.NoError(t, repo.Create(ctx, outOfRange))

	from := now.AddDate(0, 0, -7)
	logs, err := repo.ListByDateRange(ctx, from, now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, inRange.ID, logs[0].ID)
}

func TestWorkoutLogRepo_NullableNotes(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteWorkoutLogRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	note := "morning flow"

	withNotes := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutQigong,
		Minutes:     15,
		PerformedAt: now,
		Notes:       &note,
		CreatedAt:   now,
	}
	withoutNotes := &domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    domain.WorkoutStretching,
		Minutes:     10,
		PerformedAt: now.Add(-time.Hour),
		CreatedAt:   now,
	}
	require.NoError(t, repo.Create(ctx, withNotes))
	require.NoError(t, repo.Create(ctx, withoutNotes))

	logs, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	require.Len(t, logs, 2)

	// Most recent first (performed_at DESC).
	assert.Equal(t, withNotes.ID, logs[0].ID)
	require.NotNil(t, logs[0].Notes)
	assert.Equal(t, "morning flow", *logs[0].Notes)

	assert.Equal(t, withoutNotes.ID, logs[1].ID)
	assert.Nil(t, logs[1].Notes)
}

func TestWorkoutLogRepo_ListRecentLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSQLiteWorkoutLogRepo(db)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		w := &domain.WorkoutLog{
			ID:          uuid.New().String(),
			Category:    domain.WorkoutOther,
			Minutes:     10,
			PerformedAt: now.Add(-time.Duration(i) * time.Hour),
			CreatedAt:   now,
		}
		require.NoError(t, repo.Create(ctx, w))
	}

	logs, err := repo.ListRecent(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

package service

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

func workoutTestSetup(t *testing.T) (WorkoutService, *repository.SQLiteWorkoutLogRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteWorkoutLogRepo(db)
	svc := NewWorkoutService(repo)
	return svc, repo
}

func TestLogWorkout_Success(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	w, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutCalisthenics,
		Minutes:  30,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
	assert.Equal(t, domain.WorkoutCalisthenics, w.Category)
	assert.Equal(t, 30, w.Minutes)
	assert.Nil(t, w.Notes)
}

func TestLogWorkout_InvalidCategory(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: "yoga",
		Minutes:  30,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workout category")
}

func TestLogWorkout_ZeroMinutes(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutRunning,
		Minutes:  0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minutes must be positive")
}

func TestLogWorkout_NegativeMinutes(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutRunning,
		Minutes:  -5,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minutes must be positive")
}

func TestLogWorkout_DefaultPerformedAt(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	before := time.Now().UTC()
	w, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutQigong,
		Minutes:  20,
	})
	require.NoError(t, err)
	assert.False(t, w.PerformedAt.Before(before), "performed_at should default to now")
}

func TestLogWorkout_WithDate(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	date := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)
	w, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category:    domain.WorkoutKettlebell,
		Minutes:     45,
		PerformedAt: &date,
	})
	require.NoError(t, err)
	assert.Equal(t, date, w.PerformedAt)
}

func TestLogWorkout_WithNotes(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	note := "morning session, felt great"
	w, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutStretching,
		Minutes:  15,
		Notes:    &note,
	})
	require.NoError(t, err)
	require.NotNil(t, w.Notes)
	assert.Equal(t, "morning session, felt great", *w.Notes)
}

func TestWorkoutService_DeleteWorkout(t *testing.T) {
	svc, repo := workoutTestSetup(t)
	ctx := context.Background()

	w, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutGMB,
		Minutes:  25,
	})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteWorkout(ctx, w.ID))

	logs, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestWorkoutService_ListRecent(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
			Category: domain.WorkoutOther,
			Minutes:  10,
		})
		require.NoError(t, err)
	}

	logs, err := svc.ListRecent(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestWorkoutService_ListRecent_DefaultLimit(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category: domain.WorkoutRunning,
		Minutes:  30,
	})
	require.NoError(t, err)

	// Zero limit should default to 20.
	logs, err := svc.ListRecent(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestWorkoutService_ListByDateRange(t *testing.T) {
	svc, _ := workoutTestSetup(t)
	ctx := context.Background()

	past := time.Now().UTC().AddDate(0, 0, -3)
	_, err := svc.LogWorkout(ctx, LogWorkoutRequest{
		Category:    domain.WorkoutQigong,
		Minutes:     20,
		PerformedAt: &past,
	})
	require.NoError(t, err)

	from := time.Now().UTC().AddDate(0, 0, -7)
	to := time.Now().UTC().Add(time.Hour)
	logs, err := svc.ListByDateRange(ctx, from, to)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

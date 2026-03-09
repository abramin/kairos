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

func habitTestSetup(t *testing.T) (HabitService, repository.HabitRepo) {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteHabitRepo(db)
	svc := NewHabitService(repo)
	return svc, repo
}

func TestHabitService_Add_EmptyTitle(t *testing.T) {
	svc, _ := habitTestSetup(t)
	_, err := svc.Add(context.Background(), AddHabitRequest{Title: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestHabitService_Add_DefaultsApplied(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	// All zero values → defaults applied
	h, err := svc.Add(ctx, AddHabitRequest{Title: "Meditation"})
	require.NoError(t, err)
	assert.Equal(t, 1, h.CadenceDays, "cadence defaults to 1 (daily)")
	assert.Equal(t, 20, h.TargetMin, "target defaults to 20")
	assert.Equal(t, 10, h.MinSessionMin, "min = max(5, 20-10) = 10")
	assert.Equal(t, 30, h.MaxSessionMin, "max = 20+10 = 30")
}

func TestHabitService_Add_MinSessionMinFloor(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	// TargetMin=8 → MinSessionMin default = max(5, 8-10) = max(5, -2) = 5
	h, err := svc.Add(ctx, AddHabitRequest{Title: "Quick stretch", TargetMin: 8})
	require.NoError(t, err)
	assert.Equal(t, 5, h.MinSessionMin, "min floored at 5 when target-10 is negative")
	assert.Equal(t, 18, h.MaxSessionMin, "max = 8+10 = 18")
}

func TestHabitService_Add_ExplicitValuesPreserved(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{
		Title:         "Exercise",
		CadenceDays:   7,
		TargetMin:     45,
		MinSessionMin: 20,
		MaxSessionMin: 60,
	})
	require.NoError(t, err)
	assert.Equal(t, 7, h.CadenceDays)
	assert.Equal(t, 45, h.TargetMin)
	assert.Equal(t, 20, h.MinSessionMin)
	assert.Equal(t, 60, h.MaxSessionMin)
}

func TestHabitService_ListActive_ExcludesArchived(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	_, err := svc.Add(ctx, AddHabitRequest{Title: "Active"})
	require.NoError(t, err)

	toArchive, err := svc.Add(ctx, AddHabitRequest{Title: "Will archive"})
	require.NoError(t, err)
	require.NoError(t, svc.Archive(ctx, toArchive.ID))

	list, err := svc.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Active", list[0].Title)
}

func TestHabitService_GetByID_ReturnsCorrectHabit(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Journaling", CadenceDays: 3})
	require.NoError(t, err)

	fetched, err := svc.GetByID(ctx, h.ID)
	require.NoError(t, err)
	assert.Equal(t, h.ID, fetched.ID)
	assert.Equal(t, "Journaling", fetched.Title)
}

func TestHabitService_LogSession_UsesTargetMinWhenZero(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Reading", TargetMin: 30})
	require.NoError(t, err)

	log, err := svc.LogSession(ctx, LogHabitRequest{HabitID: h.ID, Minutes: 0})
	require.NoError(t, err)
	assert.Equal(t, 30, log.Minutes, "zero minutes should default to habit's TargetMin")
}

func TestHabitService_LogSession_ExplicitMinutesPreserved(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Walking", TargetMin: 30})
	require.NoError(t, err)

	log, err := svc.LogSession(ctx, LogHabitRequest{HabitID: h.ID, Minutes: 45, Note: "long walk"})
	require.NoError(t, err)
	assert.Equal(t, 45, log.Minutes)
	assert.Equal(t, "long walk", log.Note)
}

func TestHabitService_LogSession_RejectsArchivedHabit(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Old habit"})
	require.NoError(t, err)
	require.NoError(t, svc.Archive(ctx, h.ID))

	_, err = svc.LogSession(ctx, LogHabitRequest{HabitID: h.ID, Minutes: 20})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archived")
}

func TestHabitService_GetStatus_NeverLogged(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "New habit", CadenceDays: 7})
	require.NoError(t, err)

	now := time.Now().UTC()
	statuses, err := svc.GetStatus(ctx, now)
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	s := statuses[0]
	assert.Equal(t, h.ID, s.Habit.ID)
	assert.Equal(t, 9999, s.DaysSinceLog, "never logged = 9999")
	assert.Nil(t, s.LastLog)
	// daysUntilDue = 7 - 9999 < 0 and daysSince > 0 → DueToday
	assert.True(t, s.DueToday)
}

func TestHabitService_GetStatus_LoggedToday(t *testing.T) {
	svc, _ := habitTestSetup(t)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Daily habit", CadenceDays: 1})
	require.NoError(t, err)

	now := time.Now().UTC()
	_, err = svc.LogSession(ctx, LogHabitRequest{HabitID: h.ID, Minutes: 20})
	require.NoError(t, err)

	statuses, err := svc.GetStatus(ctx, now)
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	s := statuses[0]
	assert.Equal(t, 0, s.DaysSinceLog, "logged today = 0 days since")
	assert.False(t, s.DueToday, "completed today should not be due")
	assert.NotNil(t, s.LastLog)
}

// TestHabitService_GetStatus_DaysUntilDue verifies the DaysUntilDue calculation
// by seeding a log via the repo directly (bypassing LogSession's internal time.Now).
func TestHabitService_GetStatus_DaysUntilDue(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewSQLiteHabitRepo(db)
	svc := NewHabitService(repo)
	ctx := context.Background()

	h, err := svc.Add(ctx, AddHabitRequest{Title: "Weekly", CadenceDays: 7})
	require.NoError(t, err)

	// Seed a log 3 days ago directly via repo so we control PerformedAt.
	now := time.Now().UTC()
	threeDaysAgo := now.AddDate(0, 0, -3)
	l := habitLogAt(h.ID, threeDaysAgo, 30)
	require.NoError(t, repo.LogSession(ctx, &l))

	statuses, err := svc.GetStatus(ctx, now)
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	s := statuses[0]
	assert.Equal(t, 3, s.DaysSinceLog)
	assert.Equal(t, 4, s.DaysUntilDue, "cadence 7 - daysSince 3 = 4 days until due")
	assert.False(t, s.DueToday, "4 days left means not due today")
}

// habitLogAt is a helper to build a domain.HabitLog with a specific PerformedAt time.
func habitLogAt(habitID string, performedAt time.Time, minutes int) domain.HabitLog {
	return domain.HabitLog{
		ID:          "log-" + habitID,
		HabitID:     habitID,
		PerformedAt: performedAt,
		Minutes:     minutes,
		CreatedAt:   time.Now().UTC(),
	}
}

func TestHabitService_GetStatus_EmptyWhenNoHabits(t *testing.T) {
	svc, _ := habitTestSetup(t)
	statuses, err := svc.GetStatus(context.Background(), time.Now().UTC())
	require.NoError(t, err)
	assert.Empty(t, statuses)
}

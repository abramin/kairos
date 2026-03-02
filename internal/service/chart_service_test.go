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

func TestChartService_WeeklyBreakdown_MixedData(t *testing.T) {
	database := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	workoutRepo := repository.NewSQLiteWorkoutLogRepo(database)

	// Create a project with sessions.
	proj := testutil.NewTestProject("Kairos")
	require.NoError(t, projRepo.Create(ctx, proj))
	node := testutil.NewTestNode(proj.ID, "Week 1")
	require.NoError(t, nodeRepo.Create(ctx, node))
	wi := testutil.NewTestWorkItem(node.ID, "Coding")
	require.NoError(t, wiRepo.Create(ctx, wi))

	now := time.Now().UTC()
	sess := testutil.NewTestSession(wi.ID, 90, testutil.WithStartedAt(now.Add(-1*time.Hour)))
	require.NoError(t, sessRepo.Create(ctx, sess))

	// Create workout logs.
	wl := &domain.WorkoutLog{
		ID:          "wl-001",
		Category:    domain.WorkoutQigong,
		Minutes:     20,
		PerformedAt: now.Add(-2 * time.Hour),
		CreatedAt:   now,
	}
	require.NoError(t, workoutRepo.Create(ctx, wl))

	svc := NewChartService(sessRepo, workoutRepo)
	breakdown, err := svc.WeeklyBreakdown(ctx, 1)
	require.NoError(t, err)

	// Should have at least 1 week.
	require.NotEmpty(t, breakdown)

	// Current week should have both project and workout segments.
	currentWeek := breakdown[0]
	assert.Greater(t, currentWeek.TotalMin, 0)

	var hasProject, hasWorkout bool
	for _, seg := range currentWeek.Segments {
		if seg.Kind == domain.SegmentProject {
			hasProject = true
			assert.Equal(t, "Kairos", seg.Label)
		}
		if seg.Kind == domain.SegmentWorkout {
			hasWorkout = true
			assert.Equal(t, "Qigong", seg.Label)
		}
	}
	assert.True(t, hasProject, "should have project segment")
	assert.True(t, hasWorkout, "should have workout segment")
	assert.Equal(t, 110, currentWeek.TotalMin, "90 project + 20 workout")
}

func TestChartService_WeeklyBreakdown_EmptyData(t *testing.T) {
	database := testutil.NewTestDB(t)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	workoutRepo := repository.NewSQLiteWorkoutLogRepo(database)

	svc := NewChartService(sessRepo, workoutRepo)
	breakdown, err := svc.WeeklyBreakdown(context.Background(), 2)
	require.NoError(t, err)

	// Should still return week entries (just with zero totals).
	require.NotEmpty(t, breakdown)
	for _, wk := range breakdown {
		assert.Equal(t, 0, wk.TotalMin)
		assert.Empty(t, wk.Segments)
	}
}

func TestChartService_WeeklyBreakdown_SegmentOrdering(t *testing.T) {
	database := testutil.NewTestDB(t)
	ctx := context.Background()

	projRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	wiRepo := repository.NewSQLiteWorkItemRepo(database)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	workoutRepo := repository.NewSQLiteWorkoutLogRepo(database)

	// Create two projects with different session amounts.
	projA := testutil.NewTestProject("Big Project")
	require.NoError(t, projRepo.Create(ctx, projA))
	nodeA := testutil.NewTestNode(projA.ID, "NA")
	require.NoError(t, nodeRepo.Create(ctx, nodeA))
	wiA := testutil.NewTestWorkItem(nodeA.ID, "TA")
	require.NoError(t, wiRepo.Create(ctx, wiA))

	projB := testutil.NewTestProject("Small Project")
	require.NoError(t, projRepo.Create(ctx, projB))
	nodeB := testutil.NewTestNode(projB.ID, "NB")
	require.NoError(t, nodeRepo.Create(ctx, nodeB))
	wiB := testutil.NewTestWorkItem(nodeB.ID, "TB")
	require.NoError(t, wiRepo.Create(ctx, wiB))

	now := time.Now().UTC()
	sessA := testutil.NewTestSession(wiA.ID, 120, testutil.WithStartedAt(now.Add(-1*time.Hour)))
	sessB := testutil.NewTestSession(wiB.ID, 30, testutil.WithStartedAt(now.Add(-2*time.Hour)))
	require.NoError(t, sessRepo.Create(ctx, sessA))
	require.NoError(t, sessRepo.Create(ctx, sessB))

	svc := NewChartService(sessRepo, workoutRepo)
	breakdown, err := svc.WeeklyBreakdown(ctx, 1)
	require.NoError(t, err)
	require.NotEmpty(t, breakdown)

	// Segments should be ordered by minutes descending.
	segs := breakdown[0].Segments
	require.Len(t, segs, 2)
	assert.Equal(t, "Big Project", segs[0].Label)
	assert.Equal(t, "Small Project", segs[1].Label)
}

func TestWeekLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-W08", "Feb 16–22"},
		// ISO week 1 of 2026 starts Dec 29 2025 (Jan 1 is a Thursday).
		{"2026-W01", "Dec 29–4"},
		{"2026-W02", "Jan 5–11"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := weekLabel(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestISOWeek(t *testing.T) {
	d := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-W08", isoWeek(d))
}

func TestChartService_DefaultNumWeeks(t *testing.T) {
	database := testutil.NewTestDB(t)
	sessRepo := repository.NewSQLiteSessionRepo(database)
	workoutRepo := repository.NewSQLiteWorkoutLogRepo(database)

	svc := NewChartService(sessRepo, workoutRepo)
	// numWeeks=0 should default to 6.
	breakdown, err := svc.WeeklyBreakdown(context.Background(), 0)
	require.NoError(t, err)
	assert.Len(t, breakdown, 6)
}

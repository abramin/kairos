package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocateSlices_SessionBoundsNeverViolated(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	due := now.AddDate(0, 0, 14)

	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Task 1",
				DueDate:           &due,
				ProjectRisk:       domain.RiskAtRisk,
				MinSessionMin:     20,
				MaxSessionMin:     45,
				DefaultSessionMin: 30,
				PlannedMin:        100,
				LoggedMin:         0,
				NodeID:            "n-1",
			},
			Score:   50.0,
			Reasons: []app.RecommendationReason{},
		},
	}

	slices, _ := AllocateSlices(candidates, 60, 3, false)

	require.Len(t, slices, 1)
	assert.GreaterOrEqual(t, slices[0].AllocatedMin, 20, "must respect min session")
	assert.LessOrEqual(t, slices[0].AllocatedMin, 45, "must respect max session")
}

func TestAllocateSlices_InsufficientTimeBlocked(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Task",
				MinSessionMin:     20,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
				PlannedMin:        100,
			},
			Score: 50.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 15, 3, false) // 15 < min 20

	assert.Empty(t, slices)
	assert.NotEmpty(t, blockers)
	assert.Equal(t, app.BlockerSessionMinExceedsAvail, blockers[0].Code)
}

func TestAllocateSlices_VariationPrefersMultipleProjects(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	due := now.AddDate(0, 0, 14)

	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:  "wi-1", ProjectID: "p-1", ProjectName: "A",
				Title: "A Task 1", DueDate: &due,
				ProjectRisk:       domain.RiskAtRisk,
				MinSessionMin:     15,
				MaxSessionMin:     45,
				DefaultSessionMin: 30,
				PlannedMin:        100,
				NodeID:            "n-1",
			},
			Score: 60.0,
		},
		{
			Input: ScoringInput{
				WorkItemID:  "wi-2", ProjectID: "p-1", ProjectName: "A",
				Title: "A Task 2", DueDate: &due,
				ProjectRisk:       domain.RiskAtRisk,
				MinSessionMin:     15,
				MaxSessionMin:     45,
				DefaultSessionMin: 30,
				PlannedMin:        100,
				NodeID:            "n-1",
			},
			Score: 55.0,
		},
		{
			Input: ScoringInput{
				WorkItemID:  "wi-3", ProjectID: "p-2", ProjectName: "B",
				Title: "B Task 1", DueDate: &due,
				ProjectRisk:       domain.RiskOnTrack,
				MinSessionMin:     15,
				MaxSessionMin:     45,
				DefaultSessionMin: 30,
				PlannedMin:        100,
				NodeID:            "n-2",
			},
			Score: 40.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 90, 3, true)

	// With variation, should include item from project B even though A scored higher
	projectIDs := make(map[string]bool)
	for _, s := range slices {
		projectIDs[s.ProjectID] = true
	}
	assert.True(t, projectIDs["p-2"], "should include project B for variation")
}

func TestAllocateSlices_ExtendBeforeAddingSameProject(t *testing.T) {
	// Single project, two items, 60 min available.
	// Pass 1 gives wi-1 30 min; extension should grow it to 60 min,
	// leaving no room for wi-2.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID: "wi-1", ProjectID: "p-1", ProjectName: "A",
				Title:             "Wk18",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 270, LoggedMin: 0, NodeID: "n-1",
			},
			Score: 80.0,
		},
		{
			Input: ScoringInput{
				WorkItemID: "wi-2", ProjectID: "p-1", ProjectName: "A",
				Title:             "Wk19",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 270, LoggedMin: 0, NodeID: "n-2",
			},
			Score: 70.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 60, 5, true)

	require.Len(t, slices, 1, "should allocate one slice — extend wi-1 instead of adding wi-2")
	assert.Equal(t, "wi-1", slices[0].WorkItemID)
	assert.Equal(t, 60, slices[0].AllocatedMin, "wi-1 should be extended to fill available time")
}

func TestAllocateSlices_ExtensionCappedByMaxSession(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID: "wi-1", ProjectID: "p-1", ProjectName: "A",
				Title:             "Task",
				MinSessionMin:     15, MaxSessionMin: 40, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-1",
			},
			Score: 80.0,
		},
		{
			Input: ScoringInput{
				WorkItemID: "wi-2", ProjectID: "p-1", ProjectName: "A",
				Title:             "Task 2",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-2",
			},
			Score: 70.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 90, 5, true)

	require.Len(t, slices, 2, "wi-1 caps at 40, so wi-2 fills the rest")
	assert.Equal(t, "wi-1", slices[0].WorkItemID)
	assert.Equal(t, 40, slices[0].AllocatedMin, "wi-1 capped at maxSessionMin")
	assert.Equal(t, "wi-2", slices[1].WorkItemID)
}

func TestAllocateSlices_ExtensionCappedByWorkRemaining(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID: "wi-1", ProjectID: "p-1", ProjectName: "A",
				Title:             "Task",
				MinSessionMin:     15, MaxSessionMin: 90, DefaultSessionMin: 30,
				PlannedMin: 100, LoggedMin: 55, // 45 min remaining
				NodeID: "n-1",
			},
			Score: 80.0,
		},
		{
			Input: ScoringInput{
				WorkItemID: "wi-2", ProjectID: "p-1", ProjectName: "A",
				Title:             "Task 2",
				MinSessionMin:     15, MaxSessionMin: 90, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-2",
			},
			Score: 70.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 90, 5, true)

	require.Len(t, slices, 2)
	assert.Equal(t, "wi-1", slices[0].WorkItemID)
	assert.Equal(t, 45, slices[0].AllocatedMin, "wi-1 capped at work remaining")
	assert.Equal(t, "wi-2", slices[1].WorkItemID)
	assert.Equal(t, 30, slices[1].AllocatedMin, "wi-2 gets its default session")
}

func TestAllocateSlices_FullyLoggedItemBlocked(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Done Task",
				MinSessionMin:     15,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
				PlannedMin:        60,
				LoggedMin:         60, // 100% complete
				NodeID:            "n-1",
			},
			Score: 80.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 60, 3, false)

	assert.Empty(t, slices, "fully logged item should not be allocated")
	require.Len(t, blockers, 1)
	assert.Equal(t, app.BlockerWorkComplete, blockers[0].Code)
}

func TestAllocateSlices_OverLoggedItemBlocked(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Over-logged Task",
				MinSessionMin:     15,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
				PlannedMin:        60,
				LoggedMin:         90, // 150% complete
				NodeID:            "n-1",
			},
			Score: 80.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 60, 3, false)

	assert.Empty(t, slices, "over-logged item should not be allocated")
	require.Len(t, blockers, 1)
	assert.Equal(t, app.BlockerWorkComplete, blockers[0].Code)
}

func TestAllocateSlices_HabitNotBlockedWhenWorkComplete(t *testing.T) {
	// A habit with PlannedMin == LoggedMin should still be allocated because
	// IsHabit=true skips the work-complete blocker in tryAllocate.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "habit:h-1",
				ProjectID:         "habit:h-1",
				ProjectName:       "Exercise",
				Title:             "Exercise",
				MinSessionMin:     10,
				MaxSessionMin:     30,
				DefaultSessionMin: 20,
				PlannedMin:        0, // habits have no planned work
				LoggedMin:         0,
				IsHabit:           true,
				HabitID:           "h-1",
				HabitCadenceDays:  1,
				HabitDaysSince:    2,
			},
			Score: 50.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 60, 3, false)

	assert.Empty(t, blockers, "habit should not be blocked by work-complete check")
	require.Len(t, slices, 1)
	assert.Equal(t, "habit:h-1", slices[0].WorkItemID)
	assert.True(t, slices[0].IsHabit)
}

func TestAllocateSlices_HabitCeilingUsesMaxSessionMin(t *testing.T) {
	// A habit's ceiling is MaxSessionMin only — no PlannedMin-LoggedMin constraint.
	// With 60 min available and MaxSessionMin=25, habit gets at most 25.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "habit:h-1",
				ProjectID:         "habit:h-1",
				ProjectName:       "Reading",
				Title:             "Reading",
				MinSessionMin:     10,
				MaxSessionMin:     25,
				DefaultSessionMin: 20,
				PlannedMin:        0,
				LoggedMin:         0,
				IsHabit:           true,
				HabitID:           "h-1",
				HabitCadenceDays:  7,
				HabitDaysSince:    7,
			},
			Score: 60.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 60, 3, false)

	assert.Empty(t, blockers)
	require.Len(t, slices, 1)
	assert.LessOrEqual(t, slices[0].AllocatedMin, 25, "habit allocation must not exceed MaxSessionMin")
	assert.GreaterOrEqual(t, slices[0].AllocatedMin, 10, "habit allocation must meet MinSessionMin")
}

func TestAllocateSlices_RegularItemBlockedWhenWorkComplete(t *testing.T) {
	// Confirms the work-complete check still fires for non-habits.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "wi-1",
				ProjectID:         "p-1",
				ProjectName:       "A",
				Title:             "Done Task",
				MinSessionMin:     15,
				MaxSessionMin:     60,
				DefaultSessionMin: 30,
				PlannedMin:        60,
				LoggedMin:         60,
				IsHabit:           false,
			},
			Score: 80.0,
		},
	}

	slices, blockers := AllocateSlices(candidates, 60, 3, false)

	assert.Empty(t, slices)
	require.Len(t, blockers, 1)
	assert.Equal(t, app.BlockerWorkComplete, blockers[0].Code)
}

func TestAllocateSlices_HabitSliceHasHabitFields(t *testing.T) {
	// Verify WorkSlice output carries IsHabit, HabitID, CadenceDays, DaysSinceLog.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID:        "habit:h-99",
				ProjectID:         "habit:h-99",
				ProjectName:       "Meditation",
				Title:             "Meditation",
				MinSessionMin:     10,
				MaxSessionMin:     30,
				DefaultSessionMin: 20,
				IsHabit:           true,
				HabitID:           "h-99",
				HabitCadenceDays:  1,
				HabitDaysSince:    3,
			},
			Score: 50.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 60, 3, false)

	require.Len(t, slices, 1)
	assert.True(t, slices[0].IsHabit)
	assert.Equal(t, "h-99", slices[0].HabitID)
	assert.Equal(t, 1, slices[0].CadenceDays)
	assert.Equal(t, 3, slices[0].DaysSinceLog)
}

func TestAllocateSlices_ExtensionMultipleProjects(t *testing.T) {
	// Two projects, 90 min available. Pass 1 gives each 30 min (60 total).
	// Extension distributes remaining 30 min across both pass-1 slices.
	candidates := []ScoredCandidate{
		{
			Input: ScoringInput{
				WorkItemID: "wi-1", ProjectID: "p-1", ProjectName: "A",
				Title:             "A Task",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-1",
			},
			Score: 80.0,
		},
		{
			Input: ScoringInput{
				WorkItemID: "wi-2", ProjectID: "p-1", ProjectName: "A",
				Title:             "A Task 2",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-1",
			},
			Score: 70.0,
		},
		{
			Input: ScoringInput{
				WorkItemID: "wi-3", ProjectID: "p-2", ProjectName: "B",
				Title:             "B Task",
				MinSessionMin:     15, MaxSessionMin: 60, DefaultSessionMin: 30,
				PlannedMin: 200, LoggedMin: 0, NodeID: "n-2",
			},
			Score: 60.0,
		},
	}

	slices, _ := AllocateSlices(candidates, 90, 5, true)

	require.Len(t, slices, 2, "one per project — extension fills before adding deferred")
	total := slices[0].AllocatedMin + slices[1].AllocatedMin
	assert.Equal(t, 90, total, "should use all available time")
	// Extension distributes greedily: highest-priority slice (wi-1) extends first
	assert.Equal(t, 60, slices[0].AllocatedMin, "wi-1 extended to maxSessionMin")
	assert.Equal(t, 30, slices[1].AllocatedMin, "wi-3 gets remaining time at default")
}

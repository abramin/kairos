package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
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
			Reasons: []contract.RecommendationReason{},
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
	assert.Equal(t, contract.BlockerSessionMinExceedsAvail, blockers[0].Code)
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

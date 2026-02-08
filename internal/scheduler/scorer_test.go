package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestScoreWorkItem_DeadlinePressure(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	tomorrow := now.AddDate(0, 0, 1)

	result := ScoreWorkItem(ScoringInput{
		WorkItemID:        "wi-1",
		ProjectID:         "p-1",
		ProjectName:       "Test",
		Title:             "Task",
		DueDate:           &tomorrow,
		ProjectRisk:       domain.RiskAtRisk,
		Now:               now,
		Weights:           DefaultWeights(),
		Mode:              domain.ModeBalanced,
		MinSessionMin:     15,
		MaxSessionMin:     60,
		DefaultSessionMin: 30,
	})

	assert.False(t, result.Blocked)
	assert.Greater(t, result.Score, 0.0)

	// Should have deadline pressure reason
	hasDeadlinePressure := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonDeadlinePressure {
			hasDeadlinePressure = true
		}
	}
	assert.True(t, hasDeadlinePressure, "should have DEADLINE_PRESSURE reason")
}

func TestScoreWorkItem_CriticalModeBlocksNonCritical(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	result := ScoreWorkItem(ScoringInput{
		WorkItemID:        "wi-1",
		ProjectID:         "p-1",
		ProjectName:       "OnTrack Project",
		Title:             "Task",
		ProjectRisk:       domain.RiskOnTrack,
		Now:               now,
		Weights:           DefaultWeights(),
		Mode:              domain.ModeCritical,
		MinSessionMin:     15,
		MaxSessionMin:     60,
		DefaultSessionMin: 30,
	})

	assert.True(t, result.Blocked, "non-critical item should be blocked in critical mode")
}

func TestScoreWorkItem_CriticalModeBoostsCritical(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	result := ScoreWorkItem(ScoringInput{
		WorkItemID:        "wi-1",
		ProjectID:         "p-1",
		ProjectName:       "Critical Project",
		Title:             "Task",
		ProjectRisk:       domain.RiskCritical,
		Now:               now,
		Weights:           DefaultWeights(),
		Mode:              domain.ModeCritical,
		MinSessionMin:     15,
		MaxSessionMin:     60,
		DefaultSessionMin: 30,
	})

	assert.False(t, result.Blocked)
	assert.GreaterOrEqual(t, result.Score, 50.0, "critical item in critical mode should have high score")

	hasCriticalFocus := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonCriticalFocus {
			hasCriticalFocus = true
		}
	}
	assert.True(t, hasCriticalFocus)
}

func TestScoreWorkItem_SpacingBonus(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	daysAgo := 2

	result := ScoreWorkItem(ScoringInput{
		WorkItemID:         "wi-1",
		ProjectID:          "p-1",
		ProjectName:        "Test",
		Title:              "Task",
		ProjectRisk:        domain.RiskOnTrack,
		Now:                now,
		LastSessionDaysAgo: &daysAgo,
		Weights:            DefaultWeights(),
		Mode:               domain.ModeBalanced,
		MinSessionMin:      15,
		MaxSessionMin:      60,
		DefaultSessionMin:  30,
	})

	assert.False(t, result.Blocked)

	hasSpacingOK := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonSpacingOK {
			hasSpacingOK = true
			assert.NotNil(t, r.WeightDelta)
			assert.Greater(t, *r.WeightDelta, 0.0, "1-3 days spacing should give positive bonus")
		}
	}
	assert.True(t, hasSpacingOK, "should have SPACING_OK reason")
}

func TestScoreWorkItem_SpacingPenalty_WorkedToday(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	daysAgo := 0

	result := ScoreWorkItem(ScoringInput{
		WorkItemID:         "wi-1",
		ProjectID:          "p-1",
		ProjectName:        "Test",
		Title:              "Task",
		ProjectRisk:        domain.RiskOnTrack,
		Now:                now,
		LastSessionDaysAgo: &daysAgo,
		Weights:            DefaultWeights(),
		Mode:               domain.ModeBalanced,
		MinSessionMin:      15,
		MaxSessionMin:      60,
		DefaultSessionMin:  30,
	})

	hasSpacingBlocked := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonSpacingBlocked {
			hasSpacingBlocked = true
			assert.NotNil(t, r.WeightDelta)
			assert.Less(t, *r.WeightDelta, 0.0, "worked today should have negative spacing delta")
		}
	}
	assert.True(t, hasSpacingBlocked, "should have SPACING_BLOCKED reason when worked today")
}

func TestScoreWorkItem_VariationBonus(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	// First slice from project — should get variation bonus
	result := ScoreWorkItem(ScoringInput{
		WorkItemID:          "wi-1",
		ProjectID:           "p-1",
		ProjectName:         "Test",
		Title:               "Task",
		ProjectRisk:         domain.RiskOnTrack,
		Now:                 now,
		ProjectSlicesInPlan: 0,
		Weights:             DefaultWeights(),
		Mode:                domain.ModeBalanced,
		MinSessionMin:       15,
		MaxSessionMin:       60,
		DefaultSessionMin:   30,
	})

	hasVariationBonus := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonVariationBonus {
			hasVariationBonus = true
			assert.NotNil(t, r.WeightDelta)
			assert.Greater(t, *r.WeightDelta, 0.0, "first slice should get positive variation bonus")
		}
	}
	assert.True(t, hasVariationBonus, "should have VARIATION_BONUS reason for first slice from project")
}

func TestScoreWorkItem_VariationPenalty(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	// 3 slices already from this project — should get penalty
	result := ScoreWorkItem(ScoringInput{
		WorkItemID:          "wi-1",
		ProjectID:           "p-1",
		ProjectName:         "Test",
		Title:               "Task",
		ProjectRisk:         domain.RiskOnTrack,
		Now:                 now,
		ProjectSlicesInPlan: 3,
		Weights:             DefaultWeights(),
		Mode:                domain.ModeBalanced,
		MinSessionMin:       15,
		MaxSessionMin:       60,
		DefaultSessionMin:   30,
	})

	hasVariationPenalty := false
	for _, r := range result.Reasons {
		if r.Code == contract.ReasonVariationPenalty {
			hasVariationPenalty = true
			assert.NotNil(t, r.WeightDelta)
			assert.Less(t, *r.WeightDelta, 0.0, "3+ slices should get negative variation penalty")
		}
	}
	assert.True(t, hasVariationPenalty, "should have VARIATION_PENALTY reason for overrepresented project")
}

package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
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
		Weights:           defaultWeights(),
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
		if r.Code == app.ReasonDeadlinePressure {
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
		Weights:           defaultWeights(),
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
		Weights:           defaultWeights(),
		Mode:              domain.ModeCritical,
		MinSessionMin:     15,
		MaxSessionMin:     60,
		DefaultSessionMin: 30,
	})

	assert.False(t, result.Blocked)
	assert.GreaterOrEqual(t, result.Score, 50.0, "critical item in critical mode should have high score")

	hasCriticalFocus := false
	for _, r := range result.Reasons {
		if r.Code == app.ReasonCriticalFocus {
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
		Weights:            defaultWeights(),
		Mode:               domain.ModeBalanced,
		MinSessionMin:      15,
		MaxSessionMin:      60,
		DefaultSessionMin:  30,
	})

	assert.False(t, result.Blocked)

	hasSpacingOK := false
	for _, r := range result.Reasons {
		if r.Code == app.ReasonSpacingOK {
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
		Weights:            defaultWeights(),
		Mode:               domain.ModeBalanced,
		MinSessionMin:      15,
		MaxSessionMin:      60,
		DefaultSessionMin:  30,
	})

	hasSpacingBlocked := false
	for _, r := range result.Reasons {
		if r.Code == app.ReasonSpacingBlocked {
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
		Weights:             defaultWeights(),
		Mode:                domain.ModeBalanced,
		MinSessionMin:       15,
		MaxSessionMin:       60,
		DefaultSessionMin:   30,
	})

	hasVariationBonus := false
	for _, r := range result.Reasons {
		if r.Code == app.ReasonVariationBonus {
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
		Weights:             defaultWeights(),
		Mode:                domain.ModeBalanced,
		MinSessionMin:       15,
		MaxSessionMin:       60,
		DefaultSessionMin:   30,
	})

	hasVariationPenalty := false
	for _, r := range result.Reasons {
		if r.Code == app.ReasonVariationPenalty {
			hasVariationPenalty = true
			assert.NotNil(t, r.WeightDelta)
			assert.Less(t, *r.WeightDelta, 0.0, "3+ slices should get negative variation penalty")
		}
	}
	assert.True(t, hasVariationPenalty, "should have VARIATION_PENALTY reason for overrepresented project")
}

func habitInput(cadenceDays, daysSince int) ScoringInput {
	return ScoringInput{
		WorkItemID:       "habit:h-1",
		ProjectID:        "habit:h-1",
		ProjectName:      "Exercise",
		Title:            "Exercise",
		Now:              time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC),
		Weights:          defaultWeights(),
		Mode:             domain.ModeBalanced,
		MinSessionMin:    10,
		MaxSessionMin:    30,
		DefaultSessionMin: 20,
		IsHabit:          true,
		HabitID:          "h-1",
		HabitCadenceDays: cadenceDays,
		HabitDaysSince:   daysSince,
	}
}

func hasReasonCode(reasons []app.RecommendationReason, code app.RecommendationReasonCode) bool {
	for _, r := range reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

// TestScoreWorkItem_HabitUrgency covers all five tiers of scoreHabitUrgency.
func TestScoreWorkItem_HabitUrgency(t *testing.T) {
	tests := []struct {
		name         string
		cadenceDays  int
		daysSince    int
		wantBlocked  bool
		wantMinScore float64
		wantReason   app.RecommendationReasonCode
	}{
		{
			name:         "overdue (fraction >= 1.5)",
			cadenceDays:  4, daysSince: 7, // 7/4 = 1.75
			wantBlocked:  false,
			wantMinScore: 50.0,
			wantReason:   app.ReasonHabitOverdue,
		},
		{
			name:         "due today (fraction >= 1.0)",
			cadenceDays:  7, daysSince: 7, // 7/7 = 1.0
			wantBlocked:  false,
			wantMinScore: 30.0,
			wantReason:   app.ReasonHabitDue,
		},
		{
			name:         "approaching (fraction >= 0.8)",
			cadenceDays:  5, daysSince: 4, // 4/5 = 0.8
			wantBlocked:  false,
			wantMinScore: 15.0,
			wantReason:   app.ReasonHabitApproaching,
		},
		{
			name:         "not yet due (fraction < 0.8)",
			cadenceDays:  7, daysSince: 2, // 2/7 ≈ 0.29
			wantBlocked:  false,
			wantMinScore: 0.0,
			wantReason:   "",
		},
		{
			name:         "zero cadence treated as daily",
			cadenceDays:  0, daysSince: 2, // fraction = 2 (>= 1.5)
			wantBlocked:  false,
			wantMinScore: 50.0,
			wantReason:   app.ReasonHabitOverdue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ScoreWorkItem(habitInput(tc.cadenceDays, tc.daysSince))
			assert.Equal(t, tc.wantBlocked, result.Blocked)
			assert.GreaterOrEqual(t, result.Score, tc.wantMinScore)
			if tc.wantReason != "" {
				assert.True(t, hasReasonCode(result.Reasons, tc.wantReason),
					"expected reason %s in %v", tc.wantReason, result.Reasons)
			} else {
				assert.False(t, hasReasonCode(result.Reasons, app.ReasonHabitDue))
				assert.False(t, hasReasonCode(result.Reasons, app.ReasonHabitOverdue))
				assert.False(t, hasReasonCode(result.Reasons, app.ReasonHabitApproaching))
			}
		})
	}
}

// TestScoreWorkItem_HabitIsNotAffectedByWorkItemFactors verifies that non-habit
// scoring factors (deadline, variation, spacing) do not fire for habit inputs
// that lack the corresponding fields.
func TestScoreWorkItem_HabitIsNotAffectedByWorkItemFactors(t *testing.T) {
	result := ScoreWorkItem(habitInput(1, 2)) // overdue daily habit

	assert.False(t, hasReasonCode(result.Reasons, app.ReasonDeadlinePressure))
	assert.False(t, hasReasonCode(result.Reasons, app.ReasonBehindPace))
}

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

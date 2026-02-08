package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestComputeRisk_NoTargetDate(t *testing.T) {
	result := ComputeRisk(RiskInput{
		Now:        time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC),
		PlannedMin: 100,
		LoggedMin:  50,
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level)
}

func TestComputeRisk_PastDue(t *testing.T) {
	yesterday := time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:        time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC),
		TargetDate: &yesterday,
		PlannedMin: 100,
		LoggedMin:  50,
	})
	assert.Equal(t, domain.RiskCritical, result.Level)
}

func TestComputeRisk_Critical_HighRatio(t *testing.T) {
	target := time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC) // 5 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0.1,
		RecentDailyMin: 30,
	})
	// remaining = 1100, required = 220/day, recent = 30/day, ratio = 7.3 > 1.5
	assert.Equal(t, domain.RiskCritical, result.Level)
}

func TestComputeRisk_AtRisk_MediumRatio(t *testing.T) {
	target := time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC) // 31 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     600,
		LoggedMin:      0,
		BufferPct:      0.1,
		RecentDailyMin: 18, // required ~21, ratio ~1.17
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level)
}

func TestComputeRisk_OnTrack_LowRatio(t *testing.T) {
	target := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC) // ~92 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     600,
		LoggedMin:      300,
		BufferPct:      0.1,
		RecentDailyMin: 10, // remaining ~330, required ~3.6/day, ratio ~0.36
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level)
}

func TestComputeRisk_NoRecentActivity_OnPace_IsAtRisk(t *testing.T) {
	// 60% progress at 55% of timeline => on pace, but no recent sessions.
	// Without structural data this would be critical; with it, capped at at_risk.
	target := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      500,
		RecentDailyMin: 0,
		ProgressPct:    60,
		TimeElapsedPct: 55,
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level)
}

func TestComputeRisk_NoRecentActivity_BehindPace_StillCritical(t *testing.T) {
	// 30% progress at 55% of timeline => behind pace, no recent sessions => critical.
	target := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      300,
		RecentDailyMin: 0,
		ProgressPct:    30,
		TimeElapsedPct: 55,
	})
	assert.Equal(t, domain.RiskCritical, result.Level)
}

func TestComputeRisk_HighRatio_OnPace_CappedAtRisk(t *testing.T) {
	// High required/recent ratio (>1.5) but structurally ahead => capped at at_risk.
	target := time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC) // 5 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0.1,
		RecentDailyMin: 30,
		ProgressPct:    70,
		TimeElapsedPct: 60,
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level)
}

func TestComputeRisk_ZeroStructuralData_PreservesOldBehavior(t *testing.T) {
	// ProgressPct and TimeElapsedPct both 0 => onPace is false => old behavior.
	target := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      500,
		RecentDailyMin: 0,
		ProgressPct:    0,
		TimeElapsedPct: 0,
	})
	assert.Equal(t, domain.RiskCritical, result.Level)
}

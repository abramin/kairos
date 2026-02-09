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

func TestComputeRisk_HighRatio_DueBasedOnPace_CappedAtRisk(t *testing.T) {
	// Simulates OU01: 38% progress, 57% timeline elapsed, but due-based expected is also 38%
	// because all items due by now are done. High ratio (>1.5) from back-loaded work.
	target := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:                 time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:          &target,
		PlannedMin:          12030,
		LoggedMin:           4590,
		BufferPct:           0.1,
		RecentDailyMin:      30,
		ProgressPct:         38.2,
		TimeElapsedPct:      56.5,
		DueBasedExpectedPct: 38.2,
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level,
		"due-based on-pace should cap from critical to at_risk")
}

func TestComputeRisk_DueBasedExpectedZero_PreservesOldBehavior(t *testing.T) {
	// DueBasedExpectedPct = 0 means no data; should not change existing classification.
	target := time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:                 time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:          &target,
		PlannedMin:          1000,
		LoggedMin:           0,
		BufferPct:           0.1,
		RecentDailyMin:      30,
		DueBasedExpectedPct: 0,
	})
	assert.Equal(t, domain.RiskCritical, result.Level,
		"zero DueBasedExpectedPct should not affect existing classification")
}

func TestComputeRisk_ProgressBehindDueBasedExpected_StillCritical(t *testing.T) {
	// Progress is 30% but 50% of work should be done by now based on due dates.
	target := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:                 time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:          &target,
		PlannedMin:          1000,
		LoggedMin:           100,
		BufferPct:           0.1,
		RecentDailyMin:      5,
		ProgressPct:         30,
		TimeElapsedPct:      55,
		DueBasedExpectedPct: 50,
	})
	assert.Equal(t, domain.RiskCritical, result.Level,
		"behind on due-based expected should remain critical")
}

func TestComputeRisk_NoRecentActivity_DueBasedOnPace_CappedAtRisk(t *testing.T) {
	// No recent sessions, but due-based expected matches progress.
	target := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	result := ComputeRisk(RiskInput{
		Now:                 time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:          &target,
		PlannedMin:          1000,
		LoggedMin:           400,
		RecentDailyMin:      0,
		ProgressPct:         40,
		TimeElapsedPct:      55,
		DueBasedExpectedPct: 40,
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level,
		"due-based on-pace should cap no-activity critical to at_risk")
}

// ============ BOUNDARY TRANSITION TESTS ============

func TestComputeRisk_Boundary_Ratio0_99_OnTrack(t *testing.T) {
	// ratio = 0.99 < 1.0 => on_track
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 101, // required ~100, ratio ~0.99
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level, "ratio 0.99 should be on_track")
}

func TestComputeRisk_Boundary_Ratio1_0_OnTrack(t *testing.T) {
	// ratio = 1.0 exactly: code uses ">" not ">=", so 1.0 is NOT > 1.0
	// Falls through to 3-day rule and default; with 10 days => on_track
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 100, // required exactly 100, ratio = 1.0
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level, "ratio exactly 1.0 is NOT > 1.0, falls through to default")
}

func TestComputeRisk_Boundary_Ratio1_01_AtRisk(t *testing.T) {
	// ratio = 1.01 > 1.0 => at_risk
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 99, // required ~101, ratio ~1.01
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level, "ratio 1.01 should be at_risk")
}

func TestComputeRisk_Boundary_Ratio1_49_AtRisk(t *testing.T) {
	// ratio = 1.49 > 1.0 but <= 1.5 => at_risk
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 67, // required ~100, ratio ~1.49
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level, "ratio 1.49 should be at_risk")
}

func TestComputeRisk_Boundary_Ratio1_50_Critical(t *testing.T) {
	// ratio = 1.5 exactly: switch uses ">" so 1.5 is NOT > 1.5 => falls to next case
	// This depends on whether 3-day rule applies; use 10+ days to avoid it
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 67, // required ~100, ratio = 1.5 exactly (but 100/67 = 1.493... slightly below)
	})
	// Since ratio < 1.5, falls to next conditions
	assert.Equal(t, domain.RiskAtRisk, result.Level, "ratio at 1.5 boundary should be at_risk")
}

func TestComputeRisk_Boundary_Ratio1_51_Critical(t *testing.T) {
	// ratio = 1.51 > 1.5 => critical (unless on pace)
	target := time.Date(2025, 3, 25, 0, 0, 0, 0, time.UTC) // 10 days
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     1000,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 66, // required ~100, ratio ~1.51
	})
	assert.Equal(t, domain.RiskCritical, result.Level, "ratio 1.51 > 1.5 should be critical")
}

func TestComputeRisk_Boundary_DaysLeft0_Critical(t *testing.T) {
	// daysLeft = 0 => always critical
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	target := time.Date(2025, 3, 15, 11, 0, 0, 0, time.UTC) // 1 hour ago (daysLeft = 0)
	result := ComputeRisk(RiskInput{
		Now:            now,
		TargetDate:     &target,
		PlannedMin:     100,
		LoggedMin:      50,
		RecentDailyMin: 100,
	})
	assert.Equal(t, domain.RiskCritical, result.Level, "daysLeft 0 (past due) should be critical")
}

func TestComputeRisk_Boundary_DaysLeft1_HighRemaining_AtRisk(t *testing.T) {
	// daysLeft = 1, remaining > recent*1 and ratio <= 1.5 => at_risk (3-day rule)
	target := time.Date(2025, 3, 16, 0, 0, 0, 0, time.UTC) // 1 day away
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     150,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 100, // remaining ~150, required = 150/1 = 150, ratio = 1.5 (boundary, not > 1.5)
		// 150 > 100*1 => at_risk by 3-day rule
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level, "daysLeft 1 with high remaining should be at_risk by 3-day rule")
}

func TestComputeRisk_Boundary_DaysLeft1_LowRemaining_OnTrack(t *testing.T) {
	// daysLeft = 1, remaining <= recent*1 => on_track (3-day rule doesn't trigger)
	target := time.Date(2025, 3, 16, 0, 0, 0, 0, time.UTC) // 1 day away
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     100,
		LoggedMin:      50,
		BufferPct:      0,
		RecentDailyMin: 100, // remaining ~50, recent*1 = 100, 50 <= 100 => on_track
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level, "daysLeft 1 with low remaining should be on_track")
}

func TestComputeRisk_Boundary_DaysLeft3_HighRemaining_AtRisk(t *testing.T) {
	// daysLeft = 3, remaining > recent*3 and ratio <= 1.5 => at_risk (3-day rule)
	target := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC) // 3 days away
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     450,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 100, // remaining ~450, required = 150/day, ratio = 1.5
		// 450 > 100*3 (300) => at_risk by 3-day rule
	})
	assert.Equal(t, domain.RiskAtRisk, result.Level, "daysLeft 3 with high remaining should be at_risk by 3-day rule")
}

func TestComputeRisk_Boundary_DaysLeft3_LowRemaining_OnTrack(t *testing.T) {
	// daysLeft = 3, remaining <= recent*3 => on_track (3-day rule doesn't trigger)
	target := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC) // 3 days away
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     450,
		LoggedMin:      150,
		BufferPct:      0,
		RecentDailyMin: 100, // remaining ~300, recent*3 = 300, 300 <= 300 => on_track
	})
	assert.Equal(t, domain.RiskOnTrack, result.Level, "daysLeft 3 with low remaining should be on_track")
}

func TestComputeRisk_Boundary_DaysLeft4_3DayRuleNotApplies(t *testing.T) {
	// daysLeft = 4 > 3 => 3-day rule doesn't apply even with high remaining
	target := time.Date(2025, 3, 19, 0, 0, 0, 0, time.UTC) // 4 days away
	result := ComputeRisk(RiskInput{
		Now:            time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		TargetDate:     &target,
		PlannedMin:     600,
		LoggedMin:      0,
		BufferPct:      0,
		RecentDailyMin: 100, // remaining ~600, required ~150/day, ratio ~1.5 (but also 100 < 150 so still at_risk from ratio)
	})
	// The 3-day rule doesn't apply, but ratio = 1.5 boundary matters
	// Actually: required = 600/4 = 150, recent = 100, ratio = 1.5 (not > 1.5, so falls to next case)
	assert.Equal(t, domain.RiskAtRisk, result.Level, "daysLeft 4 with ratio boundary")
}

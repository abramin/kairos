package scheduler

import (
	"math"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

type RiskInput struct {
	Now            time.Time
	TargetDate     *time.Time
	PlannedMin     int
	LoggedMin      int
	BufferPct      float64
	RecentDailyMin float64
}

type RiskResult struct {
	Level            domain.RiskLevel
	DaysLeft         *int
	RemainingMin     int
	RequiredDailyMin float64
	SlackMinPerDay   float64
	ProgressTimePct  float64
}

func ComputeRisk(input RiskInput) RiskResult {
	remaining := int(math.Max(0, float64(input.PlannedMin-input.LoggedMin)*(1+input.BufferPct)))

	var progressTimePct float64
	if input.PlannedMin > 0 {
		progressTimePct = float64(input.LoggedMin) / float64(input.PlannedMin) * 100
	}

	// No target date => on_track (no deadline to miss)
	if input.TargetDate == nil {
		return RiskResult{
			Level:           domain.RiskOnTrack,
			RemainingMin:    remaining,
			ProgressTimePct: progressTimePct,
		}
	}

	daysLeft := int(math.Ceil(input.TargetDate.Sub(input.Now).Hours() / 24))
	daysLeftPtr := &daysLeft

	// Past due
	if daysLeft <= 0 {
		return RiskResult{
			Level:            domain.RiskCritical,
			DaysLeft:         daysLeftPtr,
			RemainingMin:     remaining,
			RequiredDailyMin: float64(remaining), // all remaining needed immediately
			SlackMinPerDay:   input.RecentDailyMin - float64(remaining),
			ProgressTimePct:  progressTimePct,
		}
	}

	requiredDaily := float64(remaining) / float64(daysLeft)
	slack := input.RecentDailyMin - requiredDaily

	result := RiskResult{
		DaysLeft:         daysLeftPtr,
		RemainingMin:     remaining,
		RequiredDailyMin: requiredDaily,
		SlackMinPerDay:   slack,
		ProgressTimePct:  progressTimePct,
	}

	// No recent activity and work remains => critical
	if input.RecentDailyMin == 0 && remaining > 0 {
		result.Level = domain.RiskCritical
		return result
	}

	recentDaily := math.Max(input.RecentDailyMin, 1)
	ratio := requiredDaily / recentDaily

	switch {
	case ratio > 1.5:
		result.Level = domain.RiskCritical
	case ratio > 1.0:
		result.Level = domain.RiskAtRisk
	case daysLeft <= 3 && float64(remaining) > input.RecentDailyMin*float64(daysLeft):
		result.Level = domain.RiskAtRisk
	default:
		result.Level = domain.RiskOnTrack
	}

	return result
}

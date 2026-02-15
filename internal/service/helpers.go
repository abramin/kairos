package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

// projectMetrics holds aggregated work-item data for a single project.
type projectMetrics struct {
	PlannedMin          int
	LoggedMin           int
	DoneCount           int
	TotalCount          int
	DonePlannedMin      int
	ProgressPct         float64
	TimeElapsedPct      float64
	DueBasedExpectedPct float64
}

// aggregateProjectMetrics computes totals and progress percentages from a project's work items.
func aggregateProjectMetrics(items []*domain.WorkItem, project *domain.Project, now time.Time) projectMetrics {
	var m projectMetrics
	for _, item := range items {
		if item.Status == domain.WorkItemArchived {
			continue
		}
		m.TotalCount++
		m.PlannedMin += item.PlannedMin
		m.LoggedMin += item.EffectiveLoggedMin()
		if item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
			m.DoneCount++
			m.DonePlannedMin += item.PlannedMin
		}
	}

	if m.PlannedMin > 0 {
		m.ProgressPct = float64(m.DonePlannedMin) / float64(m.PlannedMin) * 100
	}

	if project.TargetDate != nil {
		totalDays := project.TargetDate.Sub(project.StartDate).Hours() / 24
		elapsedDays := now.Sub(project.StartDate).Hours() / 24
		if totalDays > 0 {
			m.TimeElapsedPct = elapsedDays / totalDays * 100
		}
	}

	var dueByNowMin int
	for _, item := range items {
		if item.Status == domain.WorkItemArchived || item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
			continue
		}
		effectiveDue := item.DueDate
		if effectiveDue == nil {
			effectiveDue = project.TargetDate
		}
		if effectiveDue != nil && !effectiveDue.After(now) {
			dueByNowMin += item.PlannedMin
		}
	}
	if m.PlannedMin > 0 {
		m.DueBasedExpectedPct = float64(m.DonePlannedMin+dueByNowMin) / float64(m.PlannedMin) * 100
	}

	return m
}

// buildRiskInput constructs a RiskInput from pre-computed metrics.
func buildRiskInput(m projectMetrics, targetDate *time.Time, bufferPct float64, effectiveDailyMin float64, now time.Time) scheduler.RiskInput {
	return scheduler.RiskInput{
		Now:                 now,
		TargetDate:          targetDate,
		PlannedMin:          m.PlannedMin,
		LoggedMin:           m.LoggedMin,
		BufferPct:           bufferPct,
		RecentDailyMin:      effectiveDailyMin,
		ProgressPct:         m.ProgressPct,
		TimeElapsedPct:      m.TimeElapsedPct,
		DueBasedExpectedPct: m.DueBasedExpectedPct,
	}
}

// recentDailyPace computes the recent daily pace and effective daily pace from sessions.
func recentDailyPace(sessions []*domain.WorkSessionLog, days int, baselineDailyMin int) (recentDailyMin, effectiveDailyMin float64) {
	var totalMin int
	for _, sess := range sessions {
		totalMin += sess.Minutes
	}
	recentDailyMin = float64(totalMin) / float64(days)
	effectiveDailyMin = math.Max(recentDailyMin, float64(baselineDailyMin))
	return
}

// projectRiskSnapshot holds the computed risk and metrics for a single project.
type projectRiskSnapshot struct {
	Metrics           projectMetrics
	Risk              scheduler.RiskResult
	RecentDailyMin    float64
	EffectiveDailyMin float64
}

// computeProjectRiskSnapshot loads items and sessions, then computes risk for a single project.
// This is the shared core used by both GetStatus and Replan.
func computeProjectRiskSnapshot(
	ctx context.Context,
	p *domain.Project,
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	profile *domain.UserProfile,
	days int,
	now time.Time,
) (*projectRiskSnapshot, []*domain.WorkItem, error) {
	items, err := workItems.ListByProject(ctx, p.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading work items for project %s: %w", p.ID, err)
	}

	m := aggregateProjectMetrics(items, p, now)

	recentSessions, err := sessions.ListRecentByProject(ctx, p.ID, days)
	if err != nil {
		return nil, nil, fmt.Errorf("loading recent sessions for project %s: %w", p.ID, err)
	}
	recentDailyMin, effectiveDailyMin := recentDailyPace(recentSessions, days, profile.BaselineDailyMin)

	risk := scheduler.ComputeRisk(buildRiskInput(m, p.TargetDate, profile.BufferPct, effectiveDailyMin, now))

	return &projectRiskSnapshot{
		Metrics:           m,
		Risk:              risk,
		RecentDailyMin:    recentDailyMin,
		EffectiveDailyMin: effectiveDailyMin,
	}, items, nil
}

// filterByScope returns only items whose ID (extracted by getID) is in scope.
// If scope is empty, all items are returned unchanged.
func filterByScope[T any](items []T, scope []string, getID func(T) string) []T {
	if len(scope) == 0 {
		return items
	}
	scopeSet := make(map[string]bool, len(scope))
	for _, id := range scope {
		scopeSet[id] = true
	}
	var filtered []T
	for _, item := range items {
		if scopeSet[getID(item)] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filterProjectsByScope returns only projects whose ID is in scope.
func filterProjectsByScope(projects []*domain.Project, scope []string) []*domain.Project {
	return filterByScope(projects, scope, func(p *domain.Project) string { return p.ID })
}

// filterCandidatesByScope returns only candidates whose ProjectID is in scope.
func filterCandidatesByScope(candidates []repository.SchedulableCandidate, scope []string) []repository.SchedulableCandidate {
	return filterByScope(candidates, scope, func(c repository.SchedulableCandidate) string { return c.ProjectID })
}

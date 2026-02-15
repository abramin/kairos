package service

import (
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

// filterProjectsByScope returns only projects whose ID is in scope.
// If scope is empty, all projects are returned unchanged.
func filterProjectsByScope(projects []*domain.Project, scope []string) []*domain.Project {
	if len(scope) == 0 {
		return projects
	}
	scopeSet := make(map[string]bool, len(scope))
	for _, id := range scope {
		scopeSet[id] = true
	}
	var filtered []*domain.Project
	for _, p := range projects {
		if scopeSet[p.ID] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// filterCandidatesByScope returns only candidates whose ProjectID is in scope.
// If scope is empty, all candidates are returned unchanged.
func filterCandidatesByScope(candidates []repository.SchedulableCandidate, scope []string) []repository.SchedulableCandidate {
	if len(scope) == 0 {
		return candidates
	}
	scopeSet := make(map[string]bool, len(scope))
	for _, id := range scope {
		scopeSet[id] = true
	}
	var filtered []repository.SchedulableCandidate
	for _, c := range candidates {
		if scopeSet[c.ProjectID] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

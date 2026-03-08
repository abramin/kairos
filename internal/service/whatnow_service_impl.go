package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

type whatNowService struct {
	loader   *ContextLoader
	resolver *BlockResolver
	habits   repository.HabitRepo
	observer UseCaseObserver
}

func NewWhatNowService(
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	deps repository.DependencyRepo,
	profiles repository.UserProfileRepo,
	habits repository.HabitRepo,
	observers ...UseCaseObserver,
) WhatNowService {
	return &whatNowService{
		loader: &ContextLoader{
			workItems: workItems,
			sessions:  sessions,
			profiles:  profiles,
		},
		resolver: &BlockResolver{deps: deps},
		habits:   habits,
		observer: useCaseObserverOrNoop(observers),
	}
}

func (s *whatNowService) Recommend(ctx context.Context, req app.WhatNowRequest) (resp *app.WhatNowResponse, err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"available_min":     req.AvailableMin,
		"enforce_variation": req.EnforceVariation,
	}
	defer func() {
		if resp != nil {
			fields["recommendation_count"] = len(resp.Recommendations)
			fields["blocker_count"] = len(resp.Blockers)
			fields["mode"] = string(resp.Mode)
		}
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "what-now",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	maxSlices := req.MaxSlices
	if maxSlices <= 0 {
		maxSlices = 3
	}
	fields["max_slices"] = maxSlices

	var rctx *RecommendationContext
	rctx, err = s.loader.Load(ctx, req)
	if err != nil {
		return nil, err
	}

	agg := ComputeAggregates(rctx)
	mode := DetermineMode(agg)

	var unblocked []repository.SchedulableCandidate
	var blockers []app.ConstraintBlocker
	unblocked, blockers, err = s.resolver.Resolve(ctx, rctx.Candidates, rctx.Now)
	if err != nil {
		return nil, err
	}

	scored := ScoreCandidates(unblocked, rctx.RecentSessions, agg, rctx.Weights, mode, rctx.Now)
	scheduler.CanonicalSort(scored)

	slices, allocBlockers := scheduler.AllocateSlices(scored, req.AvailableMin, maxSlices, req.EnforceVariation)
	blockers = append(blockers, allocBlockers...)

	// Merge habit suggestions inline (if habits repo is wired)
	if s.habits != nil {
		habitSlices, _ := scoreAndAllocateHabits(ctx, s.habits, rctx.Now, req.AvailableMin-totalAllocated(slices))
		slices = mergeHabitSlices(slices, habitSlices, maxSlices)
	}

	resp = AssembleResponse(rctx.Now, mode, req.AvailableMin, slices, blockers, agg)
	return resp, nil
}

// totalAllocated sums allocated minutes across all slices.
func totalAllocated(slices []app.WorkSlice) int {
	total := 0
	for _, s := range slices {
		total += s.AllocatedMin
	}
	return total
}

// scoreAndAllocateHabits loads active habits, scores them by cadence compliance,
// and returns WorkSlice entries for any that are due. Habits already done today are excluded.
func scoreAndAllocateHabits(ctx context.Context, repo repository.HabitRepo, now time.Time, remainingMin int) ([]app.WorkSlice, error) {
	habits, err := repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	today := truncateToDay(now)
	var slices []app.WorkSlice

	for _, h := range habits {
		lastLog, err := repo.LastLog(ctx, h.ID)
		if err != nil {
			continue
		}

		daysSince := 9999
		if lastLog != nil {
			lastDay := truncateToDay(lastLog.PerformedAt)
			daysSince = int(today.Sub(lastDay).Hours() / 24)
		}

		// Exclude habits already done today
		if daysSince == 0 {
			continue
		}

		daysUntilDue := h.CadenceDays - daysSince

		// Only include habits that are due or due tomorrow
		if daysUntilDue > 1 {
			continue
		}

		// Score: more overdue = higher score
		score := 8.0
		if daysUntilDue <= 0 {
			score = 20.0 + float64(-daysUntilDue)*5.0
		}

		// Build reason message
		var reasonMsg string
		switch {
		case daysSince == 9999:
			reasonMsg = "Never logged — start today!"
		case daysUntilDue < 0:
			reasonMsg = fmt.Sprintf("Overdue by %d day(s) (every %d days)", -daysUntilDue, h.CadenceDays)
		case daysUntilDue == 0:
			reasonMsg = fmt.Sprintf("Due today (every %d days)", h.CadenceDays)
		default:
			reasonMsg = fmt.Sprintf("Due tomorrow (every %d days)", h.CadenceDays)
		}

		minS := h.MinSessionMin
		maxS := h.MaxSessionMin
		target := h.TargetMin
		if minS <= 0 {
			minS = 5
		}
		if maxS <= 0 {
			maxS = target + 10
		}

		if remainingMin < minS {
			continue
		}

		upper := min(maxS, remainingMin)
		allocated := clampInt(target, minS, upper)
		if allocated <= 0 {
			continue
		}

		slices = append(slices, app.WorkSlice{
			HabitID:       h.ID,
			IsHabit:       true,
			Title:         h.Title,
			AllocatedMin:  allocated,
			MinSessionMin: minS,
			MaxSessionMin: maxS,
			DefaultSessionMin: target,
			Score:         score,
			CadenceDays:   h.CadenceDays,
			DaysSinceLog:  daysSince,
			Reasons: []app.RecommendationReason{
				{Code: "HABIT_CADENCE", Message: reasonMsg},
			},
		})
		remainingMin -= allocated
	}

	return slices, nil
}

// mergeHabitSlices inserts habit slices into the recommendation list sorted by score,
// respecting the maxSlices cap. Habits compete on score with work items.
func mergeHabitSlices(workSlices, habitSlices []app.WorkSlice, maxSlices int) []app.WorkSlice {
	combined := make([]app.WorkSlice, 0, len(workSlices)+len(habitSlices))
	combined = append(combined, workSlices...)
	combined = append(combined, habitSlices...)

	// Stable sort by score descending, habits treated equally to work items
	for i := 1; i < len(combined); i++ {
		for j := i; j > 0 && combined[j].Score > combined[j-1].Score; j-- {
			combined[j], combined[j-1] = combined[j-1], combined[j]
		}
	}

	if len(combined) > maxSlices {
		combined = combined[:maxSlices]
	}
	return combined
}

func clampInt(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// --- Internal types and helpers used by ComputeAggregates ---

// projectAggregates holds per-project computed data (internal to the risk computation).
type projectAggregates struct {
	risks      map[string]scheduler.RiskResult
	names      map[string]string
	planned    map[string]int
	logged     map[string]int
	recentMin  map[string]int
	targetDate map[string]*time.Time
	startDate  map[string]*time.Time
}

// projectIndex holds intermediate per-project data used to compute risks.
type projectIndex struct {
	dueByNow           map[string]int
	completedByProject map[string]repository.CompletedWorkSummary
}

// buildProjectIndex accumulates per-project totals and indexes from raw data.
func buildProjectIndex(
	candidates []repository.SchedulableCandidate,
	completedSummaries []repository.CompletedWorkSummary,
	recentSessions []*domain.WorkSessionLog,
	now time.Time,
) (projectAggregates, projectIndex) {
	agg := projectAggregates{
		risks:      make(map[string]scheduler.RiskResult),
		names:      make(map[string]string),
		planned:    make(map[string]int),
		logged:     make(map[string]int),
		recentMin:  make(map[string]int),
		targetDate: make(map[string]*time.Time),
		startDate:  make(map[string]*time.Time),
	}

	workItemToProject := make(map[string]string, len(candidates))
	dueByNow := make(map[string]int)
	for _, c := range candidates {
		agg.planned[c.ProjectID] += c.WorkItem.PlannedMin
		agg.logged[c.ProjectID] += c.WorkItem.LoggedMin
		agg.names[c.ProjectID] = c.ProjectName
		if c.ProjectTargetDate != nil {
			agg.targetDate[c.ProjectID] = c.ProjectTargetDate
		}
		if c.ProjectStartDate != nil {
			agg.startDate[c.ProjectID] = c.ProjectStartDate
		}
		workItemToProject[c.WorkItem.ID] = c.ProjectID

		effectiveDue := earliestDueDate(c.WorkItem.DueDate, c.NodeDueDate, c.ProjectTargetDate)
		if effectiveDue != nil && !effectiveDue.After(now) {
			dueByNow[c.ProjectID] += c.WorkItem.PlannedMin
		}
	}

	completedByProject := make(map[string]repository.CompletedWorkSummary, len(completedSummaries))
	for _, cs := range completedSummaries {
		completedByProject[cs.ProjectID] = cs
	}

	for _, sess := range recentSessions {
		if pid, ok := workItemToProject[sess.WorkItemID]; ok {
			agg.recentMin[pid] += sess.Minutes
		}
	}

	return agg, projectIndex{dueByNow: dueByNow, completedByProject: completedByProject}
}

// computeProjectRisks computes risk levels for each project using timeline math.
func computeProjectRisks(agg *projectAggregates, idx projectIndex, now time.Time, bufferPct float64, baselineDailyMin int) {
	for pid := range agg.planned {
		cs := idx.completedByProject[pid]

		allPlanned := agg.planned[pid] + cs.PlannedMin
		var progressPct float64
		if allPlanned > 0 {
			progressPct = float64(cs.PlannedMin) / float64(allPlanned) * 100
		}

		var timeElapsedPct float64
		if agg.startDate[pid] != nil && agg.targetDate[pid] != nil {
			totalDays := agg.targetDate[pid].Sub(*agg.startDate[pid]).Hours() / 24
			elapsedDays := now.Sub(*agg.startDate[pid]).Hours() / 24
			if totalDays > 0 {
				timeElapsedPct = elapsedDays / totalDays * 100
			}
		}

		expectedDoneMin := cs.PlannedMin + idx.dueByNow[pid]
		var dueBasedExpectedPct float64
		if allPlanned > 0 {
			dueBasedExpectedPct = float64(expectedDoneMin) / float64(allPlanned) * 100
		}

		recentDaily := float64(agg.recentMin[pid]) / 7.0
		effectiveDaily := math.Max(recentDaily, float64(baselineDailyMin))
		agg.risks[pid] = scheduler.ComputeRisk(scheduler.RiskInput{
			Now:                 now,
			TargetDate:          agg.targetDate[pid],
			PlannedMin:          agg.planned[pid],
			LoggedMin:           agg.logged[pid],
			BufferPct:           bufferPct,
			RecentDailyMin:      effectiveDaily,
			ProgressPct:         progressPct,
			TimeElapsedPct:      timeElapsedPct,
			DueBasedExpectedPct: dueBasedExpectedPct,
		})
	}
}

// earliestDueDate returns the earliest non-nil date from the given pointers.
func earliestDueDate(dates ...*time.Time) *time.Time {
	var earliest *time.Time
	for _, d := range dates {
		if d != nil && (earliest == nil || d.Before(*earliest)) {
			earliest = d
		}
	}
	return earliest
}

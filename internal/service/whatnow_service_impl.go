package service

import (
	"context"
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

	// Promote habits to first-class scoring candidates
	if s.habits != nil {
		habitScored := s.scoreHabitCandidates(ctx, rctx.Now, rctx.Weights, mode)
		scored = append(scored, habitScored...)
	}

	scheduler.CanonicalSort(scored)

	slices, allocBlockers := scheduler.AllocateSlices(scored, req.AvailableMin, maxSlices, req.EnforceVariation)
	blockers = append(blockers, allocBlockers...)

	resp = AssembleResponse(rctx.Now, mode, req.AvailableMin, slices, blockers, agg)
	return resp, nil
}

// scoreHabitCandidates converts active habits into ScoringInput candidates and scores
// them through the main scorer pipeline. Habits already done today are excluded.
func (s *whatNowService) scoreHabitCandidates(
	ctx context.Context,
	now time.Time,
	weights scheduler.ScoringWeights,
	mode domain.PlanMode,
) []scheduler.ScoredCandidate {
	habits, err := s.habits.ListActive(ctx)
	if err != nil {
		return nil
	}

	today := truncateToDay(now)
	var scored []scheduler.ScoredCandidate

	for _, h := range habits {
		lastLog, err := s.habits.LastLog(ctx, h.ID)
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

		// Only include habits approaching due or overdue
		// (scoreHabitUrgency handles suppression for fraction < 0.8)
		daysUntilDue := h.CadenceDays - daysSince
		if daysUntilDue > 1 {
			continue
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

		input := scheduler.ScoringInput{
			WorkItemID:        "habit:" + h.ID,
			ProjectID:         "habit:" + h.ID,
			ProjectName:       h.Title,
			Title:             h.Title,
			Now:               now,
			Weights:           weights,
			Mode:              mode,
			MinSessionMin:     minS,
			MaxSessionMin:     maxS,
			DefaultSessionMin: target,
			IsHabit:           true,
			HabitID:           h.ID,
			HabitCadenceDays:  h.CadenceDays,
			HabitDaysSince:    daysSince,
		}

		scored = append(scored, scheduler.ScoreWorkItem(input))
	}

	return scored
}

// --- Internal types and helpers used by ComputeAggregates ---

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
) (ProjectAggregates, projectIndex) {
	agg := ProjectAggregates{
		Risks:      make(map[string]scheduler.RiskResult),
		Names:      make(map[string]string),
		Planned:    make(map[string]int),
		Logged:     make(map[string]int),
		RecentMin:  make(map[string]int),
		TargetDate: make(map[string]*time.Time),
		StartDate:  make(map[string]*time.Time),
	}

	workItemToProject := make(map[string]string, len(candidates))
	dueByNow := make(map[string]int)
	for _, c := range candidates {
		agg.Planned[c.ProjectID] += c.WorkItem.PlannedMin
		agg.Logged[c.ProjectID] += c.WorkItem.LoggedMin
		agg.Names[c.ProjectID] = c.ProjectName
		if c.ProjectTargetDate != nil {
			agg.TargetDate[c.ProjectID] = c.ProjectTargetDate
		}
		if c.ProjectStartDate != nil {
			agg.StartDate[c.ProjectID] = c.ProjectStartDate
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
			agg.RecentMin[pid] += sess.Minutes
		}
	}

	return agg, projectIndex{dueByNow: dueByNow, completedByProject: completedByProject}
}

// computeProjectRisks computes risk levels for each project using timeline math.
func computeProjectRisks(agg *ProjectAggregates, idx projectIndex, now time.Time, bufferPct float64, baselineDailyMin int) {
	for pid := range agg.Planned {
		cs := idx.completedByProject[pid]

		allPlanned := agg.Planned[pid] + cs.PlannedMin
		var progressPct float64
		if allPlanned > 0 {
			progressPct = float64(cs.PlannedMin) / float64(allPlanned) * 100
		}

		var timeElapsedPct float64
		if agg.StartDate[pid] != nil && agg.TargetDate[pid] != nil {
			totalDays := agg.TargetDate[pid].Sub(*agg.StartDate[pid]).Hours() / 24
			elapsedDays := now.Sub(*agg.StartDate[pid]).Hours() / 24
			if totalDays > 0 {
				timeElapsedPct = elapsedDays / totalDays * 100
			}
		}

		expectedDoneMin := cs.PlannedMin + idx.dueByNow[pid]
		var dueBasedExpectedPct float64
		if allPlanned > 0 {
			dueBasedExpectedPct = float64(expectedDoneMin) / float64(allPlanned) * 100
		}

		recentDaily := float64(agg.RecentMin[pid]) / 7.0
		effectiveDaily := math.Max(recentDaily, float64(baselineDailyMin))
		agg.Risks[pid] = scheduler.ComputeRisk(scheduler.RiskInput{
			Now:                 now,
			TargetDate:          agg.TargetDate[pid],
			PlannedMin:          agg.Planned[pid],
			LoggedMin:           agg.Logged[pid],
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

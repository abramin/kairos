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
}

func NewWhatNowService(
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	deps repository.DependencyRepo,
	profiles repository.UserProfileRepo,
) WhatNowService {
	return &whatNowService{
		loader: &ContextLoader{
			workItems: workItems,
			sessions:  sessions,
			profiles:  profiles,
		},
		resolver: &BlockResolver{deps: deps},
	}
}

func (s *whatNowService) Recommend(ctx context.Context, req app.WhatNowRequest) (*app.WhatNowResponse, error) {
	maxSlices := req.MaxSlices
	if maxSlices <= 0 {
		maxSlices = 3
	}

	rctx, err := s.loader.Load(ctx, req)
	if err != nil {
		return nil, err
	}

	agg := ComputeAggregates(rctx)
	mode := DetermineMode(agg)

	unblocked, blockers, err := s.resolver.Resolve(ctx, rctx.Candidates, rctx.Now)
	if err != nil {
		return nil, err
	}

	scored := ScoreCandidates(unblocked, rctx.RecentSessions, agg, rctx.Weights, mode, rctx.Now)
	scheduler.CanonicalSort(scored)

	slices, allocBlockers := scheduler.AllocateSlices(scored, req.AvailableMin, maxSlices, req.EnforceVariation)
	blockers = append(blockers, allocBlockers...)

	return AssembleResponse(rctx.Now, mode, req.AvailableMin, slices, blockers, agg), nil
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

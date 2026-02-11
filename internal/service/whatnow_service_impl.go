package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

type whatNowService struct {
	workItems repository.WorkItemRepo
	sessions  repository.SessionRepo
	projects  repository.ProjectRepo
	deps      repository.DependencyRepo
	profiles  repository.UserProfileRepo
}

func NewWhatNowService(
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	projects repository.ProjectRepo,
	deps repository.DependencyRepo,
	profiles repository.UserProfileRepo,
) WhatNowService {
	return &whatNowService{
		workItems: workItems,
		sessions:  sessions,
		projects:  projects,
		deps:      deps,
		profiles:  profiles,
	}
}

// projectAggregates holds per-project computed data shared across recommendation phases.
type projectAggregates struct {
	risks      map[string]scheduler.RiskResult
	names      map[string]string
	planned    map[string]int
	logged     map[string]int
	recentMin  map[string]int
	targetDate map[string]*time.Time
	startDate  map[string]*time.Time
}

func (s *whatNowService) Recommend(ctx context.Context, req contract.WhatNowRequest) (*contract.WhatNowResponse, error) {
	if req.AvailableMin <= 0 {
		return nil, &contract.WhatNowError{
			Code:    contract.ErrInvalidAvailableMin,
			Message: "available_min must be > 0",
		}
	}

	now := time.Now().UTC()
	if req.Now != nil {
		now = *req.Now
	}
	maxSlices := req.MaxSlices
	if maxSlices <= 0 {
		maxSlices = 3
	}

	profile, err := s.profiles.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading user profile: %w", err)
	}
	weights := scheduler.ScoringWeights{
		DeadlinePressure: profile.WeightDeadlinePressure,
		BehindPace:       profile.WeightBehindPace,
		Spacing:          profile.WeightSpacing,
		Variation:        profile.WeightVariation,
	}

	candidates, err := s.workItems.ListSchedulable(ctx, req.IncludeArchived)
	if err != nil {
		return nil, fmt.Errorf("loading schedulable items: %w", err)
	}
	candidates = filterCandidatesByScope(candidates, req.ProjectScope)
	if len(candidates) == 0 {
		return nil, &contract.WhatNowError{
			Code:    contract.ErrNoCandidates,
			Message: "no schedulable work items found",
		}
	}

	recentSessions, err := s.sessions.ListRecent(ctx, 7)
	if err != nil {
		return nil, fmt.Errorf("loading recent sessions: %w", err)
	}

	completedSummaries, err := s.workItems.ListCompletedSummaryByProject(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading completed work summaries: %w", err)
	}

	agg := s.computeProjectAggregates(candidates, completedSummaries, recentSessions, now, profile.BufferPct, profile.BaselineDailyMin)

	mode := domain.ModeBalanced
	for _, risk := range agg.risks {
		if risk.Level == domain.RiskCritical {
			mode = domain.ModeCritical
			break
		}
	}

	scored, blockers, err := s.scoreCandidates(ctx, candidates, recentSessions, agg, weights, mode, now)
	if err != nil {
		return nil, err
	}

	scheduler.CanonicalSort(scored)
	slices, allocBlockers := scheduler.AllocateSlices(scored, req.AvailableMin, maxSlices, req.EnforceVariation)
	blockers = append(blockers, allocBlockers...)

	return s.buildResponse(now, mode, req.AvailableMin, slices, blockers, agg), nil
}

// projectIndex holds intermediate per-project data used to compute risks.
type projectIndex struct {
	dueByNow           map[string]int
	completedByProject map[string]repository.CompletedWorkSummary
}

// computeProjectAggregates builds per-project risk, totals, and recent session data from candidates.
func (s *whatNowService) computeProjectAggregates(
	candidates []repository.SchedulableCandidate,
	completedSummaries []repository.CompletedWorkSummary,
	recentSessions []*domain.WorkSessionLog,
	now time.Time,
	bufferPct float64,
	baselineDailyMin int,
) projectAggregates {
	agg, idx := buildProjectIndex(candidates, completedSummaries, recentSessions, now)
	computeProjectRisks(&agg, idx, now, bufferPct, baselineDailyMin)
	return agg
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

	// Build work-item-to-project index for O(1) session lookups
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

		// Weighted structural progress: done planned_min / all planned_min.
		allPlanned := agg.planned[pid] + cs.PlannedMin
		var progressPct float64
		if allPlanned > 0 {
			progressPct = float64(cs.PlannedMin) / float64(allPlanned) * 100
		}

		// Timeline elapsed: (now - start) / (target - start)
		var timeElapsedPct float64
		if agg.startDate[pid] != nil && agg.targetDate[pid] != nil {
			totalDays := agg.targetDate[pid].Sub(*agg.startDate[pid]).Hours() / 24
			elapsedDays := now.Sub(*agg.startDate[pid]).Hours() / 24
			if totalDays > 0 {
				timeElapsedPct = elapsedDays / totalDays * 100
			}
		}

		// Due-date-aware expected progress: what % of total work should be done by now?
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

// scoreCandidates checks constraints and scores each candidate, returning scored items and blockers.
func (s *whatNowService) scoreCandidates(
	ctx context.Context,
	candidates []repository.SchedulableCandidate,
	recentSessions []*domain.WorkSessionLog,
	agg projectAggregates,
	weights scheduler.ScoringWeights,
	mode domain.PlanMode,
	now time.Time,
) ([]scheduler.ScoredCandidate, []contract.ConstraintBlocker, error) {
	// Build last-session-days-ago per work item
	lastSessionDaysAgo := make(map[string]*int)
	for _, sess := range recentSessions {
		daysAgo := int(now.Sub(sess.StartedAt).Hours() / 24)
		if existing, ok := lastSessionDaysAgo[sess.WorkItemID]; !ok || (existing != nil && daysAgo < *existing) {
			d := daysAgo
			lastSessionDaysAgo[sess.WorkItemID] = &d
		}
	}

	var blockers []contract.ConstraintBlocker
	var scored []scheduler.ScoredCandidate

	for _, c := range candidates {
		blocked, err := s.deps.HasUnfinishedPredecessors(ctx, c.WorkItem.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("checking dependencies: %w", err)
		}
		if blocked {
			blockers = append(blockers, contract.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       contract.BlockerDependency,
				Message:    fmt.Sprintf("Work item '%s' has unfinished predecessors", c.WorkItem.Title),
			})
			continue
		}

		if c.WorkItem.NotBefore != nil && now.Before(*c.WorkItem.NotBefore) {
			blockers = append(blockers, contract.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       contract.BlockerNotBefore,
				Message:    fmt.Sprintf("Work item '%s' not available before %s", c.WorkItem.Title, c.WorkItem.NotBefore.Format("2006-01-02")),
			})
			continue
		}

		effectiveDue := earliestDueDate(c.WorkItem.DueDate, c.NodeDueDate, c.ProjectTargetDate)

		input := scheduler.ScoringInput{
			WorkItemID:          c.WorkItem.ID,
			WorkItemSeq:         c.WorkItem.Seq,
			ProjectID:           c.ProjectID,
			ProjectName:         c.ProjectName,
			NodeTitle:           c.NodeTitle,
			Title:               c.WorkItem.Title,
			DueDate:             effectiveDue,
			ProjectRisk:         agg.risks[c.ProjectID].Level,
			Now:                 now,
			LastSessionDaysAgo:  lastSessionDaysAgo[c.WorkItem.ID],
			ProjectSlicesInPlan: 0, // variation is enforced by the allocator's two-pass approach
			Weights:             weights,
			Mode:                mode,
			Status:              c.WorkItem.Status,
			MinSessionMin:       c.WorkItem.MinSessionMin,
			MaxSessionMin:       c.WorkItem.MaxSessionMin,
			DefaultSessionMin:   c.WorkItem.DefaultSessionMin,
			Splittable:          c.WorkItem.Splittable,
			PlannedMin:          c.WorkItem.PlannedMin,
			LoggedMin:           c.WorkItem.LoggedMin,
			NodeID:              c.WorkItem.NodeID,
		}

		scored = append(scored, scheduler.ScoreWorkItem(input))
	}

	return scored, blockers, nil
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

// buildResponse assembles the final WhatNowResponse from scored slices and project aggregates.
func (s *whatNowService) buildResponse(
	now time.Time,
	mode domain.PlanMode,
	requestedMin int,
	slices []contract.WorkSlice,
	blockers []contract.ConstraintBlocker,
	agg projectAggregates,
) *contract.WhatNowResponse {
	var riskSummaries []contract.RiskSummary
	for pid, risk := range agg.risks {
		var dueDateStr *string
		if agg.targetDate[pid] != nil {
			ds := agg.targetDate[pid].Format("2006-01-02")
			dueDateStr = &ds
		}
		recentDaily := float64(agg.recentMin[pid]) / 7.0
		riskSummaries = append(riskSummaries, contract.RiskSummary{
			ProjectID:         pid,
			ProjectName:       agg.names[pid],
			RiskLevel:         risk.Level,
			DueDate:           dueDateStr,
			DaysLeft:          risk.DaysLeft,
			PlannedMinTotal:   agg.planned[pid],
			LoggedMinTotal:    agg.logged[pid],
			RemainingMinTotal: risk.RemainingMin,
			RequiredDailyMin:  risk.RequiredDailyMin,
			RecentDailyMin:    recentDaily,
			SlackMinPerDay:    risk.SlackMinPerDay,
			ProgressTimePct:   risk.ProgressTimePct,
		})
	}

	allocatedMin := 0
	for _, sl := range slices {
		allocatedMin += sl.AllocatedMin
	}

	var policyMessages []string
	for pid, risk := range agg.risks {
		if risk.Level == domain.RiskOnTrack {
			policyMessages = append(policyMessages, fmt.Sprintf("%s is on track, secondary work is safe", agg.names[pid]))
		}
	}

	return &contract.WhatNowResponse{
		GeneratedAt:     now,
		Mode:            mode,
		RequestedMin:    requestedMin,
		AllocatedMin:    allocatedMin,
		UnallocatedMin:  requestedMin - allocatedMin,
		Recommendations: slices,
		Blockers:        blockers,
		TopRiskProjects: riskSummaries,
		PolicyMessages:  policyMessages,
	}
}

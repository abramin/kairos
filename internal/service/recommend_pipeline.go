package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

// ProjectAggregates holds per-project computed data shared across recommendation phases.
type ProjectAggregates struct {
	Risks      map[string]scheduler.RiskResult
	Names      map[string]string
	Planned    map[string]int
	Logged     map[string]int
	RecentMin  map[string]int
	TargetDate map[string]*time.Time
	StartDate  map[string]*time.Time
}

// RecommendationContext bundles all data loaded for a recommendation cycle.
type RecommendationContext struct {
	Now                time.Time
	Candidates         []repository.SchedulableCandidate
	RecentSessions     []*domain.WorkSessionLog
	CompletedSummaries []repository.CompletedWorkSummary
	Weights            scheduler.ScoringWeights
	BufferPct          float64
	BaselineDailyMin   int
}

// ContextLoader loads all data needed for a recommendation cycle.
type ContextLoader struct {
	workItems repository.WorkItemRepo
	sessions  repository.SessionRepo
	profiles  repository.UserProfileRepo
}

// Load validates the request and loads candidates, sessions, profile, and summaries.
func (cl *ContextLoader) Load(ctx context.Context, req app.WhatNowRequest) (*RecommendationContext, error) {
	if req.AvailableMin <= 0 {
		return nil, &app.WhatNowError{
			Code:    app.ErrInvalidAvailableMin,
			Message: "available_min must be > 0",
		}
	}

	now := time.Now().UTC()
	if req.Now != nil {
		now = *req.Now
	}

	profile, err := cl.profiles.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading user profile: %w", err)
	}

	candidates, err := cl.workItems.ListSchedulable(ctx, req.IncludeArchived)
	if err != nil {
		return nil, fmt.Errorf("loading schedulable items: %w", err)
	}
	candidates = filterCandidatesByScope(candidates, req.ProjectScope)
	if len(candidates) == 0 {
		return nil, &app.WhatNowError{
			Code:    app.ErrNoCandidates,
			Message: "no schedulable work items found",
		}
	}

	recentSessions, err := cl.sessions.ListRecent(ctx, 7)
	if err != nil {
		return nil, fmt.Errorf("loading recent sessions: %w", err)
	}

	completedSummaries, err := cl.workItems.ListCompletedSummaryByProject(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading completed work summaries: %w", err)
	}

	return &RecommendationContext{
		Now:                now,
		Candidates:         candidates,
		RecentSessions:     recentSessions,
		CompletedSummaries: completedSummaries,
		Weights: scheduler.ScoringWeights{
			DeadlinePressure: profile.WeightDeadlinePressure,
			BehindPace:       profile.WeightBehindPace,
			Spacing:          profile.WeightSpacing,
			Variation:        profile.WeightVariation,
		},
		BufferPct:        profile.BufferPct,
		BaselineDailyMin: profile.BaselineDailyMin,
	}, nil
}

// ComputeAggregates builds per-project risk, totals, and recent session data.
func ComputeAggregates(rctx *RecommendationContext) ProjectAggregates {
	agg, idx := buildProjectIndex(rctx.Candidates, rctx.CompletedSummaries, rctx.RecentSessions, rctx.Now)
	computeProjectRisks(&agg, idx, rctx.Now, rctx.BufferPct, rctx.BaselineDailyMin)
	return ProjectAggregates{
		Risks:      agg.risks,
		Names:      agg.names,
		Planned:    agg.planned,
		Logged:     agg.logged,
		RecentMin:  agg.recentMin,
		TargetDate: agg.targetDate,
		StartDate:  agg.startDate,
	}
}

// DetermineMode returns Critical if any project has critical risk, otherwise Balanced.
func DetermineMode(agg ProjectAggregates) domain.PlanMode {
	for _, risk := range agg.Risks {
		if risk.Level == domain.RiskCritical {
			return domain.ModeCritical
		}
	}
	return domain.ModeBalanced
}

// BlockResolver resolves dependency and constraint blocks for candidates in batch.
type BlockResolver struct {
	deps repository.DependencyRepo
}

// Resolve checks dependency, NotBefore, and WorkComplete constraints, returning
// unblocked candidates and blockers. Uses a batch dependency query instead of N+1.
func (br *BlockResolver) Resolve(
	ctx context.Context,
	candidates []repository.SchedulableCandidate,
	now time.Time,
) ([]repository.SchedulableCandidate, []app.ConstraintBlocker, error) {
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.WorkItem.ID
	}

	blockedSet, err := br.deps.ListBlockedWorkItemIDs(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("checking dependencies: %w", err)
	}

	var unblocked []repository.SchedulableCandidate
	var blockers []app.ConstraintBlocker

	for _, c := range candidates {
		if blockedSet[c.WorkItem.ID] {
			blockers = append(blockers, app.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       app.BlockerDependency,
				Message:    fmt.Sprintf("Work item '%s' has unfinished predecessors", c.WorkItem.Title),
			})
			continue
		}

		if c.WorkItem.NotBefore != nil && now.Before(*c.WorkItem.NotBefore) {
			blockers = append(blockers, app.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       app.BlockerNotBefore,
				Message:    fmt.Sprintf("Work item '%s' not available before %s", c.WorkItem.Title, c.WorkItem.NotBefore.Format("2006-01-02")),
			})
			continue
		}

		if c.WorkItem.PlannedMin > 0 && c.WorkItem.LoggedMin >= c.WorkItem.PlannedMin {
			blockers = append(blockers, app.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       app.BlockerWorkComplete,
				Message:    fmt.Sprintf("Work item '%s' is fully logged (%dm/%dm)", c.WorkItem.Title, c.WorkItem.LoggedMin, c.WorkItem.PlannedMin),
			})
			continue
		}

		unblocked = append(unblocked, c)
	}

	return unblocked, blockers, nil
}

// ScoreCandidates builds scoring input for each candidate and delegates to scheduler.ScoreWorkItem.
func ScoreCandidates(
	candidates []repository.SchedulableCandidate,
	recentSessions []*domain.WorkSessionLog,
	agg ProjectAggregates,
	weights scheduler.ScoringWeights,
	mode domain.PlanMode,
	now time.Time,
) []scheduler.ScoredCandidate {
	lastSessionDaysAgo := buildLastSessionIndex(recentSessions, now)

	scored := make([]scheduler.ScoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		effectiveDue := earliestDueDate(c.WorkItem.DueDate, c.NodeDueDate, c.ProjectTargetDate)

		var lastSessionPtr *int
		if d, ok := lastSessionDaysAgo[c.WorkItem.ID]; ok {
			lastSessionPtr = &d
		}

		input := scheduler.ScoringInput{
			WorkItemID:          c.WorkItem.ID,
			WorkItemSeq:         c.WorkItem.Seq,
			ProjectID:           c.ProjectID,
			ProjectName:         c.ProjectName,
			NodeTitle:           c.NodeTitle,
			Title:               c.WorkItem.Title,
			DueDate:             effectiveDue,
			ProjectRisk:         agg.Risks[c.ProjectID].Level,
			Now:                 now,
			LastSessionDaysAgo:  lastSessionPtr,
			ProjectSlicesInPlan: 0,
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
	return scored
}

// buildLastSessionIndex computes days-ago-since-last-session per work item.
// Returns a map of work item ID → days ago (only entries for items with sessions).
func buildLastSessionIndex(sessions []*domain.WorkSessionLog, now time.Time) map[string]int {
	lastSessionDaysAgo := make(map[string]int)
	for _, sess := range sessions {
		daysAgo := int(now.Sub(sess.StartedAt).Hours() / 24)
		if existing, ok := lastSessionDaysAgo[sess.WorkItemID]; !ok || daysAgo < existing {
			lastSessionDaysAgo[sess.WorkItemID] = daysAgo
		}
	}
	return lastSessionDaysAgo
}

// AssembleResponse builds the final WhatNowResponse from slices, blockers, and project aggregates.
func AssembleResponse(
	now time.Time,
	mode domain.PlanMode,
	requestedMin int,
	slices []app.WorkSlice,
	blockers []app.ConstraintBlocker,
	agg ProjectAggregates,
) *app.WhatNowResponse {
	var riskSummaries []app.RiskSummary
	for pid, risk := range agg.Risks {
		var dueDateStr *string
		if agg.TargetDate[pid] != nil {
			ds := agg.TargetDate[pid].Format("2006-01-02")
			dueDateStr = &ds
		}
		recentDaily := float64(agg.RecentMin[pid]) / 7.0
		riskSummaries = append(riskSummaries, app.RiskSummary{
			ProjectID:         pid,
			ProjectName:       agg.Names[pid],
			RiskLevel:         risk.Level,
			DueDate:           dueDateStr,
			DaysLeft:          risk.DaysLeft,
			PlannedMinTotal:   agg.Planned[pid],
			LoggedMinTotal:    agg.Logged[pid],
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
	for pid, risk := range agg.Risks {
		if risk.Level == domain.RiskOnTrack {
			policyMessages = append(policyMessages, fmt.Sprintf("%s is on track, secondary work is safe", agg.Names[pid]))
		}
	}

	return &app.WhatNowResponse{
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

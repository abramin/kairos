package service

import (
	"context"
	"fmt"
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

	// Load user profile for weights
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

	// Load all schedulable candidates
	candidates, err := s.workItems.ListSchedulable(ctx, req.IncludeArchived)
	if err != nil {
		return nil, fmt.Errorf("loading schedulable items: %w", err)
	}

	// Filter by project scope if specified
	if len(req.ProjectScope) > 0 {
		scopeSet := make(map[string]bool)
		for _, id := range req.ProjectScope {
			scopeSet[id] = true
		}
		var filtered []repository.SchedulableCandidate
		for _, c := range candidates {
			if scopeSet[c.ProjectID] {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		return nil, &contract.WhatNowError{
			Code:    contract.ErrNoCandidates,
			Message: "no schedulable work items found",
		}
	}

	// Compute risk per project
	projectRisks := make(map[string]scheduler.RiskResult)
	projectNames := make(map[string]string)
	recentSessions, _ := s.sessions.ListRecent(ctx, 7)

	// Aggregate logged minutes per project for recent daily calculation
	projectRecentMin := make(map[string]int)

	// Build project aggregates from candidates
	projectPlanned := make(map[string]int)
	projectLogged := make(map[string]int)
	projectTargetDate := make(map[string]*time.Time)
	for _, c := range candidates {
		projectPlanned[c.ProjectID] += c.WorkItem.PlannedMin
		projectLogged[c.ProjectID] += c.WorkItem.LoggedMin
		projectNames[c.ProjectID] = c.ProjectName
		if c.ProjectTargetDate != nil {
			projectTargetDate[c.ProjectID] = c.ProjectTargetDate
		}
	}

	// Compute recent daily minutes per project from sessions
	for _, sess := range recentSessions {
		// Find the project for this session's work item
		for _, c := range candidates {
			if c.WorkItem.ID == sess.WorkItemID {
				projectRecentMin[c.ProjectID] += sess.Minutes
				break
			}
		}
	}

	// Compute risk for each project
	for pid := range projectPlanned {
		recentDaily := float64(projectRecentMin[pid]) / 7.0
		riskResult := scheduler.ComputeRisk(scheduler.RiskInput{
			Now:            now,
			TargetDate:     projectTargetDate[pid],
			PlannedMin:     projectPlanned[pid],
			LoggedMin:      projectLogged[pid],
			BufferPct:      profile.BufferPct,
			RecentDailyMin: recentDaily,
		})
		projectRisks[pid] = riskResult
	}

	// Determine mode
	mode := domain.ModeBalanced
	for _, risk := range projectRisks {
		if risk.Level == domain.RiskCritical {
			mode = domain.ModeCritical
			break
		}
	}

	// Build last-session-days-ago per work item
	lastSessionDaysAgo := make(map[string]*int)
	for _, sess := range recentSessions {
		daysAgo := int(now.Sub(sess.StartedAt).Hours() / 24)
		if existing, ok := lastSessionDaysAgo[sess.WorkItemID]; !ok || (existing != nil && daysAgo < *existing) {
			d := daysAgo
			lastSessionDaysAgo[sess.WorkItemID] = &d
		}
	}

	// Score each candidate
	var blockers []contract.ConstraintBlocker
	var scored []scheduler.ScoredCandidate
	projectSliceCount := make(map[string]int) // will track during allocation

	for _, c := range candidates {
		// Check dependency blocking
		blocked, err := s.deps.HasUnfinishedPredecessors(ctx, c.WorkItem.ID)
		if err != nil {
			return nil, fmt.Errorf("checking dependencies: %w", err)
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

		// Check not_before constraint
		if c.WorkItem.NotBefore != nil && now.Before(*c.WorkItem.NotBefore) {
			blockers = append(blockers, contract.ConstraintBlocker{
				EntityType: "work_item",
				EntityID:   c.WorkItem.ID,
				Code:       contract.BlockerNotBefore,
				Message:    fmt.Sprintf("Work item '%s' not available before %s", c.WorkItem.Title, c.WorkItem.NotBefore.Format("2006-01-02")),
			})
			continue
		}

		// Determine effective due date (earliest of work item, node, or project)
		var effectiveDue *time.Time
		if c.WorkItem.DueDate != nil {
			effectiveDue = c.WorkItem.DueDate
		}
		if c.NodeDueDate != nil && (effectiveDue == nil || c.NodeDueDate.Before(*effectiveDue)) {
			effectiveDue = c.NodeDueDate
		}
		if c.ProjectTargetDate != nil && (effectiveDue == nil || c.ProjectTargetDate.Before(*effectiveDue)) {
			effectiveDue = c.ProjectTargetDate
		}

		input := scheduler.ScoringInput{
			WorkItemID:          c.WorkItem.ID,
			ProjectID:           c.ProjectID,
			ProjectName:         c.ProjectName,
			NodeTitle:           c.NodeTitle,
			Title:               c.WorkItem.Title,
			DueDate:             effectiveDue,
			ProjectRisk:         projectRisks[c.ProjectID].Level,
			Now:                 now,
			LastSessionDaysAgo:  lastSessionDaysAgo[c.WorkItem.ID],
			ProjectSlicesInPlan: projectSliceCount[c.ProjectID],
			Weights:             weights,
			Mode:                mode,
			MinSessionMin:       c.WorkItem.MinSessionMin,
			MaxSessionMin:       c.WorkItem.MaxSessionMin,
			DefaultSessionMin:   c.WorkItem.DefaultSessionMin,
			Splittable:          c.WorkItem.Splittable,
			PlannedMin:          c.WorkItem.PlannedMin,
			LoggedMin:           c.WorkItem.LoggedMin,
			NodeID:              c.WorkItem.NodeID,
		}

		result := scheduler.ScoreWorkItem(input)
		scored = append(scored, result)
	}

	// Sort by canonical order
	scheduler.CanonicalSort(scored)

	// Allocate slices
	slices, allocBlockers := scheduler.AllocateSlices(scored, req.AvailableMin, maxSlices, req.EnforceVariation)
	blockers = append(blockers, allocBlockers...)

	// Build risk summaries
	var riskSummaries []contract.RiskSummary
	for pid, risk := range projectRisks {
		var dueDateStr *string
		if projectTargetDate[pid] != nil {
			ds := projectTargetDate[pid].Format("2006-01-02")
			dueDateStr = &ds
		}
		recentDaily := float64(projectRecentMin[pid]) / 7.0
		riskSummaries = append(riskSummaries, contract.RiskSummary{
			ProjectID:         pid,
			ProjectName:       projectNames[pid],
			RiskLevel:         risk.Level,
			DueDate:           dueDateStr,
			DaysLeft:          risk.DaysLeft,
			PlannedMinTotal:   projectPlanned[pid],
			LoggedMinTotal:    projectLogged[pid],
			RemainingMinTotal: risk.RemainingMin,
			RequiredDailyMin:  risk.RequiredDailyMin,
			RecentDailyMin:    recentDaily,
			SlackMinPerDay:    risk.SlackMinPerDay,
			ProgressTimePct:   risk.ProgressTimePct,
		})
	}

	// Compute allocated total
	allocatedMin := 0
	for _, sl := range slices {
		allocatedMin += sl.AllocatedMin
	}

	// Build policy messages
	var policyMessages []string
	for pid, risk := range projectRisks {
		if risk.Level == domain.RiskOnTrack {
			policyMessages = append(policyMessages, fmt.Sprintf("%s is on track, secondary work is safe", projectNames[pid]))
		}
	}

	resp := &contract.WhatNowResponse{
		GeneratedAt:     now,
		Mode:            mode,
		RequestedMin:    req.AvailableMin,
		AllocatedMin:    allocatedMin,
		UnallocatedMin:  req.AvailableMin - allocatedMin,
		Recommendations: slices,
		Blockers:        blockers,
		TopRiskProjects: riskSummaries,
		PolicyMessages:  policyMessages,
	}

	return resp, nil
}

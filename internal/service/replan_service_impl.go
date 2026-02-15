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

type replanService struct {
	projects  repository.ProjectRepo
	workItems repository.WorkItemRepo
	sessions  repository.SessionRepo
	profiles  repository.UserProfileRepo
}

func NewReplanService(
	projects repository.ProjectRepo,
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	profiles repository.UserProfileRepo,
) ReplanService {
	return &replanService{
		projects:  projects,
		workItems: workItems,
		sessions:  sessions,
		profiles:  profiles,
	}
}

func (s *replanService) Replan(ctx context.Context, req app.ReplanRequest) (*app.ReplanResponse, error) {
	now := time.Now().UTC()
	if req.Now != nil {
		now = *req.Now
	}

	strategy := req.Strategy
	if strategy == "" {
		strategy = "rebalance"
	}

	profile, err := s.profiles.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading profile: %w", err)
	}

	projects, err := s.projects.List(ctx, req.IncludeArchived)
	if err != nil {
		return nil, fmt.Errorf("loading projects: %w", err)
	}

	projects = filterProjectsByScope(projects, req.ProjectScope)

	activeProjects := make([]*domain.Project, 0)
	for _, p := range projects {
		if p.Status == domain.ProjectActive {
			activeProjects = append(activeProjects, p)
		}
	}

	if len(activeProjects) == 0 {
		return nil, &app.ReplanError{
			Code:    app.ReplanErrNoActiveProjects,
			Message: "no active projects to replan",
		}
	}

	var deltas []app.ProjectReplanDelta
	hasCritical := false

	for _, p := range activeProjects {
		items, err := s.workItems.ListByProject(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("loading items for project %s: %w", p.ID, err)
		}

		metricsBefore := aggregateProjectMetrics(items, p, now)

		recentSessions, err := s.sessions.ListRecentByProject(ctx, p.ID, 7)
		if err != nil {
			return nil, fmt.Errorf("loading recent sessions for project %s: %w", p.ID, err)
		}
		_, effectiveDailyMin := recentDailyPace(recentSessions, 7, profile.BaselineDailyMin)

		riskBefore := scheduler.ComputeRisk(buildRiskInput(metricsBefore, p.TargetDate, profile.BufferPct, effectiveDailyMin, now))

		// Re-estimate work items with units tracking
		changedCount := 0
		for _, item := range items {
			if !item.EligibleForReestimate() {
				continue
			}
			newPlanned := scheduler.SmoothReEstimate(item.PlannedMin, item.LoggedMin, item.UnitsTotal, item.UnitsDone)
			if item.ApplyReestimate(newPlanned, now) {
				if err := s.workItems.Update(ctx, item); err != nil {
					return nil, fmt.Errorf("updating work item %s: %w", item.ID, err)
				}
				changedCount++
			}
		}

		// Recompute metrics after re-estimation
		metricsAfter := aggregateProjectMetrics(items, p, now)
		riskAfter := scheduler.ComputeRisk(buildRiskInput(metricsAfter, p.TargetDate, profile.BufferPct, effectiveDailyMin, now))

		if riskAfter.Level == domain.RiskCritical {
			hasCritical = true
		}

		delta := app.ProjectReplanDelta{
			ProjectID:              p.ID,
			ProjectName:            p.Name,
			RiskBefore:             riskBefore.Level,
			RiskAfter:              riskAfter.Level,
			RequiredDailyMinBefore: riskBefore.RequiredDailyMin,
			RequiredDailyMinAfter:  riskAfter.RequiredDailyMin,
			RemainingMinBefore:     riskBefore.RemainingMin,
			RemainingMinAfter:      riskAfter.RemainingMin,
			ChangedItemsCount:      changedCount,
		}

		deltas = append(deltas, delta)
	}

	globalMode := domain.ModeBalanced
	if hasCritical {
		globalMode = domain.ModeCritical
	}

	resp := &app.ReplanResponse{
		GeneratedAt:        now,
		Trigger:            req.Trigger,
		Strategy:           strategy,
		RecomputedProjects: len(activeProjects),
		Deltas:             deltas,
		GlobalModeAfter:    globalMode,
	}

	return resp, nil
}

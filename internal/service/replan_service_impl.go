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

func (s *replanService) Replan(ctx context.Context, req contract.ReplanRequest) (*contract.ReplanResponse, error) {
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

	if len(req.ProjectScope) > 0 {
		scopeSet := make(map[string]bool)
		for _, id := range req.ProjectScope {
			scopeSet[id] = true
		}
		var filtered []*domain.Project
		for _, p := range projects {
			if scopeSet[p.ID] {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	activeProjects := make([]*domain.Project, 0)
	for _, p := range projects {
		if p.Status == domain.ProjectActive {
			activeProjects = append(activeProjects, p)
		}
	}

	if len(activeProjects) == 0 {
		return nil, &contract.ReplanError{
			Code:    contract.ReplanErrNoActiveProjects,
			Message: "no active projects to replan",
		}
	}

	var deltas []contract.ProjectReplanDelta
	hasCritical := false

	for _, p := range activeProjects {
		items, err := s.workItems.ListByProject(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("loading items for project %s: %w", p.ID, err)
		}

		// Compute before-risk
		var plannedBefore, loggedBefore int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived {
				continue
			}
			plannedBefore += item.PlannedMin
			loggedBefore += item.LoggedMin
		}

		recentSessions, _ := s.sessions.ListRecentByProject(ctx, p.ID, 7)
		var recentMin int
		for _, sess := range recentSessions {
			recentMin += sess.Minutes
		}
		recentDailyMin := float64(recentMin) / 7.0

		riskBefore := scheduler.ComputeRisk(scheduler.RiskInput{
			Now:            now,
			TargetDate:     p.TargetDate,
			PlannedMin:     plannedBefore,
			LoggedMin:      loggedBefore,
			BufferPct:      profile.BufferPct,
			RecentDailyMin: recentDailyMin,
		})

		// Re-estimate work items with units tracking
		changedCount := 0
		for _, item := range items {
			if item.Status == domain.WorkItemArchived || item.Status == domain.WorkItemDone {
				continue
			}
			if item.UnitsTotal > 0 && item.UnitsDone > 0 && item.DurationMode == domain.DurationEstimate {
				newPlanned := scheduler.SmoothReEstimate(item.PlannedMin, item.LoggedMin, item.UnitsTotal, item.UnitsDone)
				if newPlanned != item.PlannedMin {
					item.PlannedMin = newPlanned
					item.UpdatedAt = now
					if err := s.workItems.Update(ctx, item); err != nil {
						return nil, fmt.Errorf("updating work item %s: %w", item.ID, err)
					}
					changedCount++
				}
			}
		}

		// Compute after-risk
		var plannedAfter, loggedAfter int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived {
				continue
			}
			plannedAfter += item.PlannedMin
			loggedAfter += item.LoggedMin
		}

		riskAfter := scheduler.ComputeRisk(scheduler.RiskInput{
			Now:            now,
			TargetDate:     p.TargetDate,
			PlannedMin:     plannedAfter,
			LoggedMin:      loggedAfter,
			BufferPct:      profile.BufferPct,
			RecentDailyMin: recentDailyMin,
		})

		if riskAfter.Level == domain.RiskCritical {
			hasCritical = true
		}

		delta := contract.ProjectReplanDelta{
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

	resp := &contract.ReplanResponse{
		GeneratedAt:        now,
		Trigger:            req.Trigger,
		Strategy:           strategy,
		RecomputedProjects: len(activeProjects),
		Deltas:             deltas,
		GlobalModeAfter:    globalMode,
	}

	return resp, nil
}

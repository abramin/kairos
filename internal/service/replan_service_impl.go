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

	projects = filterProjectsByScope(projects, req.ProjectScope)

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
		var plannedBefore, loggedBefore, donePlannedBefore int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived {
				continue
			}
			plannedBefore += item.PlannedMin
			loggedBefore += item.LoggedMin
			if item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
				donePlannedBefore += item.PlannedMin
			}
		}

		recentSessions, err := s.sessions.ListRecentByProject(ctx, p.ID, 7)
		if err != nil {
			return nil, fmt.Errorf("loading recent sessions for project %s: %w", p.ID, err)
		}
		var recentMin int
		for _, sess := range recentSessions {
			recentMin += sess.Minutes
		}
		recentDailyMin := float64(recentMin) / 7.0
		effectiveDailyMin := math.Max(recentDailyMin, float64(profile.BaselineDailyMin))

		var progressPct float64
		if plannedBefore > 0 {
			progressPct = float64(donePlannedBefore) / float64(plannedBefore) * 100
		}
		var timeElapsedPct float64
		if p.TargetDate != nil {
			totalDays := p.TargetDate.Sub(p.StartDate).Hours() / 24
			elapsedDays := now.Sub(p.StartDate).Hours() / 24
			if totalDays > 0 {
				timeElapsedPct = elapsedDays / totalDays * 100
			}
		}

		// Due-date-aware expected progress
		var dueByNowMin int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived || item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
				continue
			}
			effectiveDue := item.DueDate
			if effectiveDue == nil {
				effectiveDue = p.TargetDate
			}
			if effectiveDue != nil && !effectiveDue.After(now) {
				dueByNowMin += item.PlannedMin
			}
		}
		var dueBasedExpectedPct float64
		if plannedBefore > 0 {
			dueBasedExpectedPct = float64(donePlannedBefore+dueByNowMin) / float64(plannedBefore) * 100
		}

		riskBefore := scheduler.ComputeRisk(scheduler.RiskInput{
			Now:                 now,
			TargetDate:          p.TargetDate,
			PlannedMin:          plannedBefore,
			LoggedMin:           loggedBefore,
			BufferPct:           profile.BufferPct,
			RecentDailyMin:      effectiveDailyMin,
			ProgressPct:         progressPct,
			TimeElapsedPct:      timeElapsedPct,
			DueBasedExpectedPct: dueBasedExpectedPct,
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
		var plannedAfter, loggedAfter, donePlannedAfter int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived {
				continue
			}
			plannedAfter += item.PlannedMin
			loggedAfter += item.LoggedMin
			if item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
				donePlannedAfter += item.PlannedMin
			}
		}

		var progressPctAfter float64
		if plannedAfter > 0 {
			progressPctAfter = float64(donePlannedAfter) / float64(plannedAfter) * 100
		}

		// Recompute due-based expected with updated planned totals
		var dueByNowMinAfter int
		for _, item := range items {
			if item.Status == domain.WorkItemArchived || item.Status == domain.WorkItemDone || item.Status == domain.WorkItemSkipped {
				continue
			}
			effectiveDue := item.DueDate
			if effectiveDue == nil {
				effectiveDue = p.TargetDate
			}
			if effectiveDue != nil && !effectiveDue.After(now) {
				dueByNowMinAfter += item.PlannedMin
			}
		}
		var dueBasedExpectedPctAfter float64
		if plannedAfter > 0 {
			dueBasedExpectedPctAfter = float64(donePlannedAfter+dueByNowMinAfter) / float64(plannedAfter) * 100
		}

		riskAfter := scheduler.ComputeRisk(scheduler.RiskInput{
			Now:                 now,
			TargetDate:          p.TargetDate,
			PlannedMin:          plannedAfter,
			LoggedMin:           loggedAfter,
			BufferPct:           profile.BufferPct,
			RecentDailyMin:      effectiveDailyMin,
			ProgressPct:         progressPctAfter,
			TimeElapsedPct:      timeElapsedPct,
			DueBasedExpectedPct: dueBasedExpectedPctAfter,
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

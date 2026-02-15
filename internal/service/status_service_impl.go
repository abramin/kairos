package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

type statusService struct {
	projects  repository.ProjectRepo
	workItems repository.WorkItemRepo
	sessions  repository.SessionRepo
	profiles  repository.UserProfileRepo
}

func NewStatusService(
	projects repository.ProjectRepo,
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	profiles repository.UserProfileRepo,
) StatusService {
	return &statusService{
		projects:  projects,
		workItems: workItems,
		sessions:  sessions,
		profiles:  profiles,
	}
}

func (s *statusService) GetStatus(ctx context.Context, req app.StatusRequest) (*app.StatusResponse, error) {
	now := time.Now().UTC()
	if req.Now != nil {
		now = *req.Now
	}

	days := req.IncludeRecentSessionDays
	if days <= 0 {
		days = 7
	}

	profile, err := s.profiles.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading user profile: %w", err)
	}

	projects, err := s.projects.List(ctx, req.IncludeArchived)
	if err != nil {
		return nil, fmt.Errorf("loading projects: %w", err)
	}

	projects = filterProjectsByScope(projects, req.ProjectScope)

	var views []app.ProjectStatusView
	countOnTrack, countAtRisk, countCritical := 0, 0, 0
	hasCritical := false

	for _, p := range projects {
		if p.Status != domain.ProjectActive {
			continue
		}

		items, err := s.workItems.ListByProject(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("loading work items for project %s: %w", p.ID, err)
		}

		m := aggregateProjectMetrics(items, p, now)

		recentSessions, err := s.sessions.ListRecentByProject(ctx, p.ID, days)
		if err != nil {
			return nil, fmt.Errorf("loading recent sessions for project %s: %w", p.ID, err)
		}
		recentDailyMin, effectiveDailyMin := recentDailyPace(recentSessions, days, profile.BaselineDailyMin)

		riskResult := scheduler.ComputeRisk(buildRiskInput(m, p.TargetDate, profile.BufferPct, effectiveDailyMin, now))

		var structuralPct float64
		if m.TotalCount > 0 {
			structuralPct = float64(m.DoneCount) / float64(m.TotalCount) * 100
		}

		var dueDateStr *string
		if p.TargetDate != nil {
			ds := p.TargetDate.Format("2006-01-02")
			dueDateStr = &ds
		}

		view := app.ProjectStatusView{
			ProjectID:             p.ID,
			ProjectName:           p.Name,
			Status:                p.Status,
			RiskLevel:             riskResult.Level,
			DueDate:               dueDateStr,
			DaysLeft:              riskResult.DaysLeft,
			ProgressTimePct:       riskResult.ProgressTimePct,
			ProgressStructuralPct: structuralPct,
			PlannedMinTotal:       m.PlannedMin,
			LoggedMinTotal:        m.LoggedMin,
			RemainingMinTotal:     riskResult.RemainingMin,
			RequiredDailyMin:      riskResult.RequiredDailyMin,
			RecentDailyMin:        recentDailyMin,
			SlackMinPerDay:        riskResult.SlackMinPerDay,
			SafeForSecondaryWork:  riskResult.Level == domain.RiskOnTrack,
		}

		views = append(views, view)

		switch riskResult.Level {
		case domain.RiskOnTrack:
			countOnTrack++
		case domain.RiskAtRisk:
			countAtRisk++
		case domain.RiskCritical:
			countCritical++
			hasCritical = true
		}
	}

	// Sort views by canonical order
	sort.Slice(views, func(i, j int) bool {
		ri := scheduler.RiskPriority(views[i].RiskLevel)
		rj := scheduler.RiskPriority(views[j].RiskLevel)
		if ri != rj {
			return ri < rj
		}
		// Earliest due date first, nil last
		if (views[i].DueDate == nil) != (views[j].DueDate == nil) {
			return views[i].DueDate != nil
		}
		if views[i].DueDate != nil && views[j].DueDate != nil && *views[i].DueDate != *views[j].DueDate {
			return *views[i].DueDate < *views[j].DueDate
		}
		return views[i].ProjectName < views[j].ProjectName
	})

	globalMode := domain.ModeBalanced
	if hasCritical {
		globalMode = domain.ModeCritical
	}

	policyMsg := "All projects on track"
	if hasCritical {
		policyMsg = "Critical work requires attention"
	} else if countAtRisk > 0 {
		policyMsg = "Some projects at risk, monitor closely"
	}

	return &app.StatusResponse{
		Summary: app.GlobalStatusSummary{
			GeneratedAt:     now,
			CountsTotal:     countOnTrack + countAtRisk + countCritical,
			CountsOnTrack:   countOnTrack,
			CountsAtRisk:    countAtRisk,
			CountsCritical:  countCritical,
			GlobalModeIfNow: globalMode,
			PolicyMessage:   policyMsg,
		},
		Projects: views,
	}, nil
}

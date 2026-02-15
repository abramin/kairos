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

	views, err := s.buildProjectViews(ctx, projects, profile, days, now)
	if err != nil {
		return nil, err
	}

	sortStatusViews(views)

	return &app.StatusResponse{
		Summary: buildStatusSummary(views, now),
		Projects: views,
	}, nil
}

func (s *statusService) buildProjectViews(
	ctx context.Context,
	projects []*domain.Project,
	profile *domain.UserProfile,
	days int,
	now time.Time,
) ([]app.ProjectStatusView, error) {
	var views []app.ProjectStatusView
	for _, p := range projects {
		if p.Status != domain.ProjectActive {
			continue
		}

		snap, _, err := computeProjectRiskSnapshot(ctx, p, s.workItems, s.sessions, profile, days, now)
		if err != nil {
			return nil, err
		}

		var structuralPct float64
		if snap.Metrics.TotalCount > 0 {
			structuralPct = float64(snap.Metrics.DoneCount) / float64(snap.Metrics.TotalCount) * 100
		}

		var dueDateStr *string
		if p.TargetDate != nil {
			ds := p.TargetDate.Format("2006-01-02")
			dueDateStr = &ds
		}

		views = append(views, app.ProjectStatusView{
			ProjectID:             p.ID,
			ProjectName:           p.Name,
			Status:                p.Status,
			RiskLevel:             snap.Risk.Level,
			DueDate:               dueDateStr,
			DaysLeft:              snap.Risk.DaysLeft,
			ProgressTimePct:       snap.Risk.ProgressTimePct,
			ProgressStructuralPct: structuralPct,
			PlannedMinTotal:       snap.Metrics.PlannedMin,
			LoggedMinTotal:        snap.Metrics.LoggedMin,
			RemainingMinTotal:     snap.Risk.RemainingMin,
			RequiredDailyMin:      snap.Risk.RequiredDailyMin,
			RecentDailyMin:        snap.RecentDailyMin,
			SlackMinPerDay:        snap.Risk.SlackMinPerDay,
			SafeForSecondaryWork:  snap.Risk.Level == domain.RiskOnTrack,
		})
	}
	return views, nil
}

func sortStatusViews(views []app.ProjectStatusView) {
	sort.Slice(views, func(i, j int) bool {
		ri := scheduler.RiskPriority(views[i].RiskLevel)
		rj := scheduler.RiskPriority(views[j].RiskLevel)
		if ri != rj {
			return ri < rj
		}
		if (views[i].DueDate == nil) != (views[j].DueDate == nil) {
			return views[i].DueDate != nil
		}
		if views[i].DueDate != nil && views[j].DueDate != nil && *views[i].DueDate != *views[j].DueDate {
			return *views[i].DueDate < *views[j].DueDate
		}
		return views[i].ProjectName < views[j].ProjectName
	})
}

func buildStatusSummary(views []app.ProjectStatusView, now time.Time) app.GlobalStatusSummary {
	var countOnTrack, countAtRisk, countCritical int
	for _, v := range views {
		switch v.RiskLevel {
		case domain.RiskOnTrack:
			countOnTrack++
		case domain.RiskAtRisk:
			countAtRisk++
		case domain.RiskCritical:
			countCritical++
		}
	}

	globalMode := domain.ModeBalanced
	if countCritical > 0 {
		globalMode = domain.ModeCritical
	}

	policyMsg := "All projects on track"
	if countCritical > 0 {
		policyMsg = "Critical work requires attention"
	} else if countAtRisk > 0 {
		policyMsg = "Some projects at risk, monitor closely"
	}

	return app.GlobalStatusSummary{
		GeneratedAt:     now,
		CountsTotal:     countOnTrack + countAtRisk + countCritical,
		CountsOnTrack:   countOnTrack,
		CountsAtRisk:    countAtRisk,
		CountsCritical:  countCritical,
		GlobalModeIfNow: globalMode,
		PolicyMessage:   policyMsg,
	}
}

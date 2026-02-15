package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
)

type replanService struct {
	projects  repository.ProjectRepo
	workItems repository.WorkItemRepo
	sessions  repository.SessionRepo
	profiles  repository.UserProfileRepo
	uow       db.UnitOfWork
	observer  UseCaseObserver
}

func NewReplanService(
	projects repository.ProjectRepo,
	workItems repository.WorkItemRepo,
	sessions repository.SessionRepo,
	profiles repository.UserProfileRepo,
	uow db.UnitOfWork,
	observers ...UseCaseObserver,
) ReplanService {
	return &replanService{
		projects:  projects,
		workItems: workItems,
		sessions:  sessions,
		profiles:  profiles,
		uow:       uow,
		observer:  useCaseObserverOrNoop(observers),
	}
}

func (s *replanService) Replan(ctx context.Context, req app.ReplanRequest) (resp *app.ReplanResponse, err error) {
	startedAt := time.Now().UTC()
	now := time.Now().UTC()
	if req.Now != nil {
		now = *req.Now
	}
	fields := map[string]any{
		"trigger":          req.Trigger,
		"include_archived": req.IncludeArchived,
		"project_scope":    len(req.ProjectScope),
	}
	defer func() {
		if resp != nil {
			fields["recomputed_projects"] = resp.RecomputedProjects
			fields["global_mode_after"] = string(resp.GlobalModeAfter)
		}
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "replan",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	strategy := req.Strategy
	if strategy == "" {
		strategy = "rebalance"
	}
	fields["strategy"] = strategy

	days := req.IncludeRecentSessionDays
	if days <= 0 {
		days = 7
	}

	var profile *domain.UserProfile
	profile, err = s.profiles.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading profile: %w", err)
	}

	var projects []*domain.Project
	projects, err = s.projects.List(ctx, req.IncludeArchived)
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
		snap, items, err := computeProjectRiskSnapshot(ctx, p, s.workItems, s.sessions, profile, days, now)
		if err != nil {
			return nil, err
		}

		riskBefore := snap.Risk

		// Re-estimate work items within a transaction
		changedCount, err := s.reestimateItems(ctx, items, now)
		if err != nil {
			return nil, err
		}

		// Recompute risk after re-estimation
		metricsAfter := aggregateProjectMetrics(items, p, now)
		riskAfter := scheduler.ComputeRisk(buildRiskInput(metricsAfter, p.TargetDate, profile.BufferPct, snap.EffectiveDailyMin, now))

		if riskAfter.Level == domain.RiskCritical {
			hasCritical = true
		}

		deltas = append(deltas, app.ProjectReplanDelta{
			ProjectID:              p.ID,
			ProjectName:            p.Name,
			RiskBefore:             riskBefore.Level,
			RiskAfter:              riskAfter.Level,
			RequiredDailyMinBefore: riskBefore.RequiredDailyMin,
			RequiredDailyMinAfter:  riskAfter.RequiredDailyMin,
			RemainingMinBefore:     riskBefore.RemainingMin,
			RemainingMinAfter:      riskAfter.RemainingMin,
			ChangedItemsCount:      changedCount,
		})
	}

	globalMode := domain.ModeBalanced
	if hasCritical {
		globalMode = domain.ModeCritical
	}

	resp = &app.ReplanResponse{
		GeneratedAt:        now,
		Trigger:            req.Trigger,
		Strategy:           strategy,
		RecomputedProjects: len(activeProjects),
		Deltas:             deltas,
		GlobalModeAfter:    globalMode,
	}

	return resp, nil
}

// reestimateItems applies smooth re-estimation to eligible items within a transaction.
func (s *replanService) reestimateItems(ctx context.Context, items []*domain.WorkItem, now time.Time) (int, error) {
	// Collect items that need re-estimation first.
	type reestimate struct {
		item       *domain.WorkItem
		newPlanned int
	}
	var updates []reestimate
	for _, item := range items {
		if !item.EligibleForReestimate() {
			continue
		}
		newPlanned := scheduler.SmoothReEstimate(item.PlannedMin, item.LoggedMin, item.UnitsTotal, item.UnitsDone)
		if item.ApplyReestimate(newPlanned, now) {
			updates = append(updates, reestimate{item: item, newPlanned: newPlanned})
		}
	}

	if len(updates) == 0 {
		return 0, nil
	}

	err := s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		for _, u := range updates {
			if err := txWorkItems.Update(ctx, u.item); err != nil {
				return fmt.Errorf("updating work item %s: %w", u.item.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return len(updates), nil
}

package service

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
	"github.com/google/uuid"
)

type sessionService struct {
	sessions  repository.SessionRepo
	workItems repository.WorkItemRepo
}

func NewSessionService(sessions repository.SessionRepo, workItems repository.WorkItemRepo) SessionService {
	return &sessionService{sessions: sessions, workItems: workItems}
}

func (s *sessionService) LogSession(ctx context.Context, session *domain.WorkSessionLog) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.CreatedAt = time.Now().UTC()

	// Update work item logged_min and units_done
	wi, err := s.workItems.GetByID(ctx, session.WorkItemID)
	if err != nil {
		return err
	}

	wi.LoggedMin += session.Minutes
	wi.UnitsDone += session.UnitsDoneDelta

	// Auto-set status to in_progress if still todo
	if wi.Status == domain.WorkItemTodo {
		wi.Status = domain.WorkItemInProgress
	}

	// Smooth re-estimation if units tracking available
	if wi.UnitsTotal > 0 && wi.UnitsDone > 0 && wi.DurationMode == domain.DurationEstimate {
		wi.PlannedMin = scheduler.SmoothReEstimate(wi.PlannedMin, wi.LoggedMin, wi.UnitsTotal, wi.UnitsDone)
	}

	wi.UpdatedAt = time.Now().UTC()
	if err := s.workItems.Update(ctx, wi); err != nil {
		return err
	}

	return s.sessions.Create(ctx, session)
}

func (s *sessionService) GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error) {
	return s.sessions.GetByID(ctx, id)
}

func (s *sessionService) ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error) {
	return s.sessions.ListByWorkItem(ctx, workItemID)
}

func (s *sessionService) ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error) {
	return s.sessions.ListRecent(ctx, days)
}

func (s *sessionService) ListRecentSummaryByType(ctx context.Context, days int) ([]domain.SessionSummaryByType, error) {
	return s.sessions.ListRecentSummaryByType(ctx, days)
}

func (s *sessionService) Delete(ctx context.Context, id string) error {
	return s.sessions.Delete(ctx, id)
}

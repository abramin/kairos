package service

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/scheduler"
	"github.com/google/uuid"
)

type sessionService struct {
	sessions repository.SessionRepo
	uow      db.UnitOfWork
	observer UseCaseObserver
}

func NewSessionService(
	sessions repository.SessionRepo,
	uow db.UnitOfWork,
	observers ...UseCaseObserver,
) SessionService {
	return &sessionService{
		sessions: sessions,
		uow:      uow,
		observer: useCaseObserverOrNoop(observers),
	}
}

func (s *sessionService) LogSession(ctx context.Context, session *domain.WorkSessionLog) (err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"work_item_id": session.WorkItemID,
		"minutes":      session.Minutes,
		"units_delta":  session.UnitsDoneDelta,
	}
	defer func() {
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "log-session",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.CreatedAt = time.Now().UTC()
	fields["session_id"] = session.ID

	return s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		txSessions := repository.NewSQLiteSessionRepo(tx)

		// Read work item within transaction
		wi, err := txWorkItems.GetByID(ctx, session.WorkItemID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		if err := wi.ApplySession(session.Minutes, session.UnitsDoneDelta, now); err != nil {
			return err
		}

		if wi.EligibleForReestimate() {
			newPlanned := scheduler.SmoothReEstimate(wi.PlannedMin, wi.LoggedMin, wi.UnitsTotal, wi.UnitsDone)
			wi.ApplyReestimate(newPlanned, now)
		}
		if err := txWorkItems.Update(ctx, wi); err != nil {
			return err
		}

		return txSessions.Create(ctx, session)
	})
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

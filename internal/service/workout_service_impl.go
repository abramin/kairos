package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type workoutService struct {
	workouts repository.WorkoutLogRepo
	observer UseCaseObserver
}

// NewWorkoutService creates a new WorkoutService.
func NewWorkoutService(
	workouts repository.WorkoutLogRepo,
	observers ...UseCaseObserver,
) WorkoutService {
	return &workoutService{
		workouts: workouts,
		observer: useCaseObserverOrNoop(observers),
	}
}

func (s *workoutService) LogWorkout(ctx context.Context, req LogWorkoutRequest) (w domain.WorkoutLog, err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"category": string(req.Category),
		"minutes":  req.Minutes,
	}
	defer func() {
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "log-workout",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	if !domain.ValidWorkoutCategories[string(req.Category)] {
		return w, fmt.Errorf("invalid workout category: %q", req.Category)
	}
	if req.Minutes <= 0 {
		return w, fmt.Errorf("minutes must be positive, got %d", req.Minutes)
	}

	now := time.Now().UTC()
	performedAt := now
	if req.PerformedAt != nil {
		performedAt = *req.PerformedAt
	}

	w = domain.WorkoutLog{
		ID:          uuid.New().String(),
		Category:    req.Category,
		Minutes:     req.Minutes,
		PerformedAt: performedAt,
		Notes:       req.Notes,
		CreatedAt:   now,
	}
	fields["workout_id"] = w.ID

	if err := s.workouts.Create(ctx, &w); err != nil {
		return domain.WorkoutLog{}, err
	}
	return w, nil
}

func (s *workoutService) DeleteWorkout(ctx context.Context, id string) error {
	return s.workouts.Delete(ctx, id)
}

func (s *workoutService) ListRecent(ctx context.Context, limit int) ([]domain.WorkoutLog, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.workouts.ListRecent(ctx, limit)
}

func (s *workoutService) ListByDateRange(ctx context.Context, from, to time.Time) ([]domain.WorkoutLog, error) {
	return s.workouts.ListByDateRange(ctx, from, to)
}

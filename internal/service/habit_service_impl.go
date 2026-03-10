package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type habitService struct {
	habits repository.HabitRepo
}

// NewHabitService creates a new HabitService.
func NewHabitService(habits repository.HabitRepo) HabitService {
	return &habitService{habits: habits}
}

func (s *habitService) Add(ctx context.Context, req AddHabitRequest) (*domain.Habit, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("habit title must not be empty")
	}
	if req.CadenceDays <= 0 {
		req.CadenceDays = 1
	}
	if req.TargetMin <= 0 {
		req.TargetMin = 20
	}
	if req.MinSessionMin <= 0 {
		req.MinSessionMin = max(5, req.TargetMin-10)
	}
	if req.MaxSessionMin <= 0 {
		req.MaxSessionMin = req.TargetMin + 10
	}

	now := time.Now().UTC()
	h := &domain.Habit{
		ID:            uuid.New().String(),
		Title:         title,
		CadenceDays:   req.CadenceDays,
		TargetMin:     req.TargetMin,
		MinSessionMin: req.MinSessionMin,
		MaxSessionMin: req.MaxSessionMin,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.habits.Create(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *habitService) ListActive(ctx context.Context) ([]*domain.Habit, error) {
	return s.habits.ListActive(ctx)
}

func (s *habitService) GetStatus(ctx context.Context, now time.Time) ([]HabitStatus, error) {
	habits, err := s.habits.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading habits: %w", err)
	}

	today := truncateToDay(now)
	statuses := make([]HabitStatus, 0, len(habits))
	for _, h := range habits {
		lastLog, err := s.habits.LastLog(ctx, h.ID)
		if err != nil {
			return nil, fmt.Errorf("loading last log for habit %s: %w", h.ID, err)
		}

		daysSince := 9999
		if lastLog != nil {
			lastDay := truncateToDay(lastLog.PerformedAt)
			daysSince = int(today.Sub(lastDay).Hours() / 24)
		}

		daysUntilDue := h.CadenceDays - daysSince
		statuses = append(statuses, HabitStatus{
			Habit:        h,
			LastLog:      lastLog,
			DaysSinceLog: daysSince,
			DaysUntilDue: daysUntilDue,
			DueToday:     daysUntilDue <= 0 && daysSince > 0,
		})
	}
	return statuses, nil
}

func (s *habitService) GetByID(ctx context.Context, id string) (*domain.Habit, error) {
	return s.habits.GetByID(ctx, id)
}

func (s *habitService) Archive(ctx context.Context, id string) error {
	return s.habits.Archive(ctx, id, time.Now().UTC())
}

func (s *habitService) LogSession(ctx context.Context, req LogHabitRequest) (*domain.HabitLog, error) {
	h, err := s.habits.GetByID(ctx, req.HabitID)
	if err != nil {
		return nil, fmt.Errorf("loading habit: %w", err)
	}
	if !h.IsActive() {
		return nil, fmt.Errorf("habit is archived")
	}

	minutes := req.Minutes
	if minutes <= 0 {
		minutes = h.TargetMin
	}

	now := time.Now().UTC()
	log := &domain.HabitLog{
		ID:          uuid.New().String(),
		HabitID:     req.HabitID,
		PerformedAt: now,
		Minutes:     minutes,
		Note:        req.Note,
		CreatedAt:   now,
	}
	if err := s.habits.LogSession(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *habitService) UndoLog(ctx context.Context, logID string) error {
	return s.habits.DeleteLog(ctx, logID)
}

// truncateToDay returns t truncated to midnight UTC.
func truncateToDay(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}

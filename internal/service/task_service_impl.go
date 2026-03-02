package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type taskService struct {
	tasks repository.TaskRepo
}

// NewTaskService creates a new TaskService.
func NewTaskService(tasks repository.TaskRepo) TaskService {
	return &taskService{tasks: tasks}
}

func (s *taskService) Add(ctx context.Context, req AddTaskRequest) (*domain.Task, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}
	now := time.Now().UTC()
	t := &domain.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *taskService) ListActive(ctx context.Context) ([]*domain.Task, error) {
	return s.tasks.ListActive(ctx)
}

func (s *taskService) Update(ctx context.Context, id, title, description string) error {
	if title == "" {
		return fmt.Errorf("task title is required")
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting task: %w", err)
	}
	t.Title = title
	t.Description = description
	t.UpdatedAt = time.Now().UTC()
	return s.tasks.Update(ctx, t)
}

func (s *taskService) MarkDone(ctx context.Context, id string) error {
	return s.tasks.Archive(ctx, id, time.Now().UTC())
}

func (s *taskService) Delete(ctx context.Context, id string) error {
	return s.tasks.Delete(ctx, id)
}

func (s *taskService) MoveUp(ctx context.Context, id string) error {
	tasks, err := s.tasks.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("listing tasks for reorder: %w", err)
	}
	idx := -1
	for i, t := range tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return nil // not found or already at top
	}
	return s.tasks.SwapOrder(ctx, tasks[idx].ID, tasks[idx-1].ID)
}

func (s *taskService) MoveDown(ctx context.Context, id string) error {
	tasks, err := s.tasks.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("listing tasks for reorder: %w", err)
	}
	idx := -1
	for i, t := range tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(tasks)-1 {
		return nil // not found or already at bottom
	}
	return s.tasks.SwapOrder(ctx, tasks[idx].ID, tasks[idx+1].ID)
}

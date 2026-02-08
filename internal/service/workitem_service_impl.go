package service

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type workItemService struct {
	workItems repository.WorkItemRepo
}

func NewWorkItemService(workItems repository.WorkItemRepo) WorkItemService {
	return &workItemService{workItems: workItems}
}

func (s *workItemService) Create(ctx context.Context, w *domain.WorkItem) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	w.CreatedAt = now
	w.UpdatedAt = now
	if w.Status == "" {
		w.Status = domain.WorkItemTodo
	}
	if w.DurationMode == "" {
		w.DurationMode = domain.DurationEstimate
	}
	if w.DurationSource == "" {
		w.DurationSource = domain.SourceManual
	}
	return s.workItems.Create(ctx, w)
}

func (s *workItemService) GetByID(ctx context.Context, id string) (*domain.WorkItem, error) {
	return s.workItems.GetByID(ctx, id)
}

func (s *workItemService) ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error) {
	return s.workItems.ListByNode(ctx, nodeID)
}

func (s *workItemService) ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error) {
	return s.workItems.ListByProject(ctx, projectID)
}

func (s *workItemService) Update(ctx context.Context, w *domain.WorkItem) error {
	w.UpdatedAt = time.Now().UTC()
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) MarkDone(ctx context.Context, id string) error {
	w, err := s.workItems.GetByID(ctx, id)
	if err != nil {
		return err
	}
	w.Status = domain.WorkItemDone
	w.UpdatedAt = time.Now().UTC()
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) Archive(ctx context.Context, id string) error {
	return s.workItems.Archive(ctx, id)
}

func (s *workItemService) Delete(ctx context.Context, id string) error {
	return s.workItems.Delete(ctx, id)
}

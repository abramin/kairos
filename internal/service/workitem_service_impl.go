package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type workItemService struct {
	workItems repository.WorkItemRepo
	nodes     repository.PlanNodeRepo
}

func NewWorkItemService(workItems repository.WorkItemRepo, nodes repository.PlanNodeRepo) WorkItemService {
	return &workItemService{workItems: workItems, nodes: nodes}
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

	if w.Seq == 0 {
		node, err := s.nodes.GetByID(ctx, w.NodeID)
		if err != nil {
			return fmt.Errorf("looking up node for seq: %w", err)
		}
		seq, err := s.nodes.NextProjectSeq(ctx, node.ProjectID)
		if err != nil {
			return fmt.Errorf("assigning seq: %w", err)
		}
		w.Seq = seq
	}

	return s.workItems.Create(ctx, w)
}

func (s *workItemService) GetByID(ctx context.Context, id string) (*domain.WorkItem, error) {
	return s.workItems.GetByID(ctx, id)
}

func (s *workItemService) GetBySeq(ctx context.Context, projectID string, seq int) (*domain.WorkItem, error) {
	return s.workItems.GetBySeq(ctx, projectID, seq)
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

func (s *workItemService) MarkInProgress(ctx context.Context, id string) error {
	w, err := s.workItems.GetByID(ctx, id)
	if err != nil {
		return err
	}
	w.Status = domain.WorkItemInProgress
	w.UpdatedAt = time.Now().UTC()
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) Archive(ctx context.Context, id string) error {
	return s.workItems.Archive(ctx, id)
}

func (s *workItemService) Delete(ctx context.Context, id string) error {
	return s.workItems.Delete(ctx, id)
}

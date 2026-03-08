package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type workItemService struct {
	workItems repository.WorkItemRepo
	nodes     repository.PlanNodeRepo
	uow       db.UnitOfWork
}

func NewWorkItemService(
	workItems repository.WorkItemRepo,
	nodes repository.PlanNodeRepo,
	uow db.UnitOfWork,
) WorkItemService {
	return &workItemService{
		workItems: workItems,
		nodes:     nodes,
		uow:       uow,
	}
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
	if w.MinSessionMin == 0 && w.MaxSessionMin == 0 && w.DefaultSessionMin == 0 {
		w.MinSessionMin = domain.DefaultMinSessionMin
		w.MaxSessionMin = domain.DefaultMaxSessionMin
		w.DefaultSessionMin = domain.DefaultDefaultSessionMin
	}

	return s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		txSeqs := repository.NewSQLiteProjectSequenceRepo(tx)

		if w.Seq == 0 {
			node, err := txNodes.GetByID(ctx, w.NodeID)
			if err != nil {
				return fmt.Errorf("looking up node for seq: %w", err)
			}
			seq, err := txSeqs.NextProjectSeq(ctx, node.ProjectID)
			if err != nil {
				return fmt.Errorf("assigning seq: %w", err)
			}
			w.Seq = seq
		}

		return txWorkItems.Create(ctx, w)
	})
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
	if err := w.MarkDone(time.Now().UTC()); err != nil {
		return err
	}
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) Reopen(ctx context.Context, id string) error {
	w, err := s.workItems.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := w.Reopen(time.Now().UTC()); err != nil {
		return err
	}
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) MarkInProgress(ctx context.Context, id string) error {
	w, err := s.workItems.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := w.MarkInProgress(time.Now().UTC()); err != nil {
		return err
	}
	return s.workItems.Update(ctx, w)
}

func (s *workItemService) ListDueItems(ctx context.Context, daysAhead int) ([]DueItem, error) {
	if daysAhead <= 0 {
		daysAhead = 14
	}
	candidates, err := s.workItems.ListSchedulable(ctx, false)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, daysAhead)
	var out []DueItem
	for _, c := range candidates {
		due := c.WorkItem.DueDate
		if due == nil {
			due = c.NodeDueDate
		}
		if due == nil || due.After(cutoff) {
			continue
		}
		out = append(out, DueItem{
			WorkItemID:  c.WorkItem.ID,
			Seq:         c.WorkItem.Seq,
			Title:       c.WorkItem.Title,
			ProjectName: c.ProjectName,
			DueDate:     *due,
			PlannedMin:  c.WorkItem.PlannedMin,
			Status:      c.WorkItem.Status,
		})
	}
	return out, nil
}

func (s *workItemService) Archive(ctx context.Context, id string) error {
	return s.workItems.Archive(ctx, id)
}

func (s *workItemService) Delete(ctx context.Context, id string) error {
	return s.workItems.Delete(ctx, id)
}

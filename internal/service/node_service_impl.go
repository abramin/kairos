package service

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

type nodeService struct {
	nodes repository.PlanNodeRepo
}

func NewNodeService(nodes repository.PlanNodeRepo) NodeService {
	return &nodeService{nodes: nodes}
}

func (s *nodeService) Create(ctx context.Context, n *domain.PlanNode) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	n.CreatedAt = now
	n.UpdatedAt = now
	return s.nodes.Create(ctx, n)
}

func (s *nodeService) GetByID(ctx context.Context, id string) (*domain.PlanNode, error) {
	return s.nodes.GetByID(ctx, id)
}

func (s *nodeService) ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListByProject(ctx, projectID)
}

func (s *nodeService) ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListChildren(ctx, parentID)
}

func (s *nodeService) ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error) {
	return s.nodes.ListRoots(ctx, projectID)
}

func (s *nodeService) Update(ctx context.Context, n *domain.PlanNode) error {
	n.UpdatedAt = time.Now().UTC()
	return s.nodes.Update(ctx, n)
}

func (s *nodeService) Delete(ctx context.Context, id string) error {
	return s.nodes.Delete(ctx, id)
}

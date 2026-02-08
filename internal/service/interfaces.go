package service

import (
	"context"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
)

type ProjectService interface {
	Create(ctx context.Context, p *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	List(ctx context.Context, includeArchived bool) ([]*domain.Project, error)
	Update(ctx context.Context, p *domain.Project) error
	Archive(ctx context.Context, id string) error
	Unarchive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string, force bool) error
}

type NodeService interface {
	Create(ctx context.Context, n *domain.PlanNode) error
	GetByID(ctx context.Context, id string) (*domain.PlanNode, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error)
	ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	Update(ctx context.Context, n *domain.PlanNode) error
	Delete(ctx context.Context, id string) error
}

type WorkItemService interface {
	Create(ctx context.Context, w *domain.WorkItem) error
	GetByID(ctx context.Context, id string) (*domain.WorkItem, error)
	ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error)
	Update(ctx context.Context, w *domain.WorkItem) error
	MarkDone(ctx context.Context, id string) error
	Archive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type SessionService interface {
	LogSession(ctx context.Context, s *domain.WorkSessionLog) error
	GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error)
	ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error)
	ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error)
	Delete(ctx context.Context, id string) error
}

type WhatNowService interface {
	Recommend(ctx context.Context, req contract.WhatNowRequest) (*contract.WhatNowResponse, error)
}

type StatusService interface {
	GetStatus(ctx context.Context, req contract.StatusRequest) (*contract.StatusResponse, error)
}

type ReplanService interface {
	Replan(ctx context.Context, req contract.ReplanRequest) (*contract.ReplanResponse, error)
}

type TemplateService interface {
	List(ctx context.Context) ([]domain.Template, error)
	Get(ctx context.Context, name string) (*domain.Template, error)
	InitProject(ctx context.Context, templateName string, projectName string, shortID string, startDate string, dueDate *string, vars map[string]string) (*domain.Project, error)
}

// ImportResult holds the outcome of a project import.
type ImportResult struct {
	Project         *domain.Project
	NodeCount       int
	WorkItemCount   int
	DependencyCount int
}

type ImportService interface {
	ImportProject(ctx context.Context, filePath string) (*ImportResult, error)
	ImportProjectFromSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error)
}

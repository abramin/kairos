package repository

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SchedulableCandidate is a joined view of a work item with its project context,
// used by the scheduler for scoring candidates.
type SchedulableCandidate struct {
	WorkItem          domain.WorkItem
	ProjectID         string
	ProjectName       string
	ProjectDomain     string
	NodeTitle         string
	NodeDueDate       *time.Time
	ProjectTargetDate *time.Time
}

type ProjectRepo interface {
	Create(ctx context.Context, p *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	GetByShortID(ctx context.Context, shortID string) (*domain.Project, error)
	List(ctx context.Context, includeArchived bool) ([]*domain.Project, error)
	Update(ctx context.Context, p *domain.Project) error
	Archive(ctx context.Context, id string) error
	Unarchive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type PlanNodeRepo interface {
	Create(ctx context.Context, n *domain.PlanNode) error
	GetByID(ctx context.Context, id string) (*domain.PlanNode, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error)
	ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	Update(ctx context.Context, n *domain.PlanNode) error
	Delete(ctx context.Context, id string) error
}

type WorkItemRepo interface {
	Create(ctx context.Context, w *domain.WorkItem) error
	GetByID(ctx context.Context, id string) (*domain.WorkItem, error)
	ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error)
	ListSchedulable(ctx context.Context, includeArchived bool) ([]SchedulableCandidate, error)
	Update(ctx context.Context, w *domain.WorkItem) error
	Archive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type DependencyRepo interface {
	Create(ctx context.Context, d *domain.Dependency) error
	Delete(ctx context.Context, predecessorID, successorID string) error
	ListPredecessors(ctx context.Context, workItemID string) ([]domain.Dependency, error)
	ListSuccessors(ctx context.Context, workItemID string) ([]domain.Dependency, error)
	HasUnfinishedPredecessors(ctx context.Context, workItemID string) (bool, error)
}

type SessionRepo interface {
	Create(ctx context.Context, s *domain.WorkSessionLog) error
	GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error)
	ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error)
	ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error)
	ListRecentByProject(ctx context.Context, projectID string, days int) ([]*domain.WorkSessionLog, error)
	Delete(ctx context.Context, id string) error
}

type UserProfileRepo interface {
	Get(ctx context.Context) (*domain.UserProfile, error)
	Upsert(ctx context.Context, p *domain.UserProfile) error
}

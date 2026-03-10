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
	ProjectStartDate  *time.Time
}

// CompletedWorkSummary holds per-project aggregates for completed (done/skipped) work items.
type CompletedWorkSummary struct {
	ProjectID      string
	PlannedMin     int
	LoggedMin      int
	DoneItemCount  int
	TotalItemCount int
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
	GetBySeq(ctx context.Context, projectID string, seq int) (*domain.PlanNode, error)
	NextProjectSeq(ctx context.Context, projectID string) (int, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error)
	ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	Update(ctx context.Context, n *domain.PlanNode) error
	Delete(ctx context.Context, id string) error
}

// ProjectSequenceRepo allocates project-scoped sequential IDs shared across
// both plan_nodes and work_items.
type ProjectSequenceRepo interface {
	NextProjectSeq(ctx context.Context, projectID string) (int, error)
}

type WorkItemRepo interface {
	Create(ctx context.Context, w *domain.WorkItem) error
	GetByID(ctx context.Context, id string) (*domain.WorkItem, error)
	GetBySeq(ctx context.Context, projectID string, seq int) (*domain.WorkItem, error)
	ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error)
	ListSchedulable(ctx context.Context, includeArchived bool) ([]SchedulableCandidate, error)
	ListCompletedSummaryByProject(ctx context.Context) ([]CompletedWorkSummary, error)
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
	ListBlockedWorkItemIDs(ctx context.Context, candidateIDs []string) (map[string]bool, error)
}

// ProjectWeekMinutes holds aggregated session minutes for a single project in a single ISO week.
type ProjectWeekMinutes struct {
	ProjectName string
	ISOWeek     string // e.g. "2026-W08"
	TotalMin    int
}

type SessionRepo interface {
	Create(ctx context.Context, s *domain.WorkSessionLog) error
	GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error)
	ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error)
	ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error)
	ListRecentByProject(ctx context.Context, projectID string, days int) ([]*domain.WorkSessionLog, error)
	ListRecentSummaryByType(ctx context.Context, days int) ([]domain.SessionSummaryByType, error)
	ListSessionMinutesByWeek(ctx context.Context, from, to time.Time) ([]ProjectWeekMinutes, error)
	Delete(ctx context.Context, id string) error
}

type UserProfileRepo interface {
	Get(ctx context.Context) (*domain.UserProfile, error)
	Upsert(ctx context.Context, p *domain.UserProfile) error
}

// NodeRefRepo tracks the mapping from JSON ref strings to node IDs for project updates.
type NodeRefRepo interface {
	Set(ctx context.Context, nodeID, projectID, ref string) error
	GetByProjectAndRef(ctx context.Context, projectID, ref string) (string, error)
	DeleteByNodeID(ctx context.Context, nodeID string) error
}

// WorkItemRefRepo tracks the mapping from JSON ref strings to work item IDs for project updates.
type WorkItemRefRepo interface {
	Set(ctx context.Context, workItemID, projectID, ref string) error
	GetByProjectAndRef(ctx context.Context, projectID, ref string) (string, error)
	DeleteByWorkItemID(ctx context.Context, workItemID string) error
}

// WorkoutLogRepo manages workout log persistence.
type WorkoutLogRepo interface {
	Create(ctx context.Context, log *domain.WorkoutLog) error
	Delete(ctx context.Context, id string) error
	ListByDateRange(ctx context.Context, from, to time.Time) ([]domain.WorkoutLog, error)
	ListRecent(ctx context.Context, limit int) ([]domain.WorkoutLog, error)
}

// HabitRepo manages recurring habit persistence.
type HabitRepo interface {
	Create(ctx context.Context, h *domain.Habit) error
	ListActive(ctx context.Context) ([]*domain.Habit, error)
	GetByID(ctx context.Context, id string) (*domain.Habit, error)
	Archive(ctx context.Context, id string, now time.Time) error
	LogSession(ctx context.Context, log *domain.HabitLog) error
	DeleteLog(ctx context.Context, logID string) error
	LastLog(ctx context.Context, habitID string) (*domain.HabitLog, error)
	ListLogs(ctx context.Context, habitID string, limit int) ([]domain.HabitLog, error)
}

// TaskRepo manages global (non-project-scoped) task checklist persistence.
type TaskRepo interface {
	Create(ctx context.Context, t *domain.Task) error
	GetByID(ctx context.Context, id string) (*domain.Task, error)
	ListActive(ctx context.Context) ([]*domain.Task, error)
	Update(ctx context.Context, t *domain.Task) error
	Archive(ctx context.Context, id string, now time.Time) error
	Delete(ctx context.Context, id string) error
	SwapOrder(ctx context.Context, idA, idB string) error
}

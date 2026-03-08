package service

import (
	"context"
	"time"

	"github.com/alexanderramin/kairos/internal/app"
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
	GetBySeq(ctx context.Context, projectID string, seq int) (*domain.PlanNode, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	ListChildren(ctx context.Context, parentID string) ([]*domain.PlanNode, error)
	ListRoots(ctx context.Context, projectID string) ([]*domain.PlanNode, error)
	Update(ctx context.Context, n *domain.PlanNode) error
	Delete(ctx context.Context, id string) error
}

type WorkItemService interface {
	Create(ctx context.Context, w *domain.WorkItem) error
	GetByID(ctx context.Context, id string) (*domain.WorkItem, error)
	GetBySeq(ctx context.Context, projectID string, seq int) (*domain.WorkItem, error)
	ListByNode(ctx context.Context, nodeID string) ([]*domain.WorkItem, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.WorkItem, error)
	Update(ctx context.Context, w *domain.WorkItem) error
	MarkDone(ctx context.Context, id string) error
	Reopen(ctx context.Context, id string) error
	MarkInProgress(ctx context.Context, id string) error
	Archive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type SessionService interface {
	LogSession(ctx context.Context, s *domain.WorkSessionLog) error
	GetByID(ctx context.Context, id string) (*domain.WorkSessionLog, error)
	ListByWorkItem(ctx context.Context, workItemID string) ([]*domain.WorkSessionLog, error)
	ListRecent(ctx context.Context, days int) ([]*domain.WorkSessionLog, error)
	ListRecentSummaryByType(ctx context.Context, days int) ([]domain.SessionSummaryByType, error)
	Delete(ctx context.Context, id string) error
}

type WhatNowService interface {
	Recommend(ctx context.Context, req app.WhatNowRequest) (*app.WhatNowResponse, error)
}

type StatusService interface {
	GetStatus(ctx context.Context, req app.StatusRequest) (*app.StatusResponse, error)
}

type ReplanService interface {
	Replan(ctx context.Context, req app.ReplanRequest) (*app.ReplanResponse, error)
}

type TemplateService interface {
	List(ctx context.Context) ([]domain.Template, error)
	Get(ctx context.Context, name string) (*domain.Template, error)
	InitProject(ctx context.Context, templateName string, projectName string, shortID string, startDate string, dueDate *string, vars map[string]string) (*domain.Project, error)
}

type ImportResult = app.ImportResult

type ImportService interface {
	ImportProject(ctx context.Context, filePath string) (*ImportResult, error)
	ImportProjectFromSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error)
}

type ProjectUpdateResult = app.ProjectUpdateResult

type ProjectUpdateService interface {
	UpdateProjectFromJSON(ctx context.Context, projectShortID string, filePath string) (*ProjectUpdateResult, error)
}

type ChartService interface {
	WeeklyBreakdown(ctx context.Context, numWeeks int) ([]domain.WeeklyBreakdown, error)
}

// LogWorkoutRequest holds parameters for logging a workout.
type LogWorkoutRequest struct {
	Category    domain.WorkoutCategory
	Minutes     int
	PerformedAt *time.Time
	Notes       *string
}

type WorkoutService interface {
	LogWorkout(ctx context.Context, req LogWorkoutRequest) (domain.WorkoutLog, error)
	DeleteWorkout(ctx context.Context, id string) error
	ListRecent(ctx context.Context, limit int) ([]domain.WorkoutLog, error)
	ListByDateRange(ctx context.Context, from, to time.Time) ([]domain.WorkoutLog, error)
}

// AddTaskRequest holds parameters for creating a new global task.
type AddTaskRequest struct {
	Title       string
	Description string
}

// TaskService manages the global standalone task checklist.
type TaskService interface {
	Add(ctx context.Context, req AddTaskRequest) (*domain.Task, error)
	ListActive(ctx context.Context) ([]*domain.Task, error)
	Update(ctx context.Context, id, title, description string) error
	MarkDone(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	MoveUp(ctx context.Context, id string) error
	MoveDown(ctx context.Context, id string) error
}

package app

import (
	"context"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
)

type StatusUseCase interface {
	GetStatus(ctx context.Context, req StatusRequest) (*StatusResponse, error)
}

type WhatNowUseCase interface {
	Recommend(ctx context.Context, req WhatNowRequest) (*WhatNowResponse, error)
}

type ReplanUseCase interface {
	Replan(ctx context.Context, req ReplanRequest) (*ReplanResponse, error)
}

type LogSessionUseCase interface {
	LogSession(ctx context.Context, s *domain.WorkSessionLog) error
}

type InitProjectUseCase interface {
	InitProject(ctx context.Context, templateName string, projectName string, shortID string, startDate string, dueDate *string, vars map[string]string) (*domain.Project, error)
}

type ImportResult struct {
	Project         *domain.Project
	NodeCount       int
	WorkItemCount   int
	DependencyCount int
}

type ImportProjectUseCase interface {
	ImportProject(ctx context.Context, filePath string) (*ImportResult, error)
	ImportProjectFromSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error)
}

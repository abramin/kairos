package service

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
)

type importService struct {
	projects  repository.ProjectRepo
	nodes     repository.PlanNodeRepo
	workItems repository.WorkItemRepo
	deps      repository.DependencyRepo
}

func NewImportService(
	projects repository.ProjectRepo,
	nodes repository.PlanNodeRepo,
	workItems repository.WorkItemRepo,
	deps repository.DependencyRepo,
) ImportService {
	return &importService{
		projects:  projects,
		nodes:     nodes,
		workItems: workItems,
		deps:      deps,
	}
}

func (s *importService) ImportProject(ctx context.Context, filePath string) (*ImportResult, error) {
	schema, err := importer.LoadImportSchema(filePath)
	if err != nil {
		return nil, fmt.Errorf("loading import file: %w", err)
	}
	return s.importSchema(ctx, schema)
}

func (s *importService) ImportProjectFromSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error) {
	return s.importSchema(ctx, schema)
}

func (s *importService) importSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error) {
	if errs := importer.ValidateImportSchema(schema); len(errs) > 0 {
		return nil, formatValidationErrors(errs)
	}

	generated, err := importer.Convert(schema)
	if err != nil {
		return nil, fmt.Errorf("converting import schema: %w", err)
	}

	if err := s.projects.Create(ctx, generated.Project); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	for _, node := range generated.Nodes {
		if err := s.nodes.Create(ctx, node); err != nil {
			return nil, fmt.Errorf("creating node %q: %w", node.Title, err)
		}
	}

	for _, wi := range generated.WorkItems {
		if err := s.workItems.Create(ctx, wi); err != nil {
			return nil, fmt.Errorf("creating work item %q: %w", wi.Title, err)
		}
	}

	for _, dep := range generated.Dependencies {
		if err := s.deps.Create(ctx, &dep); err != nil {
			return nil, fmt.Errorf("creating dependency: %w", err)
		}
	}

	return &ImportResult{
		Project:         generated.Project,
		NodeCount:       len(generated.Nodes),
		WorkItemCount:   len(generated.WorkItems),
		DependencyCount: len(generated.Dependencies),
	}, nil
}

func formatValidationErrors(errs []error) error {
	msg := fmt.Sprintf("import validation failed (%d errors):", len(errs))
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}

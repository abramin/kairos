package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
)

type importService struct {
	uow      db.UnitOfWork
	observer UseCaseObserver
}

func NewImportService(
	uow db.UnitOfWork,
	observers ...UseCaseObserver,
) ImportService {
	return &importService{
		uow:      uow,
		observer: useCaseObserverOrNoop(observers),
	}
}

func (s *importService) ImportProject(ctx context.Context, filePath string) (*ImportResult, error) {
	schema, err := importer.LoadImportSchema(filePath)
	if err != nil {
		return nil, fmt.Errorf("loading import file: %w", err)
	}
	return s.importSchema(ctx, schema, "file")
}

func (s *importService) ImportProjectFromSchema(ctx context.Context, schema *importer.ImportSchema) (*ImportResult, error) {
	return s.importSchema(ctx, schema, "schema")
}

func (s *importService) importSchema(ctx context.Context, schema *importer.ImportSchema, source string) (result *ImportResult, err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"source": source,
	}
	defer func() {
		if result != nil {
			fields["node_count"] = result.NodeCount
			fields["work_item_count"] = result.WorkItemCount
			fields["dependency_count"] = result.DependencyCount
		}
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "import-project",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	if errs := importer.ValidateImportSchema(schema); len(errs) > 0 {
		return nil, formatValidationErrors(errs)
	}

	generated, err := importer.Convert(schema)
	if err != nil {
		return nil, fmt.Errorf("converting import schema: %w", err)
	}

	err = s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txProjects := repository.NewSQLiteProjectRepo(tx)
		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		txDeps := repository.NewSQLiteDependencyRepo(tx)

		if err := txProjects.Create(ctx, generated.Project); err != nil {
			return fmt.Errorf("creating project: %w", err)
		}

		for _, node := range generated.Nodes {
			if err := txNodes.Create(ctx, node); err != nil {
				return fmt.Errorf("creating node %q: %w", node.Title, err)
			}
		}

		for _, wi := range generated.WorkItems {
			if err := txWorkItems.Create(ctx, wi); err != nil {
				return fmt.Errorf("creating work item %q: %w", wi.Title, err)
			}
		}

		for _, dep := range generated.Dependencies {
			if err := txDeps.Create(ctx, &dep); err != nil {
				return fmt.Errorf("creating dependency: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	result = &ImportResult{
		Project:         generated.Project,
		NodeCount:       len(generated.Nodes),
		WorkItemCount:   len(generated.WorkItems),
		DependencyCount: len(generated.Dependencies),
	}
	return result, nil
}

func formatValidationErrors(errs []error) error {
	msg := fmt.Sprintf("import validation failed (%d errors):", len(errs))
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}

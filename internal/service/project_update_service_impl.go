package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
)

type projectUpdateService struct {
	uow      db.UnitOfWork
	observer UseCaseObserver
}

func NewProjectUpdateService(
	uow db.UnitOfWork,
	observers ...UseCaseObserver,
) ProjectUpdateService {
	return &projectUpdateService{
		uow:      uow,
		observer: useCaseObserverOrNoop(observers),
	}
}

func (s *projectUpdateService) UpdateProjectFromJSON(
	ctx context.Context,
	projectShortID string,
	filePath string,
) (result *ProjectUpdateResult, err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"project_short_id": projectShortID,
	}
	defer func() {
		if result != nil {
			fields["nodes_created"] = result.NodesCreated
			fields["nodes_updated"] = result.NodesUpdated
			fields["work_items_created"] = result.WorkItemsCreated
			fields["work_items_updated"] = result.WorkItemsUpdated
			fields["dependencies_added"] = result.DependenciesAdded
		}
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "project-update",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	// Load and validate schema
	schema, err := importer.LoadImportSchema(filePath)
	if err != nil {
		return nil, fmt.Errorf("loading import file: %w", err)
	}

	if errs := importer.ValidateImportSchema(schema); len(errs) > 0 {
		return nil, formatValidationErrors(errs)
	}

	now := time.Now().UTC()

	// Apply updates within transaction
	err = s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		// Initialize repository instances
		txProjects := repository.NewSQLiteProjectRepo(tx)
		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		txDeps := repository.NewSQLiteDependencyRepo(tx)
		txNodeRefs := repository.NewSQLiteNodeRefRepo(tx)
		txWIRefs := repository.NewSQLiteWorkItemRefRepo(tx)

		// Load existing project
		project, err := txProjects.GetByShortID(ctx, projectShortID)
		if err != nil {
			return fmt.Errorf("project with short_id %q not found: %w", projectShortID, err)
		}

		// Apply node updates and get refMap
		nodeStats, nodeRefMap, err := importer.ApplyNodeUpdates(
			ctx, txNodes, txNodeRefs, project.ID, schema.Nodes, now,
		)
		if err != nil {
			return err
		}

		// Build full refMap for work items (node refs already resolved)
		refMap := make(map[string]string)
		for ref, nodeID := range nodeRefMap {
			refMap[ref] = nodeID
		}

		// Apply work item updates
		wiStats, err := importer.ApplyWorkItemUpdates(
			ctx, txWorkItems, txWIRefs, project.ID, schema.WorkItems, refMap, schema.Defaults, now,
		)
		if err != nil {
			return err
		}

		// Merge dependencies; need to rebuild refMap to include work items
		// First, collect all work item refs from the schema
		for _, wi := range schema.WorkItems {
			wiID, err := txWIRefs.GetByProjectAndRef(ctx, project.ID, wi.Ref)
			if err != nil {
				return fmt.Errorf("looking up work item ref %q: %w", wi.Ref, err)
			}
			if wiID != "" {
				refMap[wi.Ref] = wiID
			}
		}

		depsAdded, err := importer.MergeDependencies(ctx, txDeps, schema.Dependencies, refMap)
		if err != nil {
			return err
		}

		result = &ProjectUpdateResult{
			Project:              project,
			NodesCreated:         nodeStats.Created,
			NodesUpdated:         nodeStats.Updated,
			WorkItemsCreated:     wiStats.Created,
			WorkItemsUpdated:     wiStats.Updated,
			DependenciesAdded:    depsAdded,
			DependenciesPreserved: 0, // We don't track preserved dependencies explicitly
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

package importer

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/generation"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

// UpdateStats tracks counts of created vs updated entities.
type UpdateStats struct {
	Created int
	Updated int
}

// ApplyNodeUpdates updates or creates nodes based on the import schema.
// For each node in schemaNodes:
// - If a node with matching ref exists in the project, update its metadata
// - Otherwise, create a new node and record the ref
// Returns refMap: ref string → node UUID for use in work item updates.
func ApplyNodeUpdates(
	ctx context.Context,
	nodeRepo repository.PlanNodeRepo,
	nodeRefRepo repository.NodeRefRepo,
	projectID string,
	schemaNodes []NodeImport,
	now time.Time,
) (*UpdateStats, map[string]string, error) {
	refMap := make(map[string]string)
	stats := &UpdateStats{}

	// Pre-load existing nodes for title-based fallback matching
	existingNodes, err := nodeRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading existing nodes: %w", err)
	}
	titleToNodeID := make(map[string]string, len(existingNodes))
	for _, n := range existingNodes {
		titleToNodeID[n.Title] = n.ID
	}

	for _, sNode := range schemaNodes {
		// Try to find existing node by ref
		existingID, err := nodeRefRepo.GetByProjectAndRef(ctx, projectID, sNode.Ref)
		if err != nil {
			return nil, nil, fmt.Errorf("looking up node ref %q: %w", sNode.Ref, err)
		}

		// Fallback: match by title if no ref found
		if existingID == "" {
			if id, ok := titleToNodeID[sNode.Title]; ok {
				existingID = id
				// Backfill the ref for future updates
				if err := nodeRefRepo.Set(ctx, existingID, projectID, sNode.Ref); err != nil {
					return nil, nil, fmt.Errorf("backfilling node ref %q: %w", sNode.Ref, err)
				}
			}
		}

		if existingID != "" {
			// UPDATE existing node
			node, err := nodeRepo.GetByID(ctx, existingID)
			if err != nil {
				return nil, nil, fmt.Errorf("loading existing node %s: %w", existingID, err)
			}

			// Update mutable fields only; preserve archived_at, created_at, seq
			node.Title = sNode.Title
			node.Kind = domain.NodeKind(sNode.Kind)
			node.OrderIndex = sNode.Order

			// Parse optional dates (already pointers in schema)
			if sNode.DueDate != nil {
				dueDate, err := time.Parse("2006-01-02", *sNode.DueDate)
				if err == nil {
					node.DueDate = &dueDate
				}
			} else {
				node.DueDate = nil
			}

			if sNode.NotBefore != nil {
				nbDate, err := time.Parse("2006-01-02", *sNode.NotBefore)
				if err == nil {
					node.NotBefore = &nbDate
				}
			} else {
				node.NotBefore = nil
			}

			if sNode.NotAfter != nil {
				naDate, err := time.Parse("2006-01-02", *sNode.NotAfter)
				if err == nil {
					node.NotAfter = &naDate
				}
			} else {
				node.NotAfter = nil
			}

			node.PlannedMinBudget = sNode.PlannedMinBudget
			node.UpdatedAt = now

			if err := nodeRepo.Update(ctx, node); err != nil {
				return nil, nil, fmt.Errorf("updating node %s: %w", existingID, err)
			}
			stats.Updated++
			refMap[sNode.Ref] = existingID
		} else {
			// CREATE new node
			newID := uuid.New().String()
			node := &domain.PlanNode{
				ID:        newID,
				ProjectID: projectID,
				Title:     sNode.Title,
				Kind:      domain.NodeKind(sNode.Kind),
				OrderIndex: sNode.Order,
			}

			// Parse optional dates (already pointers in schema)
			if sNode.DueDate != nil {
				dueDate, err := time.Parse("2006-01-02", *sNode.DueDate)
				if err == nil {
					node.DueDate = &dueDate
				}
			}

			if sNode.NotBefore != nil {
				nbDate, err := time.Parse("2006-01-02", *sNode.NotBefore)
				if err == nil {
					node.NotBefore = &nbDate
				}
			}

			if sNode.NotAfter != nil {
				naDate, err := time.Parse("2006-01-02", *sNode.NotAfter)
				if err == nil {
					node.NotAfter = &naDate
				}
			}

			node.PlannedMinBudget = sNode.PlannedMinBudget
			node.CreatedAt = now
			node.UpdatedAt = now

			if err := nodeRepo.Create(ctx, node); err != nil {
				return nil, nil, fmt.Errorf("creating node %q: %w", sNode.Title, err)
			}

			// Record the ref mapping
			if err := nodeRefRepo.Set(ctx, newID, projectID, sNode.Ref); err != nil {
				return nil, nil, fmt.Errorf("recording node ref %q: %w", sNode.Ref, err)
			}

			stats.Created++
			refMap[sNode.Ref] = newID
		}
	}

	return stats, refMap, nil
}

// ApplyWorkItemUpdates updates or creates work items based on the import schema.
// For each work item in schemaItems:
// - If a work item with matching ref exists in the project, update its metadata (preserving LoggedMin and Status)
// - Otherwise, create a new work item and record the ref
// nodeRefMap should contain the mapping from node refs to node UUIDs (from ApplyNodeUpdates).
func ApplyWorkItemUpdates(
	ctx context.Context,
	wiRepo repository.WorkItemRepo,
	wiRefRepo repository.WorkItemRefRepo,
	projectID string,
	schemaItems []WorkItemImport,
	nodeRefMap map[string]string,
	defaults *DefaultsImport,
	now time.Time,
) (*UpdateStats, error) {
	stats := &UpdateStats{}

	// Pre-load existing work items for title-based fallback matching
	existingItems, err := wiRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("loading existing work items: %w", err)
	}
	type wiKey struct{ nodeID, title string }
	titleToWIID := make(map[wiKey]string, len(existingItems))
	for _, wi := range existingItems {
		titleToWIID[wiKey{wi.NodeID, wi.Title}] = wi.ID
	}

	for _, sWI := range schemaItems {
		// Resolve node_ref to node UUID
		nodeID, ok := nodeRefMap[sWI.NodeRef]
		if !ok {
			return nil, fmt.Errorf("node_ref %q not found for work item %q", sWI.NodeRef, sWI.Ref)
		}

		// Try to find existing work item by ref
		existingID, err := wiRefRepo.GetByProjectAndRef(ctx, projectID, sWI.Ref)
		if err != nil {
			return nil, fmt.Errorf("looking up work item ref %q: %w", sWI.Ref, err)
		}

		// Fallback: match by title + node if no ref found
		if existingID == "" {
			if id, ok := titleToWIID[wiKey{nodeID, sWI.Title}]; ok {
				existingID = id
				// Backfill the ref for future updates
				if err := wiRefRepo.Set(ctx, existingID, projectID, sWI.Ref); err != nil {
					return nil, fmt.Errorf("backfilling work item ref %q: %w", sWI.Ref, err)
				}
			}
		}

		if existingID != "" {
			// UPDATE existing work item (preserve LoggedMin, Status, CreatedAt, sessions)
			item, err := wiRepo.GetByID(ctx, existingID)
			if err != nil {
				return nil, fmt.Errorf("loading existing work item %s: %w", existingID, err)
			}

			// Update metadata; preserve LoggedMin, Status, CreatedAt
			item.Title = sWI.Title
			item.Type = sWI.Type
			item.Description = "" // JSON schema doesn't include description yet

			// Resolve duration defaults
			resolved := generation.ResolveWorkItemDefaults(
				generation.WorkItemDefaultsInput{
					DurationMode:        sWI.DurationMode,
					SessionPolicy:       sWI.SessionPolicy,
					PlannedMin:          sWI.PlannedMin,
					EstimateConfidence:  sWI.EstimateConfidence,
				},
				generation.WorkItemDefaultsInput{
					DurationMode:  defaults.DurationMode,
					SessionPolicy: defaults.SessionPolicy,
				},
			)

			item.DurationMode = domain.DurationMode(resolved.DurationMode)
			item.PlannedMin = resolved.PlannedMin
			item.EstimateConfidence = resolved.EstimateConfidence
			item.MinSessionMin = resolved.MinSessionMin
			item.MaxSessionMin = resolved.MaxSessionMin
			item.DefaultSessionMin = resolved.DefaultSessionMin
			item.Splittable = resolved.Splittable
			item.UpdatedAt = now

			// Parse optional due date (already pointers in schema)
			if sWI.DueDate != nil {
				dueDate, err := time.Parse("2006-01-02", *sWI.DueDate)
				if err == nil {
					item.DueDate = &dueDate
				}
			} else {
				item.DueDate = nil
			}

			// Parse optional not_before date (already pointers in schema)
			if sWI.NotBefore != nil {
				nbDate, err := time.Parse("2006-01-02", *sWI.NotBefore)
				if err == nil {
					item.NotBefore = &nbDate
				}
			} else {
				item.NotBefore = nil
			}

			if err := wiRepo.Update(ctx, item); err != nil {
				return nil, fmt.Errorf("updating work item %s: %w", existingID, err)
			}
			stats.Updated++
		} else {
			// CREATE new work item
			newID := uuid.New().String()

			// Resolve duration defaults
			resolved := generation.ResolveWorkItemDefaults(
				generation.WorkItemDefaultsInput{
					DurationMode:        sWI.DurationMode,
					SessionPolicy:       sWI.SessionPolicy,
					PlannedMin:          sWI.PlannedMin,
					EstimateConfidence:  sWI.EstimateConfidence,
				},
				generation.WorkItemDefaultsInput{
					DurationMode:  defaults.DurationMode,
					SessionPolicy: defaults.SessionPolicy,
				},
			)

			item := &domain.WorkItem{
				ID:                  newID,
				NodeID:              nodeID,
				Title:               sWI.Title,
				Type:                sWI.Type,
				Status:              domain.WorkItemTodo,
				DurationMode:        domain.DurationMode(resolved.DurationMode),
				PlannedMin:          resolved.PlannedMin,
				EstimateConfidence:  resolved.EstimateConfidence,
				LoggedMin:           0,
				MinSessionMin:       resolved.MinSessionMin,
				MaxSessionMin:       resolved.MaxSessionMin,
				DefaultSessionMin:   resolved.DefaultSessionMin,
				Splittable:          resolved.Splittable,
				DurationSource:      domain.SourceManual,
				CreatedAt:           now,
				UpdatedAt:           now,
			}

			// Parse optional due date (already pointers in schema)
			if sWI.DueDate != nil {
				dueDate, err := time.Parse("2006-01-02", *sWI.DueDate)
				if err == nil {
					item.DueDate = &dueDate
				}
			}

			// Parse optional not_before date (already pointers in schema)
			if sWI.NotBefore != nil {
				nbDate, err := time.Parse("2006-01-02", *sWI.NotBefore)
				if err == nil {
					item.NotBefore = &nbDate
				}
			}

			if err := wiRepo.Create(ctx, item); err != nil {
				return nil, fmt.Errorf("creating work item %q: %w", sWI.Title, err)
			}

			// Record the ref mapping
			if err := wiRefRepo.Set(ctx, newID, projectID, sWI.Ref); err != nil {
				return nil, fmt.Errorf("recording work item ref %q: %w", sWI.Ref, err)
			}

			stats.Created++
		}
	}

	return stats, nil
}

// MergeDependencies adds new dependencies from the schema, preserving existing ones.
// Only adds dependencies that don't already exist.
// refMap should map dependency ref strings to work item UUIDs.
func MergeDependencies(
	ctx context.Context,
	depRepo repository.DependencyRepo,
	schemaDeps []DependencyImport,
	refMap map[string]string,
) (int, error) {
	added := 0

	for _, sDep := range schemaDeps {
		predID, ok := refMap[sDep.PredecessorRef]
		if !ok {
			return 0, fmt.Errorf("predecessor_ref %q not found in ref map", sDep.PredecessorRef)
		}

		succID, ok := refMap[sDep.SuccessorRef]
		if !ok {
			return 0, fmt.Errorf("successor_ref %q not found in ref map", sDep.SuccessorRef)
		}

		// Check if dependency already exists
		successors, err := depRepo.ListSuccessors(ctx, predID)
		if err != nil {
			return 0, fmt.Errorf("listing successors of %s: %w", predID, err)
		}

		alreadyExists := false
		for _, succ := range successors {
			if succ.SuccessorWorkItemID == succID {
				alreadyExists = true
				break
			}
		}

		if !alreadyExists {
			dep := &domain.Dependency{
				PredecessorWorkItemID: predID,
				SuccessorWorkItemID:   succID,
			}
			if err := depRepo.Create(ctx, dep); err != nil {
				return 0, fmt.Errorf("creating dependency %s → %s: %w", predID, succID, err)
			}
			added++
		}
	}

	return added, nil
}

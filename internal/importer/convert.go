package importer

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/generation"
	tmpl "github.com/alexanderramin/kairos/internal/template"
	"github.com/google/uuid"
)

// Convert transforms a validated ImportSchema into domain objects ready for persistence.
// Call ValidateImportSchema first; Convert assumes the schema is valid.
func Convert(schema *ImportSchema) (*tmpl.GeneratedProject, error) {
	now := time.Now().UTC()

	project, err := convertProject(schema, now)
	if err != nil {
		return nil, err
	}

	refMap := make(map[string]string) // ref -> UUID
	nodes, err := convertNodes(schema.Nodes, project.ID, refMap, now)
	if err != nil {
		return nil, err
	}

	workItems, err := convertWorkItems(schema, refMap, now)
	if err != nil {
		return nil, err
	}

	generation.AssignSequentialIDs(nodes, workItems)

	deps, err := convertDependencies(schema, refMap)
	if err != nil {
		return nil, err
	}

	return &tmpl.GeneratedProject{
		Project:      project,
		Nodes:        nodes,
		WorkItems:    workItems,
		Dependencies: deps,
	}, nil
}

// convertProject parses dates and builds the domain.Project.
func convertProject(schema *ImportSchema, now time.Time) (*domain.Project, error) {
	startDate, err := generation.ParseRequiredDate(schema.Project.StartDate, "project.start_date")
	if err != nil {
		return nil, err
	}

	targetDate, err := generation.ParseOptionalDate(schema.Project.TargetDate, "project.target_date")
	if err != nil {
		return nil, err
	}

	return &domain.Project{
		ID:         uuid.New().String(),
		ShortID:    strings.ToUpper(schema.Project.ShortID),
		Name:       schema.Project.Name,
		Domain:     schema.Project.Domain,
		StartDate:  startDate,
		TargetDate: targetDate,
		Status:     domain.ProjectActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// convertNodes builds domain.PlanNode objects from schema nodes, populating refMap.
func convertNodes(schemaNodes []NodeImport, projectID string, refMap map[string]string, now time.Time) ([]*domain.PlanNode, error) {
	nodes := make([]*domain.PlanNode, 0, len(schemaNodes))
	for i, n := range schemaNodes {
		realID := uuid.New().String()
		refMap[n.Ref] = realID

		var parentID *string
		if n.ParentRef != nil && *n.ParentRef != "" {
			if pid, ok := refMap[*n.ParentRef]; ok {
				parentID = &pid
			}
		}

		kind := n.Kind
		if kind == "" {
			kind = string(domain.NodeGeneric)
		}

		dueDate, err := generation.ParseOptionalDate(n.DueDate, fmt.Sprintf("nodes[%d].due_date", i))
		if err != nil {
			return nil, err
		}
		notBefore, err := generation.ParseOptionalDate(n.NotBefore, fmt.Sprintf("nodes[%d].not_before", i))
		if err != nil {
			return nil, err
		}
		notAfter, err := generation.ParseOptionalDate(n.NotAfter, fmt.Sprintf("nodes[%d].not_after", i))
		if err != nil {
			return nil, err
		}

		node := &domain.PlanNode{
			ID:               realID,
			ProjectID:        projectID,
			ParentID:         parentID,
			Title:            n.Title,
			Kind:             domain.NodeKind(kind),
			OrderIndex:       n.Order,
			DueDate:          dueDate,
			NotBefore:        notBefore,
			NotAfter:         notAfter,
			PlannedMinBudget: n.PlannedMinBudget,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// convertWorkItems builds domain.WorkItem objects from schema work items, applying defaults cascade.
func convertWorkItems(schema *ImportSchema, refMap map[string]string, now time.Time) ([]*domain.WorkItem, error) {
	workItems := make([]*domain.WorkItem, 0, len(schema.WorkItems))
	defaults := generation.WorkItemDefaultsInput{}
	if schema.Defaults != nil {
		defaults.DurationMode = schema.Defaults.DurationMode
		defaults.SessionPolicy = schema.Defaults.SessionPolicy
	}
	for i, wi := range schema.WorkItems {
		realID := uuid.New().String()
		refMap[wi.Ref] = realID

		nodeUUID, ok := refMap[wi.NodeRef]
		if !ok {
			return nil, fmt.Errorf("node_ref %q not found for work item %q", wi.NodeRef, wi.Ref)
		}

		// Apply defaults cascade: work item field > schema defaults > hardcoded
		resolved := generation.ResolveWorkItemDefaults(
			generation.WorkItemDefaultsInput{
				DurationMode:       wi.DurationMode,
				SessionPolicy:      wi.SessionPolicy,
				PlannedMin:         wi.PlannedMin,
				EstimateConfidence: wi.EstimateConfidence,
			},
			defaults,
		)
		status := wi.Status
		if status == "" {
			status = "todo"
		}

		var unitsKind string
		var unitsTotal int
		if wi.Units != nil {
			unitsKind = wi.Units.Kind
			unitsTotal = wi.Units.Total
		}

		// Resolve logged_min: explicit value > auto-fill for done items > 0
		loggedMin := 0
		if wi.LoggedMin != nil {
			loggedMin = *wi.LoggedMin
		} else if status == "done" && resolved.PlannedMin > 0 {
			loggedMin = resolved.PlannedMin
		}

		dueDate, err := generation.ParseOptionalDate(wi.DueDate, fmt.Sprintf("work_items[%d].due_date", i))
		if err != nil {
			return nil, err
		}
		notBefore, err := generation.ParseOptionalDate(wi.NotBefore, fmt.Sprintf("work_items[%d].not_before", i))
		if err != nil {
			return nil, err
		}

		item := &domain.WorkItem{
			ID:                 realID,
			NodeID:             nodeUUID,
			Title:              wi.Title,
			Type:               wi.Type,
			Status:             domain.WorkItemStatus(status),
			DurationMode:       domain.DurationMode(resolved.DurationMode),
			PlannedMin:         resolved.PlannedMin,
			LoggedMin:          loggedMin,
			DurationSource:     domain.SourceManual,
			EstimateConfidence: resolved.EstimateConfidence,
			MinSessionMin:      resolved.MinSessionMin,
			MaxSessionMin:      resolved.MaxSessionMin,
			DefaultSessionMin:  resolved.DefaultSessionMin,
			Splittable:         resolved.Splittable,
			UnitsKind:          unitsKind,
			UnitsTotal:         unitsTotal,
			DueDate:            dueDate,
			NotBefore:          notBefore,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		workItems = append(workItems, item)
	}
	return workItems, nil
}

// convertDependencies builds explicit dependencies or infers a default linear chain.
func convertDependencies(schema *ImportSchema, refMap map[string]string) ([]domain.Dependency, error) {
	if len(schema.Dependencies) == 0 {
		return inferImportDeps(schema.Nodes, schema.WorkItems, refMap), nil
	}

	var deps []domain.Dependency
	for _, d := range schema.Dependencies {
		predUUID, ok := refMap[d.PredecessorRef]
		if !ok {
			return nil, fmt.Errorf("predecessor_ref %q not found", d.PredecessorRef)
		}
		succUUID, ok := refMap[d.SuccessorRef]
		if !ok {
			return nil, fmt.Errorf("successor_ref %q not found", d.SuccessorRef)
		}
		deps = append(deps, domain.Dependency{
			PredecessorWorkItemID: predUUID,
			SuccessorWorkItemID:   succUUID,
		})
	}
	return deps, nil
}

// inferImportDeps builds DependencyCandidate entries from import schema types
// and delegates to the shared InferLinearDependencies algorithm.
func inferImportDeps(nodes []NodeImport, workItems []WorkItemImport, refMap map[string]string) []domain.Dependency {
	nodePos := make(map[string]int, len(nodes))
	nodeOrder := make(map[string]int, len(nodes))
	for i, n := range nodes {
		nodePos[n.Ref] = i
		nodeOrder[n.Ref] = n.Order
	}

	candidates := make([]generation.DependencyCandidate, 0, len(workItems))
	for i, wi := range workItems {
		candidates = append(candidates, generation.DependencyCandidate{
			ID:        refMap[wi.Ref],
			NodeOrder: nodeOrder[wi.NodeRef],
			NodePos:   nodePos[wi.NodeRef],
			ItemPos:   i,
		})
	}

	return generation.InferLinearDependencies(candidates)
}

package importer

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
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
	nodes := convertNodes(schema.Nodes, project.ID, refMap, now)

	workItems, err := convertWorkItems(schema, refMap, now)
	if err != nil {
		return nil, err
	}

	assignSequentialIDs(nodes, workItems)

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
	startDate, err := time.Parse("2006-01-02", schema.Project.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date: %w", err)
	}

	var targetDate *time.Time
	if schema.Project.TargetDate != nil {
		t, err := time.Parse("2006-01-02", *schema.Project.TargetDate)
		if err != nil {
			return nil, fmt.Errorf("parsing target_date: %w", err)
		}
		targetDate = &t
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
func convertNodes(schemaNodes []NodeImport, projectID string, refMap map[string]string, now time.Time) []*domain.PlanNode {
	nodes := make([]*domain.PlanNode, 0, len(schemaNodes))
	for _, n := range schemaNodes {
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

		node := &domain.PlanNode{
			ID:               realID,
			ProjectID:        projectID,
			ParentID:         parentID,
			Title:            n.Title,
			Kind:             domain.NodeKind(kind),
			OrderIndex:       n.Order,
			DueDate:          parseOptionalDate(n.DueDate),
			NotBefore:        parseOptionalDate(n.NotBefore),
			NotAfter:         parseOptionalDate(n.NotAfter),
			PlannedMinBudget: n.PlannedMinBudget,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// convertWorkItems builds domain.WorkItem objects from schema work items, applying defaults cascade.
func convertWorkItems(schema *ImportSchema, refMap map[string]string, now time.Time) ([]*domain.WorkItem, error) {
	workItems := make([]*domain.WorkItem, 0, len(schema.WorkItems))
	for _, wi := range schema.WorkItems {
		realID := uuid.New().String()
		refMap[wi.Ref] = realID

		nodeUUID, ok := refMap[wi.NodeRef]
		if !ok {
			return nil, fmt.Errorf("node_ref %q not found for work item %q", wi.NodeRef, wi.Ref)
		}

		// Apply defaults cascade: work item field > schema defaults > hardcoded
		durationMode := domain.CoalesceStr(wi.DurationMode, defaultDurationMode(schema.Defaults), "estimate")
		status := wi.Status
		if status == "" {
			status = "todo"
		}

		minSession := domain.IntFromPtrWithDefault(15,
			tmpl.SessionPolicyField(wi.SessionPolicy, "min"),
			tmpl.SessionPolicyField(defaultSessionPolicy(schema.Defaults), "min"))
		maxSession := domain.IntFromPtrWithDefault(60,
			tmpl.SessionPolicyField(wi.SessionPolicy, "max"),
			tmpl.SessionPolicyField(defaultSessionPolicy(schema.Defaults), "max"))
		defSession := domain.IntFromPtrWithDefault(30,
			tmpl.SessionPolicyField(wi.SessionPolicy, "default"),
			tmpl.SessionPolicyField(defaultSessionPolicy(schema.Defaults), "default"))
		splittable := domain.BoolFromPtrWithDefault(true,
			tmpl.SessionPolicySplittable(wi.SessionPolicy),
			tmpl.SessionPolicySplittable(defaultSessionPolicy(schema.Defaults)))

		plannedMin := domain.IntFromPtrWithDefault(0, wi.PlannedMin)
		estimateConf := domain.Float64FromPtrWithDefault(0.5, wi.EstimateConfidence)

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
		} else if status == "done" && plannedMin > 0 {
			loggedMin = plannedMin
		}

		item := &domain.WorkItem{
			ID:                 realID,
			NodeID:             nodeUUID,
			Title:              wi.Title,
			Type:               wi.Type,
			Status:             domain.WorkItemStatus(status),
			DurationMode:       domain.DurationMode(durationMode),
			PlannedMin:         plannedMin,
			LoggedMin:          loggedMin,
			DurationSource:     domain.SourceManual,
			EstimateConfidence: estimateConf,
			MinSessionMin:      minSession,
			MaxSessionMin:      maxSession,
			DefaultSessionMin:  defSession,
			Splittable:         splittable,
			UnitsKind:          unitsKind,
			UnitsTotal:         unitsTotal,
			DueDate:            parseOptionalDate(wi.DueDate),
			NotBefore:          parseOptionalDate(wi.NotBefore),
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		workItems = append(workItems, item)
	}
	return workItems, nil
}

// assignSequentialIDs assigns Seq values in tree order: node, then its work items, then next node.
func assignSequentialIDs(nodes []*domain.PlanNode, workItems []*domain.WorkItem) {
	wiByNode := make(map[string][]*domain.WorkItem, len(nodes))
	for _, wi := range workItems {
		wiByNode[wi.NodeID] = append(wiByNode[wi.NodeID], wi)
	}
	seq := 1
	for _, node := range nodes {
		node.Seq = seq
		seq++
		for _, wi := range wiByNode[node.ID] {
			wi.Seq = seq
			seq++
		}
	}
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

func parseOptionalDate(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}

func defaultDurationMode(d *DefaultsImport) string {
	if d != nil {
		return d.DurationMode
	}
	return ""
}

func defaultSessionPolicy(d *DefaultsImport) *SessionPolicyImport {
	if d != nil {
		return d.SessionPolicy
	}
	return nil
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

	candidates := make([]tmpl.DependencyCandidate, 0, len(workItems))
	for i, wi := range workItems {
		candidates = append(candidates, tmpl.DependencyCandidate{
			ID:        refMap[wi.Ref],
			NodeOrder: nodeOrder[wi.NodeRef],
			NodePos:   nodePos[wi.NodeRef],
			ItemPos:   i,
		})
	}

	return tmpl.InferLinearDependencies(candidates)
}

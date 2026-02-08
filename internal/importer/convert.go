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

	project := &domain.Project{
		ID:         uuid.New().String(),
		ShortID:    strings.ToUpper(schema.Project.ShortID),
		Name:       schema.Project.Name,
		Domain:     schema.Project.Domain,
		StartDate:  startDate,
		TargetDate: targetDate,
		Status:     domain.ProjectActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	refMap := make(map[string]string) // ref -> UUID

	// Convert nodes
	nodes := make([]*domain.PlanNode, 0, len(schema.Nodes))
	for _, n := range schema.Nodes {
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
			kind = "generic"
		}

		node := &domain.PlanNode{
			ID:               realID,
			ProjectID:        project.ID,
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

	// Convert work items
	workItems := make([]*domain.WorkItem, 0, len(schema.WorkItems))
	for _, wi := range schema.WorkItems {
		realID := uuid.New().String()
		refMap[wi.Ref] = realID

		nodeUUID, ok := refMap[wi.NodeRef]
		if !ok {
			return nil, fmt.Errorf("node_ref %q not found for work item %q", wi.NodeRef, wi.Ref)
		}

		// Apply defaults cascade: work item field > schema defaults > hardcoded
		durationMode := coalesceStr(wi.DurationMode, defaultDurationMode(schema.Defaults), "estimate")
		status := wi.Status
		if status == "" {
			status = "todo"
		}

		minSession := intFromPtrWithDefault(15,
			sessionPolicyField(wi.SessionPolicy, "min"),
			sessionPolicyField(defaultSessionPolicy(schema.Defaults), "min"))
		maxSession := intFromPtrWithDefault(60,
			sessionPolicyField(wi.SessionPolicy, "max"),
			sessionPolicyField(defaultSessionPolicy(schema.Defaults), "max"))
		defSession := intFromPtrWithDefault(30,
			sessionPolicyField(wi.SessionPolicy, "default"),
			sessionPolicyField(defaultSessionPolicy(schema.Defaults), "default"))
		splittable := boolFromPtrWithDefault(true,
			sessionPolicyBool(wi.SessionPolicy),
			sessionPolicyBool(defaultSessionPolicy(schema.Defaults)))

		plannedMin := intFromPtrWithDefault(0, wi.PlannedMin)
		estimateConf := float64FromPtrWithDefault(0.5, wi.EstimateConfidence)

		var unitsKind string
		var unitsTotal int
		if wi.Units != nil {
			unitsKind = wi.Units.Kind
			unitsTotal = wi.Units.Total
		}

		item := &domain.WorkItem{
			ID:                 realID,
			NodeID:             nodeUUID,
			Title:              wi.Title,
			Type:               wi.Type,
			Status:             domain.WorkItemStatus(status),
			DurationMode:       domain.DurationMode(durationMode),
			PlannedMin:         plannedMin,
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

	// Convert dependencies
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

	return &tmpl.GeneratedProject{
		Project:      project,
		Nodes:        nodes,
		WorkItems:    workItems,
		Dependencies: deps,
	}, nil
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

func coalesceStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func intFromPtrWithDefault(fallback int, ptrs ...*int) int {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

func boolFromPtrWithDefault(fallback bool, ptrs ...*bool) bool {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

func float64FromPtrWithDefault(fallback float64, ptrs ...*float64) float64 {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
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

func sessionPolicyField(sp *SessionPolicyImport, field string) *int {
	if sp == nil {
		return nil
	}
	switch field {
	case "min":
		return sp.MinSessionMin
	case "max":
		return sp.MaxSessionMin
	case "default":
		return sp.DefaultSessionMin
	}
	return nil
}

func sessionPolicyBool(sp *SessionPolicyImport) *bool {
	if sp == nil {
		return nil
	}
	return sp.Splittable
}

package importer

import (
	"fmt"
	"time"
)

var (
	validNodeKinds     = map[string]bool{"week": true, "module": true, "book": true, "stage": true, "section": true, "generic": true}
	validDurationModes = map[string]bool{"fixed": true, "estimate": true, "derived": true}
	validWorkStatuses  = map[string]bool{"todo": true, "in_progress": true, "done": true, "skipped": true, "archived": true}
)

// ValidateImportSchema checks the import schema for errors before conversion.
// Returns a slice of all validation errors found.
func ValidateImportSchema(schema *ImportSchema) []error {
	var errs []error

	errs = append(errs, validateProject(&schema.Project)...)
	errs = append(errs, validateDefaults(schema.Defaults)...)

	nodeRefs := make(map[string]bool)
	errs = append(errs, validateNodes(schema.Nodes, nodeRefs)...)

	wiRefs := make(map[string]bool)
	errs = append(errs, validateWorkItems(schema.WorkItems, nodeRefs, wiRefs)...)

	errs = append(errs, validateDependencies(schema.Dependencies, wiRefs)...)

	return errs
}

func validateProject(p *ProjectImport) []error {
	var errs []error

	if p.ShortID == "" {
		errs = append(errs, fmt.Errorf("project.short_id is required"))
	}
	if p.Name == "" {
		errs = append(errs, fmt.Errorf("project.name is required"))
	}
	if p.Domain == "" {
		errs = append(errs, fmt.Errorf("project.domain is required"))
	}
	if p.StartDate == "" {
		errs = append(errs, fmt.Errorf("project.start_date is required"))
	} else if _, err := time.Parse("2006-01-02", p.StartDate); err != nil {
		errs = append(errs, fmt.Errorf("project.start_date: invalid date format %q (expected YYYY-MM-DD)", p.StartDate))
	}
	if p.TargetDate != nil {
		if _, err := time.Parse("2006-01-02", *p.TargetDate); err != nil {
			errs = append(errs, fmt.Errorf("project.target_date: invalid date format %q (expected YYYY-MM-DD)", *p.TargetDate))
		} else if p.StartDate != "" {
			start, startErr := time.Parse("2006-01-02", p.StartDate)
			target, targetErr := time.Parse("2006-01-02", *p.TargetDate)
			if startErr == nil && targetErr == nil && !target.After(start) {
				errs = append(errs, fmt.Errorf("project.target_date %q must be after start_date %q", *p.TargetDate, p.StartDate))
			}
		}
	}

	return errs
}

func validateDefaults(d *DefaultsImport) []error {
	if d == nil {
		return nil
	}
	var errs []error

	if d.DurationMode != "" && !validDurationModes[d.DurationMode] {
		errs = append(errs, fmt.Errorf("defaults.duration_mode: invalid value %q", d.DurationMode))
	}
	if d.SessionPolicy != nil {
		errs = append(errs, validateSessionPolicy("defaults.session_policy", d.SessionPolicy)...)
	}

	return errs
}

func validateSessionPolicy(prefix string, sp *SessionPolicyImport) []error {
	var errs []error

	if sp.MinSessionMin != nil && *sp.MinSessionMin <= 0 {
		errs = append(errs, fmt.Errorf("%s.min_session_min must be positive", prefix))
	}
	if sp.MaxSessionMin != nil && *sp.MaxSessionMin <= 0 {
		errs = append(errs, fmt.Errorf("%s.max_session_min must be positive", prefix))
	}
	if sp.DefaultSessionMin != nil && *sp.DefaultSessionMin <= 0 {
		errs = append(errs, fmt.Errorf("%s.default_session_min must be positive", prefix))
	}

	minVal := 0
	maxVal := 0
	defVal := 0
	if sp.MinSessionMin != nil {
		minVal = *sp.MinSessionMin
	}
	if sp.MaxSessionMin != nil {
		maxVal = *sp.MaxSessionMin
	}
	if sp.DefaultSessionMin != nil {
		defVal = *sp.DefaultSessionMin
	}

	if minVal > 0 && maxVal > 0 && minVal > maxVal {
		errs = append(errs, fmt.Errorf("%s: min_session_min (%d) must be <= max_session_min (%d)", prefix, minVal, maxVal))
	}
	if defVal > 0 && minVal > 0 && defVal < minVal {
		errs = append(errs, fmt.Errorf("%s: default_session_min (%d) must be >= min_session_min (%d)", prefix, defVal, minVal))
	}
	if defVal > 0 && maxVal > 0 && defVal > maxVal {
		errs = append(errs, fmt.Errorf("%s: default_session_min (%d) must be <= max_session_min (%d)", prefix, defVal, maxVal))
	}

	return errs
}

func validateNodes(nodes []NodeImport, nodeRefs map[string]bool) []error {
	var errs []error

	for i, n := range nodes {
		prefix := fmt.Sprintf("nodes[%d]", i)

		if n.Ref == "" {
			errs = append(errs, fmt.Errorf("%s.ref is required", prefix))
		} else if nodeRefs[n.Ref] {
			errs = append(errs, fmt.Errorf("%s.ref: duplicate ref %q", prefix, n.Ref))
		} else {
			nodeRefs[n.Ref] = true
		}

		if n.Title == "" {
			errs = append(errs, fmt.Errorf("%s.title is required", prefix))
		}
		if n.Kind == "" {
			errs = append(errs, fmt.Errorf("%s.kind is required", prefix))
		} else if !validNodeKinds[n.Kind] {
			errs = append(errs, fmt.Errorf("%s.kind: invalid value %q", prefix, n.Kind))
		}

		if n.ParentRef != nil && *n.ParentRef != "" {
			if !nodeRefs[*n.ParentRef] {
				errs = append(errs, fmt.Errorf("%s.parent_ref: ref %q not found (must appear earlier in nodes list)", prefix, *n.ParentRef))
			}
		}

		errs = append(errs, validateOptionalDate(prefix+".due_date", n.DueDate)...)
		errs = append(errs, validateOptionalDate(prefix+".not_before", n.NotBefore)...)
		errs = append(errs, validateOptionalDate(prefix+".not_after", n.NotAfter)...)
	}

	return errs
}

func validateWorkItems(items []WorkItemImport, nodeRefs map[string]bool, wiRefs map[string]bool) []error {
	var errs []error

	for i, wi := range items {
		prefix := fmt.Sprintf("work_items[%d]", i)

		if wi.Ref == "" {
			errs = append(errs, fmt.Errorf("%s.ref is required", prefix))
		} else if wiRefs[wi.Ref] {
			errs = append(errs, fmt.Errorf("%s.ref: duplicate ref %q", prefix, wi.Ref))
		} else {
			wiRefs[wi.Ref] = true
		}

		if wi.NodeRef == "" {
			errs = append(errs, fmt.Errorf("%s.node_ref is required", prefix))
		} else if !nodeRefs[wi.NodeRef] {
			errs = append(errs, fmt.Errorf("%s.node_ref: ref %q not found in nodes", prefix, wi.NodeRef))
		}

		if wi.Title == "" {
			errs = append(errs, fmt.Errorf("%s.title is required", prefix))
		}
		if wi.Type == "" {
			errs = append(errs, fmt.Errorf("%s.type is required", prefix))
		}

		if wi.Status != "" && !validWorkStatuses[wi.Status] {
			errs = append(errs, fmt.Errorf("%s.status: invalid value %q", prefix, wi.Status))
		}
		if wi.DurationMode != "" && !validDurationModes[wi.DurationMode] {
			errs = append(errs, fmt.Errorf("%s.duration_mode: invalid value %q", prefix, wi.DurationMode))
		}

		if wi.SessionPolicy != nil {
			errs = append(errs, validateSessionPolicy(prefix+".session_policy", wi.SessionPolicy)...)
		}

		errs = append(errs, validateOptionalDate(prefix+".due_date", wi.DueDate)...)
		errs = append(errs, validateOptionalDate(prefix+".not_before", wi.NotBefore)...)
	}

	return errs
}

func validateDependencies(deps []DependencyImport, wiRefs map[string]bool) []error {
	var errs []error

	for i, d := range deps {
		prefix := fmt.Sprintf("dependencies[%d]", i)

		if d.PredecessorRef == "" {
			errs = append(errs, fmt.Errorf("%s.predecessor_ref is required", prefix))
		} else if !wiRefs[d.PredecessorRef] {
			errs = append(errs, fmt.Errorf("%s.predecessor_ref: ref %q not found in work_items", prefix, d.PredecessorRef))
		}

		if d.SuccessorRef == "" {
			errs = append(errs, fmt.Errorf("%s.successor_ref is required", prefix))
		} else if !wiRefs[d.SuccessorRef] {
			errs = append(errs, fmt.Errorf("%s.successor_ref: ref %q not found in work_items", prefix, d.SuccessorRef))
		}

		if d.PredecessorRef != "" && d.SuccessorRef != "" && d.PredecessorRef == d.SuccessorRef {
			errs = append(errs, fmt.Errorf("%s: self-dependency (predecessor_ref == successor_ref == %q)", prefix, d.PredecessorRef))
		}
	}

	// Check for circular dependencies
	if len(deps) > 1 {
		errs = append(errs, detectCycles(deps)...)
	}

	return errs
}

func detectCycles(deps []DependencyImport) []error {
	// Build adjacency list
	graph := make(map[string][]string)
	nodes := make(map[string]bool)
	for _, d := range deps {
		if d.PredecessorRef != "" && d.SuccessorRef != "" && d.PredecessorRef != d.SuccessorRef {
			graph[d.PredecessorRef] = append(graph[d.PredecessorRef], d.SuccessorRef)
			nodes[d.PredecessorRef] = true
			nodes[d.SuccessorRef] = true
		}
	}

	// DFS cycle detection
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully processed
	)

	color := make(map[string]int)
	var errs []error

	var visit func(node string) bool
	visit = func(node string) bool {
		color[node] = gray
		for _, neighbor := range graph[node] {
			if color[neighbor] == gray {
				errs = append(errs, fmt.Errorf("circular dependency detected involving %q and %q", node, neighbor))
				return true
			}
			if color[neighbor] == white {
				if visit(neighbor) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}

	for node := range nodes {
		if color[node] == white {
			visit(node)
		}
	}

	return errs
}

func validateOptionalDate(field string, dateStr *string) []error {
	if dateStr == nil || *dateStr == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *dateStr); err != nil {
		return []error{fmt.Errorf("%s: invalid date format %q (expected YYYY-MM-DD)", field, *dateStr)}
	}
	return nil
}

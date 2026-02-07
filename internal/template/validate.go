package template

import "fmt"

// ValidateSchema checks a TemplateSchema for structural errors.
// Returns a slice of errors (empty if valid).
func ValidateSchema(schema *TemplateSchema) []error {
	var errs []error

	if schema.ID == "" {
		errs = append(errs, fmt.Errorf("template id is required"))
	}
	if schema.Name == "" {
		errs = append(errs, fmt.Errorf("template name is required"))
	}
	if schema.Domain == "" {
		errs = append(errs, fmt.Errorf("template domain is required"))
	}
	if len(schema.Nodes) == 0 {
		errs = append(errs, fmt.Errorf("at least one node is required"))
	}
	if len(schema.WorkItems) == 0 {
		errs = append(errs, fmt.Errorf("at least one work item is required"))
	}

	// Check node configs.
	nodeIDs := map[string]bool{}
	for i, n := range schema.Nodes {
		if n.ID == "" {
			errs = append(errs, fmt.Errorf("node[%d]: id is required", i))
		}
		if n.Title == "" {
			errs = append(errs, fmt.Errorf("node[%d]: title is required", i))
		}
		if n.Kind == "" {
			errs = append(errs, fmt.Errorf("node[%d]: kind is required", i))
		}
		if nodeIDs[n.ID] {
			errs = append(errs, fmt.Errorf("node[%d]: duplicate id %q", i, n.ID))
		}
		nodeIDs[n.ID] = true
	}

	// Check work item configs.
	for i, w := range schema.WorkItems {
		if w.ID == "" {
			errs = append(errs, fmt.Errorf("work_item[%d]: id is required", i))
		}
		if w.NodeID == "" {
			errs = append(errs, fmt.Errorf("work_item[%d]: node_id is required", i))
		}
		if w.Title == "" {
			errs = append(errs, fmt.Errorf("work_item[%d]: title is required", i))
		}
		if w.Type == "" {
			errs = append(errs, fmt.Errorf("work_item[%d]: type is required", i))
		}
	}

	// Check dependency configs reference known IDs.
	allIDs := map[string]bool{}
	for k := range nodeIDs {
		allIDs[k] = true
	}
	for _, w := range schema.WorkItems {
		allIDs[w.ID] = true
	}
	for i, d := range schema.Dependencies {
		if d.Predecessor == "" {
			errs = append(errs, fmt.Errorf("dependency[%d]: predecessor is required", i))
		}
		if d.Successor == "" {
			errs = append(errs, fmt.Errorf("dependency[%d]: successor is required", i))
		}
	}

	return errs
}

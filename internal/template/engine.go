package template

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/google/uuid"
)

// GeneratedProject is the output of template execution.
type GeneratedProject struct {
	Project      *domain.Project
	Nodes        []*domain.PlanNode
	WorkItems    []*domain.WorkItem
	Dependencies []domain.Dependency
}

// LoadSchema reads and parses a template JSON file.
func LoadSchema(path string) (*TemplateSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema TemplateSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	return &schema, nil
}

// Execute generates all domain objects from a template schema.
func Execute(schema *TemplateSchema, projectName, startDate string, dueDate *string, userVars map[string]string) (*GeneratedProject, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}

	var targetDate *time.Time
	if dueDate != nil {
		t, err := time.Parse("2006-01-02", *dueDate)
		if err != nil {
			return nil, fmt.Errorf("invalid due date: %w", err)
		}
		targetDate = &t
	}

	// Resolve variables: apply defaults, then user overrides
	vars, err := resolveVariables(schema.Variables, userVars)
	if err != nil {
		return nil, fmt.Errorf("resolving variables: %w", err)
	}

	now := time.Now().UTC()

	// Create project
	project := &domain.Project{
		ID:         uuid.New().String(),
		Name:       projectName,
		Domain:     schema.Domain,
		StartDate:  start,
		TargetDate: targetDate,
		Status:     domain.ProjectActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Template ID -> real UUID mapping
	idMap := make(map[string]string)

	// Generate nodes
	var nodes []*domain.PlanNode
	for _, nc := range schema.Nodes {
		repeats, err := ParseRepeats(nc.Repeat)
		if err != nil {
			return nil, fmt.Errorf("parsing repeats for node '%s': %w", nc.ID, err)
		}

		err = iterateRepeats(repeats, vars, func(loopVars map[string]int) error {
			// Expand template ID
			expandedID, err := ExpandTemplate(nc.ID, loopVars)
			if err != nil {
				return fmt.Errorf("expanding node ID '%s': %w", nc.ID, err)
			}

			realID := uuid.New().String()
			idMap[expandedID] = realID

			// Expand title
			title, err := ExpandTemplate(nc.Title, loopVars)
			if err != nil {
				return fmt.Errorf("expanding title '%s': %w", nc.Title, err)
			}

			// Resolve parent
			var parentID *string
			if nc.ParentID != nil && *nc.ParentID != "" {
				expandedParent, err := ExpandTemplate(*nc.ParentID, loopVars)
				if err != nil {
					return fmt.Errorf("expanding parent_id '%s': %w", *nc.ParentID, err)
				}
				if realParent, ok := idMap[expandedParent]; ok {
					parentID = &realParent
				}
			}

			// Order
			var orderIndex int
			if nc.Order != "" {
				orderIndex, err = evalIntExpr(nc.Order, loopVars)
				if err != nil {
					// Fall back to 0
					orderIndex = 0
				}
			}

			// Constraints
			var notBefore, notAfter, nodeDueDate *time.Time
			if nc.Constraints != nil {
				if nc.Constraints.NotBeforeOffsetDays != "" {
					days, evalErr := evalIntExpr(nc.Constraints.NotBeforeOffsetDays, loopVars)
					if evalErr == nil {
						t := start.AddDate(0, 0, days)
						notBefore = &t
					}
				}
				if nc.Constraints.NotAfterOffsetDays != "" {
					days, evalErr := evalIntExpr(nc.Constraints.NotAfterOffsetDays, loopVars)
					if evalErr == nil {
						t := start.AddDate(0, 0, days)
						notAfter = &t
					}
				}
				if nc.Constraints.DueDateOffsetDays != "" {
					days, evalErr := evalIntExpr(nc.Constraints.DueDateOffsetDays, loopVars)
					if evalErr == nil {
						t := start.AddDate(0, 0, days)
						nodeDueDate = &t
					}
				}
			}

			// Budgets
			var plannedMinBudget *int
			if nc.Budgets != nil {
				plannedMinBudget = nc.Budgets.PlannedMinBudget
			}

			node := &domain.PlanNode{
				ID:               realID,
				ProjectID:        project.ID,
				ParentID:         parentID,
				Title:            title,
				Kind:             domain.NodeKind(nc.Kind),
				OrderIndex:       orderIndex,
				DueDate:          nodeDueDate,
				NotBefore:        notBefore,
				NotAfter:         notAfter,
				PlannedMinBudget: plannedMinBudget,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			nodes = append(nodes, node)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Generate work items
	var workItems []*domain.WorkItem
	for _, wc := range schema.WorkItems {
		repeats, err := ParseRepeats(wc.Repeat)
		if err != nil {
			return nil, fmt.Errorf("parsing repeats for work item '%s': %w", wc.ID, err)
		}

		err = iterateRepeats(repeats, vars, func(loopVars map[string]int) error {
			expandedID, err := ExpandTemplate(wc.ID, loopVars)
			if err != nil {
				return fmt.Errorf("expanding work item ID '%s': %w", wc.ID, err)
			}

			realID := uuid.New().String()
			idMap[expandedID] = realID

			title, err := ExpandTemplate(wc.Title, loopVars)
			if err != nil {
				return fmt.Errorf("expanding title '%s': %w", wc.Title, err)
			}

			// Resolve node_id
			expandedNodeID, err := ExpandTemplate(wc.NodeID, loopVars)
			if err != nil {
				return fmt.Errorf("expanding node_id '%s': %w", wc.NodeID, err)
			}
			realNodeID, ok := idMap[expandedNodeID]
			if !ok {
				return fmt.Errorf("node '%s' not found (expanded from '%s')", expandedNodeID, wc.NodeID)
			}

			// Apply defaults, then override with work item config
			durationMode := coalesceStr(wc.DurationMode, defaultDurationMode(schema.Defaults), "estimate")
			minSession := intFromPtrWithDefault(15, sessionPolicyField(wc.SessionPolicy, "min"), sessionPolicyField(defaultSessionPolicy(schema.Defaults), "min"))
			maxSession := intFromPtrWithDefault(60, sessionPolicyField(wc.SessionPolicy, "max"), sessionPolicyField(defaultSessionPolicy(schema.Defaults), "max"))
			defSession := intFromPtrWithDefault(30, sessionPolicyField(wc.SessionPolicy, "default"), sessionPolicyField(defaultSessionPolicy(schema.Defaults), "default"))
			splittable := boolFromPtrWithDefault(true, sessionPolicyBool(wc.SessionPolicy), sessionPolicyBool(defaultSessionPolicy(schema.Defaults)))
			plannedMin := intFromPtrWithDefault(0, wc.PlannedMin)
			estimateConf := float64FromPtrWithDefault(0.5, wc.EstimateConf)

			var unitsKind string
			var unitsTotal int
			if wc.Units != nil {
				unitsKind = wc.Units.Kind
				unitsTotal = wc.Units.Total
			}

			// Constraints
			var wiDueDate, wiNotBefore *time.Time
			if wc.Constraints != nil {
				if wc.Constraints.DueDateOffsetDays != "" {
					days, evalErr := evalIntExpr(wc.Constraints.DueDateOffsetDays, loopVars)
					if evalErr == nil {
						t := start.AddDate(0, 0, days)
						wiDueDate = &t
					}
				}
				if wc.Constraints.NotBeforeOffsetDays != "" {
					days, evalErr := evalIntExpr(wc.Constraints.NotBeforeOffsetDays, loopVars)
					if evalErr == nil {
						t := start.AddDate(0, 0, days)
						wiNotBefore = &t
					}
				}
			}

			wi := &domain.WorkItem{
				ID:                 realID,
				NodeID:             realNodeID,
				Title:              title,
				Type:               wc.Type,
				Status:             domain.WorkItemTodo,
				DurationMode:       domain.DurationMode(durationMode),
				PlannedMin:         plannedMin,
				DurationSource:     domain.SourceTemplate,
				EstimateConfidence: estimateConf,
				MinSessionMin:      minSession,
				MaxSessionMin:      maxSession,
				DefaultSessionMin:  defSession,
				Splittable:         splittable,
				UnitsKind:          unitsKind,
				UnitsTotal:         unitsTotal,
				DueDate:            wiDueDate,
				NotBefore:          wiNotBefore,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			workItems = append(workItems, wi)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Generate dependencies
	var deps []domain.Dependency
	for _, dc := range schema.Dependencies {
		// Dependencies may also have repeat patterns embedded in their IDs.
		// We expand each using the same variable space.
		// For non-repeated deps, we just expand once with base vars.
		predIDs := expandDependencyID(dc.Predecessor, idMap)
		succIDs := expandDependencyID(dc.Successor, idMap)

		// Match by expanded pattern: for each resolved predecessor, pair with its successor
		for tmplID, realID := range predIDs {
			// Find the matching successor with the same loop values
			if succReal, ok := succIDs[tmplID]; ok {
				deps = append(deps, domain.Dependency{
					PredecessorWorkItemID: realID,
					SuccessorWorkItemID:   succReal,
				})
			}
		}
	}

	return &GeneratedProject{
		Project:      project,
		Nodes:        nodes,
		WorkItems:    workItems,
		Dependencies: deps,
	}, nil
}

// resolveVariables builds the resolved variable map from defaults + user overrides.
func resolveVariables(defs []VariableConfig, userVars map[string]string) (map[string]int, error) {
	vars := make(map[string]int)

	for _, v := range defs {
		// Apply default
		if v.Default != nil {
			var def int
			if err := json.Unmarshal(v.Default, &def); err == nil {
				vars[v.Key] = def
			}
		}

		// Apply user override
		if userVars != nil {
			if val, ok := userVars[v.Key]; ok {
				intVal, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("variable '%s': expected integer, got '%s'", v.Key, val)
				}
				if v.Min != nil && intVal < *v.Min {
					return nil, fmt.Errorf("variable '%s': value %d below minimum %d", v.Key, intVal, *v.Min)
				}
				if v.Max != nil && intVal > *v.Max {
					return nil, fmt.Errorf("variable '%s': value %d above maximum %d", v.Key, intVal, *v.Max)
				}
				vars[v.Key] = intVal
			}
		}

		// Check required
		if v.Required {
			if _, ok := vars[v.Key]; !ok {
				return nil, fmt.Errorf("required variable '%s' not provided", v.Key)
			}
		}
	}

	return vars, nil
}

// iterateRepeats runs a callback for each combination of repeat variables.
// If no repeats, runs once with just the base vars.
func iterateRepeats(repeats []RepeatConfig, baseVars map[string]int, fn func(vars map[string]int) error) error {
	if len(repeats) == 0 {
		// No repeat - run once with base vars
		return fn(baseVars)
	}

	return iterateRepeatLevel(repeats, 0, baseVars, fn)
}

func iterateRepeatLevel(repeats []RepeatConfig, level int, vars map[string]int, fn func(vars map[string]int) error) error {
	if level >= len(repeats) {
		return fn(vars)
	}

	r := repeats[level]
	from := r.From
	to := 0

	if r.To != nil {
		to = *r.To
	} else if r.ToVar != "" {
		if v, ok := vars[r.ToVar]; ok {
			to = v
		} else {
			return fmt.Errorf("variable '%s' not defined for repeat bound", r.ToVar)
		}
	} else {
		return fmt.Errorf("repeat for '%s' has no 'to' or 'to_var'", r.Var)
	}

	for i := from; i <= to; i++ {
		loopVars := copyVars(vars)
		loopVars[r.Var] = i
		if err := iterateRepeatLevel(repeats, level+1, loopVars, fn); err != nil {
			return err
		}
	}
	return nil
}

func copyVars(vars map[string]int) map[string]int {
	cp := make(map[string]int, len(vars))
	for k, v := range vars {
		cp[k] = v
	}
	return cp
}

func evalIntExpr(expr string, vars map[string]int) (int, error) {
	// If it looks like a plain integer, parse directly
	if n, err := strconv.Atoi(expr); err == nil {
		return n, nil
	}
	// If the expression contains template braces, expand them first
	if containsBrace(expr) {
		expanded, err := ExpandTemplate(expr, vars)
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(expanded)
	}
	// Otherwise evaluate as a raw expression
	return EvalExpr(expr, vars)
}

// expandDependencyID finds all entries in idMap that match a template pattern.
// For simple IDs (no braces), returns a single match.
// For patterns like "w{i}_draft", finds all matching expanded IDs.
func expandDependencyID(pattern string, idMap map[string]string) map[string]string {
	matches := make(map[string]string)

	// If pattern has no braces, it's a literal
	if !containsBrace(pattern) {
		if realID, ok := idMap[pattern]; ok {
			matches[pattern] = realID
		}
		return matches
	}

	// For patterns with variables, find all idMap entries that could match.
	// We use a simplified approach: iterate all idMap keys and check if
	// they match the pattern structurally.
	prefix, suffix := splitPatternAroundFirstBrace(pattern)
	for key, realID := range idMap {
		if matchesPattern(key, prefix, suffix) {
			matches[key] = realID
		}
	}
	return matches
}

func containsBrace(s string) bool {
	for _, c := range s {
		if c == '{' {
			return true
		}
	}
	return false
}

func splitPatternAroundFirstBrace(pattern string) (string, string) {
	for i, c := range pattern {
		if c == '{' {
			// Find closing brace
			for j := i + 1; j < len(pattern); j++ {
				if pattern[j] == '}' {
					return pattern[:i], pattern[j+1:]
				}
			}
		}
	}
	return pattern, ""
}

func matchesPattern(key, prefix, suffix string) bool {
	if len(key) < len(prefix)+len(suffix) {
		return false
	}
	if prefix != "" && !hasPrefix(key, prefix) {
		return false
	}
	if suffix != "" && !hasSuffix(key, suffix) {
		return false
	}
	return true
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Helper functions for coalescing defaults.
// These take pointer values from config and return concrete types matching domain structs.

func coalesceStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// intFromPtrWithDefault returns the first non-nil *int value, or the fallback.
func intFromPtrWithDefault(fallback int, ptrs ...*int) int {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

// boolFromPtrWithDefault returns the first non-nil *bool value, or the fallback.
func boolFromPtrWithDefault(fallback bool, ptrs ...*bool) bool {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

// float64FromPtrWithDefault returns the first non-nil *float64 value, or the fallback.
func float64FromPtrWithDefault(fallback float64, ptrs ...*float64) float64 {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

func defaultDurationMode(d *DefaultsConfig) string {
	if d != nil {
		return d.DurationMode
	}
	return ""
}

func defaultSessionPolicy(d *DefaultsConfig) *SessionPolicyConfig {
	if d != nil {
		return d.SessionPolicy
	}
	return nil
}

func sessionPolicyField(sp *SessionPolicyConfig, field string) *int {
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

func sessionPolicyBool(sp *SessionPolicyConfig) *bool {
	if sp == nil {
		return nil
	}
	return sp.Splittable
}

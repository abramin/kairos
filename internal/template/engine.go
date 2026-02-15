package template

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/generation"
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
	start, err := generation.ParseRequiredDate(startDate, "start date")
	if err != nil {
		return nil, err
	}

	targetDate, err := generation.ParseOptionalDate(dueDate, "due date")
	if err != nil {
		return nil, err
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
	defaults := generation.WorkItemDefaultsInput{}
	if schema.Defaults != nil {
		defaults.DurationMode = schema.Defaults.DurationMode
		defaults.SessionPolicy = schema.Defaults.SessionPolicy
	}
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
			resolved := generation.ResolveWorkItemDefaults(
				generation.WorkItemDefaultsInput{
					DurationMode:       wc.DurationMode,
					SessionPolicy:      wc.SessionPolicy,
					PlannedMin:         wc.PlannedMin,
					EstimateConfidence: wc.EstimateConf,
				},
				defaults,
			)

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
				DurationMode:       domain.DurationMode(resolved.DurationMode),
				PlannedMin:         resolved.PlannedMin,
				DurationSource:     domain.SourceTemplate,
				EstimateConfidence: resolved.EstimateConfidence,
				MinSessionMin:      resolved.MinSessionMin,
				MaxSessionMin:      resolved.MaxSessionMin,
				DefaultSessionMin:  resolved.DefaultSessionMin,
				Splittable:         resolved.Splittable,
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

	// Assign deterministic sequence IDs in tree order.
	generation.AssignSequentialIDs(nodes, workItems)

	// Generate dependencies.
	// If omitted, infer a default linear chain by node order, then work item order.
	var deps []domain.Dependency
	if len(schema.Dependencies) > 0 {
		for _, dc := range schema.Dependencies {
			deps = append(deps, generateDependencyLinks(dc.Predecessor, dc.Successor, idMap, vars)...)
		}
	} else {
		deps = inferTemplateDeps(nodes, workItems)
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

func containsBrace(s string) bool {
	for _, c := range s {
		if c == '{' {
			return true
		}
	}
	return false
}

func generateDependencyLinks(predecessorPattern, successorPattern string, idMap map[string]string, vars map[string]int) []domain.Dependency {
	loopVars := dependencyLoopVars(predecessorPattern, successorPattern, vars)
	var deps []domain.Dependency
	seen := make(map[string]bool)

	err := iterateDependencyVars(loopVars, vars, func(loop map[string]int) error {
		expandedPred, err := expandDependencyPattern(predecessorPattern, loop)
		if err != nil {
			return nil
		}
		expandedSucc, err := expandDependencyPattern(successorPattern, loop)
		if err != nil {
			return nil
		}

		predID, okPred := idMap[expandedPred]
		succID, okSucc := idMap[expandedSucc]
		if !okPred || !okSucc {
			return nil
		}

		key := predID + "->" + succID
		if seen[key] {
			return nil
		}
		seen[key] = true
		deps = append(deps, domain.Dependency{
			PredecessorWorkItemID: predID,
			SuccessorWorkItemID:   succID,
		})
		return nil
	})
	if err != nil {
		return deps
	}

	return deps
}

func expandDependencyPattern(pattern string, loopVars map[string]int) (string, error) {
	if !containsBrace(pattern) {
		return pattern, nil
	}
	return ExpandTemplate(pattern, loopVars)
}

func dependencyLoopVars(predecessorPattern, successorPattern string, vars map[string]int) []string {
	set := make(map[string]bool)
	for _, name := range extractPatternVars(predecessorPattern) {
		if max, ok := vars[name]; ok && max > 0 {
			set[name] = true
		}
	}
	for _, name := range extractPatternVars(successorPattern) {
		if max, ok := vars[name]; ok && max > 0 {
			set[name] = true
		}
	}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func iterateDependencyVars(varNames []string, vars map[string]int, fn func(loop map[string]int) error) error {
	if len(varNames) == 0 {
		return fn(copyVars(vars))
	}

	var rec func(idx int, current map[string]int) error
	rec = func(idx int, current map[string]int) error {
		if idx >= len(varNames) {
			return fn(current)
		}
		name := varNames[idx]
		maxVal, ok := vars[name]
		if !ok || maxVal <= 0 {
			return nil
		}
		for i := 1; i <= maxVal; i++ {
			next := copyVars(current)
			next[name] = i
			if err := rec(idx+1, next); err != nil {
				return err
			}
		}
		return nil
	}

	return rec(0, copyVars(vars))
}

func extractPatternVars(pattern string) []string {
	set := make(map[string]bool)
	for _, expr := range extractBraceExpressions(pattern) {
		for _, token := range tokenizeExprIdentifiers(expr) {
			set[token] = true
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names
}

func extractBraceExpressions(s string) []string {
	var exprs []string
	var b strings.Builder
	inExpr := false

	for _, r := range s {
		if r == '{' {
			inExpr = true
			b.Reset()
			continue
		}
		if r == '}' {
			if inExpr {
				exprs = append(exprs, b.String())
			}
			inExpr = false
			continue
		}
		if inExpr {
			b.WriteRune(r)
		}
	}

	return exprs
}

func tokenizeExprIdentifiers(expr string) []string {
	var names []string
	seen := make(map[string]bool)
	for i := 0; i < len(expr); {
		r := rune(expr[i])
		if unicode.IsLetter(r) || r == '_' {
			start := i
			i++
			for i < len(expr) {
				rr := rune(expr[i])
				if unicode.IsLetter(rr) || unicode.IsDigit(rr) || rr == '_' {
					i++
					continue
				}
				break
			}
			name := expr[start:i]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
			continue
		}
		i++
	}
	return names
}

// inferTemplateDeps builds DependencyCandidate entries from domain objects
// and delegates to the shared InferLinearDependencies algorithm.
func inferTemplateDeps(nodes []*domain.PlanNode, workItems []*domain.WorkItem) []domain.Dependency {
	nodePos := make(map[string]int, len(nodes))
	nodeOrder := make(map[string]int, len(nodes))
	for i, n := range nodes {
		nodePos[n.ID] = i
		nodeOrder[n.ID] = n.OrderIndex
	}

	candidates := make([]generation.DependencyCandidate, 0, len(workItems))
	for i, wi := range workItems {
		candidates = append(candidates, generation.DependencyCandidate{
			ID:        wi.ID,
			NodeOrder: nodeOrder[wi.NodeID],
			NodePos:   nodePos[wi.NodeID],
			ItemPos:   i,
		})
	}

	return generation.InferLinearDependencies(candidates)
}

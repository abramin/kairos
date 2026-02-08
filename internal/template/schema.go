package template

import "encoding/json"

// TemplateSchema is the top-level JSON template structure.
type TemplateSchema struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Description  string             `json:"description,omitempty"`
	Domain       string             `json:"domain"`
	Defaults     *DefaultsConfig    `json:"defaults,omitempty"`
	Project      *ProjectConfig     `json:"project,omitempty"`
	Generation   *GenerationConfig  `json:"generation,omitempty"`
	Variables    []VariableConfig   `json:"variables,omitempty"`
	Nodes        []NodeConfig       `json:"nodes"`
	WorkItems    []WorkItemConfig   `json:"work_items"`
	Dependencies []DependencyConfig `json:"dependencies,omitempty"`
	Validation   *ValidationConfig  `json:"validation,omitempty"`
}

type DefaultsConfig struct {
	DurationMode  string               `json:"duration_mode,omitempty"`
	SessionPolicy *SessionPolicyConfig `json:"session_policy,omitempty"`
	BufferPct     *float64             `json:"buffer_pct,omitempty"`
}

type SessionPolicyConfig struct {
	MinSessionMin     *int  `json:"min_session_min,omitempty"`
	MaxSessionMin     *int  `json:"max_session_min,omitempty"`
	DefaultSessionMin *int  `json:"default_session_min,omitempty"`
	Splittable        *bool `json:"splittable,omitempty"`
}

type ProjectConfig struct {
	TargetDateMode string `json:"target_date_mode,omitempty"` // "optional" or "required"
	Status         string `json:"status,omitempty"`
}

type GenerationConfig struct {
	Mode   string `json:"mode"`   // "upfront"
	Anchor string `json:"anchor"` // "project_start_date"
}

type VariableConfig struct {
	Key      string          `json:"key"`
	Type     string          `json:"type"` // "int", "string"
	Required bool            `json:"required"`
	Default  json.RawMessage `json:"default,omitempty"`
	Min      *int            `json:"min,omitempty"`
	Max      *int            `json:"max,omitempty"`
}

// RepeatConfig can be a single repeat or nested.
// In JSON, "repeat" can be an object (single) or array of objects (nested).
type RepeatConfig struct {
	Var   string `json:"var"`
	From  int    `json:"from"`
	To    *int   `json:"to,omitempty"`     // explicit upper bound
	ToVar string `json:"to_var,omitempty"` // variable name for upper bound
}

type ConstraintsConfig struct {
	NotBeforeOffsetDays string `json:"not_before_offset_days,omitempty"` // expression, e.g. "{(i-1)*7}"
	NotAfterOffsetDays  string `json:"not_after_offset_days,omitempty"`  // expression
	DueDateOffsetDays   string `json:"due_date_offset_days,omitempty"`   // expression
}

type BudgetsConfig struct {
	PlannedMinBudget *int `json:"planned_min_budget,omitempty"`
}

type UnitsConfig struct {
	Kind  string `json:"kind"`
	Total int    `json:"total"`
}

type NodeConfig struct {
	ID          string             `json:"id"`
	Repeat      json.RawMessage    `json:"repeat,omitempty"` // can be object or array
	Title       string             `json:"title"`
	Kind        string             `json:"kind"`
	ParentID    *string            `json:"parent_id"`       // template ID reference, null for root
	Order       string             `json:"order,omitempty"` // expression
	Constraints *ConstraintsConfig `json:"constraints,omitempty"`
	Budgets     *BudgetsConfig     `json:"budgets,omitempty"`
}

type WorkItemConfig struct {
	ID            string               `json:"id"`
	Repeat        json.RawMessage      `json:"repeat,omitempty"`
	NodeID        string               `json:"node_id"` // template ID reference
	Title         string               `json:"title"`
	Type          string               `json:"type"`
	Status        string               `json:"status,omitempty"`
	DurationMode  string               `json:"duration_mode,omitempty"`
	PlannedMin    *int                 `json:"planned_min,omitempty"`
	SessionPolicy *SessionPolicyConfig `json:"session_policy,omitempty"`
	Units         *UnitsConfig         `json:"units,omitempty"`
	Constraints   *ConstraintsConfig   `json:"constraints,omitempty"`
	EstimateConf  *float64             `json:"estimate_confidence,omitempty"`
}

type DependencyConfig struct {
	Predecessor string `json:"predecessor"` // template ID pattern
	Successor   string `json:"successor"`   // template ID pattern
}

type ValidationConfig struct {
	RequireUniqueIDs           bool `json:"require_unique_ids"`
	RejectCircularDependencies bool `json:"reject_circular_dependencies"`
	EnforceSessionBounds       bool `json:"enforce_session_bounds"`
}

// ParseRepeats parses the repeat field which can be a single object or an array.
func ParseRepeats(raw json.RawMessage) ([]RepeatConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	// Try array first
	var arr []RepeatConfig
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}

	// Try single object
	var single RepeatConfig
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, err
	}
	return []RepeatConfig{single}, nil
}

package importer

import (
	"encoding/json"
	"fmt"
	"os"
)

// ImportSchema is the top-level JSON structure for project import.
type ImportSchema struct {
	Project      ProjectImport      `json:"project"`
	Defaults     *DefaultsImport    `json:"defaults,omitempty"`
	Nodes        []NodeImport       `json:"nodes"`
	WorkItems    []WorkItemImport   `json:"work_items"`
	Dependencies []DependencyImport `json:"dependencies,omitempty"`
}

// ProjectImport defines the project-level fields in the import file.
type ProjectImport struct {
	ShortID    string  `json:"short_id"`
	Name       string  `json:"name"`
	Domain     string  `json:"domain"`
	StartDate  string  `json:"start_date"`
	TargetDate *string `json:"target_date,omitempty"`
}

// DefaultsImport defines project-wide defaults that cascade to work items.
type DefaultsImport struct {
	DurationMode  string               `json:"duration_mode,omitempty"`
	SessionPolicy *SessionPolicyImport `json:"session_policy,omitempty"`
}

// SessionPolicyImport defines session bounds for work items.
type SessionPolicyImport struct {
	MinSessionMin     *int  `json:"min_session_min,omitempty"`
	MaxSessionMin     *int  `json:"max_session_min,omitempty"`
	DefaultSessionMin *int  `json:"default_session_min,omitempty"`
	Splittable        *bool `json:"splittable,omitempty"`
}

// NodeImport defines a plan node in the import file.
type NodeImport struct {
	Ref              string  `json:"ref"`
	ParentRef        *string `json:"parent_ref,omitempty"`
	Title            string  `json:"title"`
	Kind             string  `json:"kind"`
	Order            int     `json:"order"`
	DueDate          *string `json:"due_date,omitempty"`
	NotBefore        *string `json:"not_before,omitempty"`
	NotAfter         *string `json:"not_after,omitempty"`
	PlannedMinBudget *int    `json:"planned_min_budget,omitempty"`
}

// WorkItemImport defines a work item in the import file.
type WorkItemImport struct {
	Ref                string               `json:"ref"`
	NodeRef            string               `json:"node_ref"`
	Title              string               `json:"title"`
	Type               string               `json:"type"`
	Status             string               `json:"status,omitempty"`
	DurationMode       string               `json:"duration_mode,omitempty"`
	PlannedMin         *int                 `json:"planned_min,omitempty"`
	LoggedMin          *int                 `json:"logged_min,omitempty"`
	EstimateConfidence *float64             `json:"estimate_confidence,omitempty"`
	SessionPolicy      *SessionPolicyImport `json:"session_policy,omitempty"`
	Units              *UnitsImport         `json:"units,omitempty"`
	DueDate            *string              `json:"due_date,omitempty"`
	NotBefore          *string              `json:"not_before,omitempty"`
}

// UnitsImport defines unit-based progress tracking for a work item.
type UnitsImport struct {
	Kind  string `json:"kind"`
	Total int    `json:"total"`
}

// DependencyImport defines a dependency between two work items.
type DependencyImport struct {
	PredecessorRef string `json:"predecessor_ref"`
	SuccessorRef   string `json:"successor_ref"`
}

// LoadImportSchema reads and parses a project import JSON file.
func LoadImportSchema(path string) (*ImportSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema ImportSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parsing import file: %w", err)
	}
	return &schema, nil
}

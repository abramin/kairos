package domain

import "time"

type PlanNode struct {
	ID               string
	ProjectID        string
	Seq              int // project-scoped sequential ID (shared with work items)
	ParentID         *string
	Title            string
	Kind             NodeKind
	OrderIndex       int
	DueDate          *time.Time
	NotBefore        *time.Time
	NotAfter         *time.Time
	PlannedMinBudget *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

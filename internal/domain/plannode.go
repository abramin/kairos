package domain

import "time"

type PlanNode struct {
	ID               string
	ProjectID        string
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

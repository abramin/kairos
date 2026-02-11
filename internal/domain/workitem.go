package domain

import "time"

type WorkItem struct {
	ID          string
	NodeID      string
	Seq         int // project-scoped sequential ID (shared with plan nodes)
	Title       string
	Description string
	Type        string
	Status      WorkItemStatus
	ArchivedAt  *time.Time
	CompletedAt *time.Time

	// Duration
	DurationMode       DurationMode
	PlannedMin         int
	LoggedMin          int
	DurationSource     DurationSource
	EstimateConfidence float64

	// Session policy
	MinSessionMin     int
	MaxSessionMin     int
	DefaultSessionMin int
	Splittable        bool

	// Scope progress
	UnitsKind  string
	UnitsTotal int
	UnitsDone  int

	// Constraints
	DueDate   *time.Time
	NotBefore *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

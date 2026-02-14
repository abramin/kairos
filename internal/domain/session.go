package domain

import "time"

type WorkSessionLog struct {
	ID             string
	WorkItemID     string
	StartedAt      time.Time
	Minutes        int
	UnitsDoneDelta int
	Note           string
	CreatedAt      time.Time
}

// SessionSummaryByType aggregates session minutes per work item, including type info.
type SessionSummaryByType struct {
	WorkItemTitle string
	WorkItemType  string
	TotalMinutes  int
}

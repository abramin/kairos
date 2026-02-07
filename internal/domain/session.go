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

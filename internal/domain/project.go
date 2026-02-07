package domain

import "time"

type Project struct {
	ID         string
	ShortID    string
	Name       string
	Domain     string
	StartDate  time.Time
	TargetDate *time.Time
	Status     ProjectStatus
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

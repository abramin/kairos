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

// DisplayID returns the best short identifier for display.
// It prefers shortID; if empty it truncates fullID to 8 characters.
func DisplayID(shortID, fullID string) string {
	if shortID != "" {
		return shortID
	}
	if len(fullID) >= 8 {
		return fullID[:8]
	}
	return fullID
}

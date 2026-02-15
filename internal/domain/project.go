package domain

import (
	"fmt"
	"regexp"
	"time"
)

var shortIDPattern = regexp.MustCompile(`^[A-Z]{3,6}[0-9]{2,4}$`)

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

// ValidateShortID checks that ShortID is non-empty and matches the required
// format: 3-6 uppercase letters followed by 2-4 digits (e.g. PHI01, MATH0234).
func (p *Project) ValidateShortID() error {
	if p.ShortID == "" {
		return fmt.Errorf("short ID is required (use --id flag)")
	}
	if !shortIDPattern.MatchString(p.ShortID) {
		return fmt.Errorf("short ID %q must be 3-6 uppercase letters followed by 2-4 digits (e.g. PHI01)", p.ShortID)
	}
	return nil
}

// DisplayID returns the best short identifier for display.
// It prefers ShortID; if empty it truncates ID to 8 characters.
func (p *Project) DisplayID() string {
	if p.ShortID != "" {
		return p.ShortID
	}
	if len(p.ID) >= 8 {
		return p.ID[:8]
	}
	return p.ID
}

package domain

import "time"

// Habit is a recurring open-ended activity (e.g. "French novel reading")
// that has no completion state — only a cadence and a target duration per session.
type Habit struct {
	ID            string
	Title         string
	CadenceDays   int        // target interval: 1=daily, 7=weekly, etc.
	TargetMin     int        // desired session duration in minutes
	MinSessionMin int        // minimum viable session length
	MaxSessionMin int        // maximum session length
	ArchivedAt    *time.Time // soft-delete
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsActive returns true when the habit has not been archived.
func (h *Habit) IsActive() bool {
	return h.ArchivedAt == nil
}

// Archive soft-deletes the habit. Idempotent.
func (h *Habit) Archive(now time.Time) {
	if h.ArchivedAt != nil {
		return
	}
	h.ArchivedAt = &now
	h.UpdatedAt = now
}

// HabitLog records a single habit session.
type HabitLog struct {
	ID          string
	HabitID     string
	PerformedAt time.Time
	Minutes     int
	Note        string
	CreatedAt   time.Time
}

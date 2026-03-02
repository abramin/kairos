package domain

import "time"

// Task is a standalone global to-do item, not tied to any project/node/work-item.
type Task struct {
	ID          string
	Title       string
	Description string
	OrderIndex  int
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsActive returns true when the task has not been archived.
func (t *Task) IsActive() bool {
	return t.ArchivedAt == nil
}

// Archive soft-deletes the task. Idempotent.
func (t *Task) Archive(now time.Time) {
	if t.ArchivedAt != nil {
		return
	}
	t.ArchivedAt = &now
	t.UpdatedAt = now
}

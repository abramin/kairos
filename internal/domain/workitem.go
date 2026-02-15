package domain

import (
	"fmt"
	"time"
)

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

// IsTerminal returns true for done, skipped, or archived statuses.
func (w *WorkItem) IsTerminal() bool {
	return w.Status == WorkItemDone || w.Status == WorkItemSkipped || w.Status == WorkItemArchived
}

// MarkDone transitions the work item to done and sets CompletedAt.
// Idempotent if already done. Returns error if archived.
func (w *WorkItem) MarkDone(now time.Time) error {
	if w.Status == WorkItemDone {
		return nil
	}
	if w.Status == WorkItemArchived {
		return fmt.Errorf("cannot mark done: work item in %s status", w.Status)
	}
	w.Status = WorkItemDone
	w.CompletedAt = &now
	w.UpdatedAt = now
	return nil
}

// MarkInProgress transitions the work item to in_progress.
// Idempotent if already in_progress. Returns error if archived or done.
func (w *WorkItem) MarkInProgress(now time.Time) error {
	if w.Status == WorkItemInProgress {
		return nil
	}
	if w.Status == WorkItemArchived || w.Status == WorkItemDone {
		return fmt.Errorf("cannot mark in-progress: work item in %s status", w.Status)
	}
	w.Status = WorkItemInProgress
	w.UpdatedAt = now
	return nil
}

// Reopen transitions a done work item back to todo and clears CompletedAt.
// Returns error if not currently done.
func (w *WorkItem) Reopen(now time.Time) error {
	if w.Status != WorkItemDone {
		return fmt.Errorf("cannot reopen: work item in %s status", w.Status)
	}
	w.Status = WorkItemTodo
	w.CompletedAt = nil
	w.UpdatedAt = now
	return nil
}

// ApplySession accumulates logged minutes and units from a session.
// Auto-transitions todo → in_progress on first session.
// Does NOT handle re-estimation — caller is responsible for that.
func (w *WorkItem) ApplySession(minutes, unitsDelta int, now time.Time) error {
	if w.Status == WorkItemArchived {
		return fmt.Errorf("cannot log session: work item in %s status", w.Status)
	}
	w.LoggedMin += minutes
	w.UnitsDone += unitsDelta

	if w.Status == WorkItemTodo {
		w.Status = WorkItemInProgress
	}

	w.UpdatedAt = now
	return nil
}

// EligibleForReestimate returns true if this item qualifies for smooth
// re-estimation: has unit tracking, is in estimate mode, and is not terminal.
func (w *WorkItem) EligibleForReestimate() bool {
	return w.UnitsTotal > 0 && w.UnitsDone > 0 &&
		w.DurationMode == DurationEstimate &&
		!w.IsTerminal()
}

// ApplyReestimate updates PlannedMin if the new value differs.
// Returns true if the value changed.
func (w *WorkItem) ApplyReestimate(newPlannedMin int, now time.Time) bool {
	if newPlannedMin == w.PlannedMin {
		return false
	}
	w.PlannedMin = newPlannedMin
	w.UpdatedAt = now
	return true
}

// EffectiveLoggedMin returns LoggedMin, but for done/skipped items
// returns max(LoggedMin, PlannedMin) — completed work counts as at least planned.
func (w *WorkItem) EffectiveLoggedMin() int {
	if (w.Status == WorkItemDone || w.Status == WorkItemSkipped) && w.LoggedMin < w.PlannedMin {
		return w.PlannedMin
	}
	return w.LoggedMin
}

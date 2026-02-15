package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// LogSessionInput holds the parameters for logging a work session.
type LogSessionInput struct {
	ItemID     string
	Title      string
	Minutes    int
	UnitsDelta int
	Note       string
}

// execLogSession creates and persists a WorkSessionLog, updates shared state,
// and returns a formatted success message.
func execLogSession(ctx context.Context, app *App, state *SharedState, in LogSessionInput) (string, error) {

	s := &domain.WorkSessionLog{
		ID:             uuid.New().String(),
		WorkItemID:     in.ItemID,
		StartedAt:      time.Now(),
		Minutes:        in.Minutes,
		UnitsDoneDelta: in.UnitsDelta,
		Note:           in.Note,
		CreatedAt:      time.Now(),
	}
	logSession := app.logSessionUseCase()
	if logSession == nil {
		return "", fmt.Errorf("log-session use case is not configured")
	}
	if err := logSession.LogSession(ctx, s); err != nil {
		return "", err
	}

	state.ActiveItemID = in.ItemID
	state.LastDuration = in.Minutes

	msg := fmt.Sprintf("%s Logged %s to %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(formatter.FormatMinutes(in.Minutes)),
		formatter.Bold(in.Title))
	if in.UnitsDelta > 0 {
		msg += fmt.Sprintf(" (+%d units)", in.UnitsDelta)
	}
	return msg, nil
}

// execStartItem marks a work item as in-progress and updates shared state.
func execStartItem(ctx context.Context, app *App, state *SharedState,
	itemID, title string, seq int) (string, error) {

	if err := app.WorkItems.MarkInProgress(ctx, itemID); err != nil {
		return "", err
	}
	state.SetActiveItem(itemID, title, seq)
	return fmt.Sprintf("%s Started: %s",
		formatter.StyleGreen.Render("▶"),
		formatter.Bold(title)), nil
}

// execMarkDone marks a work item as done and clears context if it was active.
func execMarkDone(ctx context.Context, app *App, state *SharedState,
	itemID, title string) (string, error) {

	if err := app.WorkItems.MarkDone(ctx, itemID); err != nil {
		return "", err
	}
	if state.ActiveItemID == itemID {
		state.ClearItemContext()
	}
	return fmt.Sprintf("%s Done: %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(title)), nil
}

// wizardCompleteError returns a wizardCompleteMsg that displays a formatted error.
func wizardCompleteError(err error) tea.Msg {
	return wizardCompleteMsg{nextCmd: outputCmd(shellError(err))}
}

// wizardCompleteOutput returns a wizardCompleteMsg that displays a message string.
func wizardCompleteOutput(msg string) tea.Msg {
	return wizardCompleteMsg{nextCmd: outputCmd(msg)}
}

// wrapAsWizardComplete calls fn and wraps the result in a wizardCompleteMsg,
// formatting errors via shellError. Used by action menu handlers that execute
// a (string, error) action and then pop back to the previous view.
func wrapAsWizardComplete(fn func() (string, error)) tea.Msg {
	msg, err := fn()
	if err != nil {
		return wizardCompleteError(err)
	}
	return wizardCompleteOutput(msg)
}

// execConfirmDelete pushes a confirmation wizard and runs deleteFn if confirmed.
// Shared structure for deleting work items, nodes, and other entities.
func execConfirmDelete(state *SharedState, prompt, title string, deleteFn func(ctx context.Context) error) tea.Cmd {
	var confirmed bool
	form := wizardConfirm(prompt, &confirmed)
	return pushView(newWizardView(state, "Confirm Delete", form, func() tea.Cmd {
		if !confirmed {
			return func() tea.Msg { return wizardCompleteOutput(formatter.Dim("Cancelled.")) }
		}
		return func() tea.Msg {
			if err := deleteFn(context.Background()); err != nil {
				return wizardCompleteError(err)
			}
			return wizardCompleteOutput(fmt.Sprintf("%s Deleted: %s",
				formatter.StyleGreen.Render("✔"),
				formatter.Bold(title)))
		}
	}))
}

// execDeleteItem pushes a confirmation wizard and deletes the work item
// if confirmed. Used by both the action menu and task list views.
func execDeleteItem(state *SharedState, itemID, title string) tea.Cmd {
	return execConfirmDelete(state, fmt.Sprintf("Delete %q?", title), title, func(ctx context.Context) error {
		err := state.App.WorkItems.Delete(ctx, itemID)
		if err == nil && state.ActiveItemID == itemID {
			state.ClearItemContext()
		}
		return err
	})
}

package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func formErrorOutput(err error) tea.Msg {
	return cmdOutputMsg{output: shellError(err)}
}

func formSuccessOutput(msg string) tea.Msg {
	return cmdOutputMsg{output: msg}
}

// wizardErrorView returns a wizard that just shows an error and pops back.
func wizardErrorView(state *SharedState, title string, err error) View {
	form := huh.NewForm(
		huh.NewGroup(huh.NewNote().Title("Error").Description(err.Error())),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
	return newWizardView(state, title, form, func() tea.Cmd {
		return func() tea.Msg { return formErrorOutput(err) }
	})
}

// newLogFormView creates a wizard form for logging a session against a work item.
// It collects duration, units completed, and optional notes, then persists
// the session via SessionService.
func newLogFormView(state *SharedState, itemID, title string) View {
	defaultMin := 60
	if state.LastDuration > 0 {
		defaultMin = state.LastDuration
	}

	var duration string
	var unitsDone string
	var notes string

	duration = strconv.Itoa(defaultMin)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Duration (minutes)").
				Placeholder(strconv.Itoa(defaultMin)).
				Value(&duration).
				Validate(validatePositiveInt),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Units Completed (optional)").
				Placeholder("0").
				Value(&unitsDone).
				Validate(validateNonNegativeInt),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Notes (optional)").
				Value(&notes),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		return func() tea.Msg {
			minutes := parsePositiveInt(duration, defaultMin)
			unitsDelta := parsePositiveInt(unitsDone, 0)

			msg, err := execLogSession(context.Background(), state.App, state, LogSessionInput{
				ItemID: itemID, Title: title, Minutes: minutes, UnitsDelta: unitsDelta, Note: notes,
			})
			if err != nil {
				return formErrorOutput(err)
			}
			return formSuccessOutput(msg)
		}
	}

	return newWizardView(state, "Log Session", form, done)
}

// newAdjustLoggedView creates a wizard form for correcting the total logged
// minutes on a work item. Pre-populates with the current value and lets the
// user type a corrected total.
func newAdjustLoggedView(state *SharedState, itemID, title string) View {
	ctx := context.Background()
	item, err := state.App.WorkItems.GetByID(ctx, itemID)
	if err != nil {
		return wizardErrorView(state, "Adjust Logged Time", err)
	}

	oldLogged := item.LoggedMin
	newValue := strconv.Itoa(oldLogged)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Total Logged Minutes (currently %s)", formatter.FormatMinutes(oldLogged))).
				Placeholder(strconv.Itoa(oldLogged)).
				Value(&newValue).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("enter a value")
					}
					return validateNonNegativeInt(s)
				}),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		return func() tea.Msg {
			ctx := context.Background()
			minutes, _ := strconv.Atoi(newValue)

			current, err := state.App.WorkItems.GetByID(ctx, itemID)
			if err != nil {
				return formErrorOutput(err)
			}

			current.LoggedMin = minutes
			if err := state.App.WorkItems.Update(ctx, current); err != nil {
				return formErrorOutput(err)
			}

			return formSuccessOutput(fmt.Sprintf("%s Adjusted %s: %s %s %s",
				formatter.StyleGreen.Render("✔"),
				formatter.Bold(title),
				formatter.FormatMinutes(oldLogged),
				formatter.Dim("→"),
				formatter.Bold(formatter.FormatMinutes(minutes))))
		}
	}

	return newWizardView(state, "Adjust Logged Time", form, done)
}

// editWorkItemFields holds form-bound values for the edit work item wizard.
type editWorkItemFields struct {
	title      string
	desc       string
	plannedMin string
	itemType   string
	dueDate    string
	notBefore  string
	minSession string
	maxSession string
}

// applyEditWorkItem persists edited fields to the work item in the database.
func applyEditWorkItem(app *App, itemID string, f *editWorkItemFields) tea.Msg {
	ctx := context.Background()
	current, err := app.WorkItems.GetByID(ctx, itemID)
	if err != nil {
		return formErrorOutput(err)
	}

	current.Title = f.title
	current.Description = f.desc
	current.PlannedMin = parsePositiveInt(f.plannedMin, current.PlannedMin)
	current.Type = f.itemType

	if f.dueDate == "" {
		current.DueDate = nil
	} else if t, err := time.Parse("2006-01-02", f.dueDate); err == nil {
		current.DueDate = &t
	}

	if f.notBefore == "" {
		current.NotBefore = nil
	} else if t, err := time.Parse("2006-01-02", f.notBefore); err == nil {
		current.NotBefore = &t
	}

	if f.minSession == "" {
		current.MinSessionMin = 0
	} else if v, err := strconv.Atoi(f.minSession); err == nil {
		current.MinSessionMin = v
	}

	if f.maxSession == "" {
		current.MaxSessionMin = 0
	} else if v, err := strconv.Atoi(f.maxSession); err == nil {
		current.MaxSessionMin = v
	}

	if err := app.WorkItems.Update(ctx, current); err != nil {
		return formErrorOutput(err)
	}

	return formSuccessOutput(fmt.Sprintf("%s Updated: %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(f.title)))
}

// newEditWorkItemView creates a wizard form for editing a work item's fields.
// Pre-populates from current values. Empty date/optional fields clear the value.
func newEditWorkItemView(state *SharedState, itemID, title string) View {
	ctx := context.Background()
	item, err := state.App.WorkItems.GetByID(ctx, itemID)
	if err != nil {
		return wizardErrorView(state, "Edit Work Item", err)
	}

	f := &editWorkItemFields{
		title:      item.Title,
		desc:       item.Description,
		plannedMin: strconv.Itoa(item.PlannedMin),
		itemType:   item.Type,
	}
	if f.itemType == "" {
		f.itemType = "task"
	}
	if item.DueDate != nil {
		f.dueDate = item.DueDate.Format("2006-01-02")
	}
	if item.NotBefore != nil {
		f.notBefore = item.NotBefore.Format("2006-01-02")
	}
	if item.MinSessionMin > 0 {
		f.minSession = strconv.Itoa(item.MinSessionMin)
	}
	if item.MaxSessionMin > 0 {
		f.maxSession = strconv.Itoa(item.MaxSessionMin)
	}

	// Build type options, ensuring the current type is always present.
	typeOptions := []huh.Option[string]{
		huh.NewOption("Reading", "reading"),
		huh.NewOption("Zettel", "zettel"),
		huh.NewOption("Practice", "practice"),
		huh.NewOption("Review", "review"),
		huh.NewOption("Assignment", "assignment"),
		huh.NewOption("Task", "task"),
		huh.NewOption("Quiz", "quiz"),
		huh.NewOption("Study", "study"),
		huh.NewOption("Training", "training"),
		huh.NewOption("Activity", "activity"),
		huh.NewOption("Submission", "submission"),
	}
	if !domain.ValidWorkItemTypes[f.itemType] {
		typeOptions = append(typeOptions, huh.NewOption(f.itemType, f.itemType))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Value(&f.title).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description").
				Placeholder("optional").
				Value(&f.desc),
			huh.NewInput().
				Title(fmt.Sprintf("Planned Minutes (currently %s)", formatter.FormatMinutes(item.PlannedMin))).
				Placeholder(strconv.Itoa(item.PlannedMin)).
				Value(&f.plannedMin).
				Validate(validatePositiveInt),
			huh.NewSelect[string]().
				Title("Type").
				Options(typeOptions...).
				Value(&f.itemType),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Due Date (YYYY-MM-DD, blank to clear)").
				Placeholder("2025-06-30").
				Value(&f.dueDate).
				Validate(validateOptionalDate),
			huh.NewInput().
				Title("Not Before (YYYY-MM-DD, blank to clear)").
				Placeholder("2025-01-15").
				Value(&f.notBefore).
				Validate(validateOptionalDate),
			huh.NewInput().
				Title("Min Session Minutes (blank for default)").
				Placeholder("15").
				Value(&f.minSession).
				Validate(validateNonNegativeInt),
			huh.NewInput().
				Title("Max Session Minutes (blank for default)").
				Placeholder("120").
				Value(&f.maxSession).
				Validate(validateNonNegativeInt),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		return func() tea.Msg { return applyEditWorkItem(state.App, itemID, f) }
	}

	return newWizardView(state, "Edit Work Item", form, done)
}

// newAddWorkItemView creates a wizard form for adding a new work item to a node.
// Collects title and planned duration, then creates via the service layer.
func newAddWorkItemView(state *SharedState, nodeID string) View {
	var newTitle string
	newDuration := "60"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Placeholder("Work item title").
				Value(&newTitle).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Planned Minutes").
				Placeholder("60").
				Value(&newDuration).
				Validate(validatePositiveInt),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		return func() tea.Msg {
			dur := parsePositiveInt(newDuration, 60)

			w := &domain.WorkItem{
				NodeID:     nodeID,
				Title:      newTitle,
				Type:       "task",
				PlannedMin: dur,
			}
			if err := state.App.WorkItems.Create(context.Background(), w); err != nil {
				return formErrorOutput(err)
			}

			state.SetActiveItem(w.ID, w.Title, w.Seq)
			return formSuccessOutput(fmt.Sprintf("%s Added: %s (%s)",
				formatter.StyleGreen.Render("✔"),
				formatter.Bold(newTitle),
				formatter.FormatMinutes(dur)))
		}
	}

	return newWizardView(state, "Add Work Item", form, done)
}

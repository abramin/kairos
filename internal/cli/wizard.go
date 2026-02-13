package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// kairosHuhTheme returns a custom huh theme using the existing Gruvbox palette.
func kairosHuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Focused state: orange accent
	t.Focused.Title = lipgloss.NewStyle().Foreground(formatter.ColorHeader).Bold(true)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(formatter.ColorHeader)
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(formatter.ColorGreen)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(formatter.ColorFg)
	t.Focused.FocusedButton = lipgloss.NewStyle().Foreground(formatter.ColorFg).Background(formatter.ColorHeader).Padding(0, 1)
	t.Focused.BlurredButton = lipgloss.NewStyle().Foreground(formatter.ColorDim).Padding(0, 1)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(formatter.ColorHeader)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(formatter.ColorHeader)
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(formatter.ColorFg)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Focused.Description = lipgloss.NewStyle().Foreground(formatter.ColorDim)

	// Blurred state: dimmed
	t.Blurred.Title = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Blurred.SelectSelector = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Blurred.UnselectedOption = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(formatter.ColorDim)
	t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(formatter.ColorDim)

	return t
}

// wizardSelectProject creates a huh form to select a project from the list.
func wizardSelectProject(ctx context.Context, app *App, result *string) *huh.Form {
	projects, err := app.Projects.List(ctx, false)
	if err != nil || len(projects) == 0 {
		return nil
	}

	options := make([]huh.Option[string], 0, len(projects))
	for _, p := range projects {
		label := p.Name
		if p.ShortID != "" {
			label = fmt.Sprintf("%s — %s", p.ShortID, p.Name)
		}
		options = append(options, huh.NewOption(label, p.ID))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which Project?").
				Options(options...).
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// wizardSelectWorkItem creates a huh form to select a work item from a project.
// filterStatuses limits the displayed items to matching statuses (nil = show all non-done/archived).
func wizardSelectWorkItem(ctx context.Context, app *App, projectID string, filterStatuses []domain.WorkItemStatus, result *string) *huh.Form {
	items, err := app.WorkItems.ListByProject(ctx, projectID)
	if err != nil || len(items) == 0 {
		return nil
	}

	// Build status filter set.
	statusSet := make(map[domain.WorkItemStatus]bool)
	if len(filterStatuses) > 0 {
		for _, s := range filterStatuses {
			statusSet[s] = true
		}
	}

	options := make([]huh.Option[string], 0, len(items))
	for _, w := range items {
		// Skip archived/done unless explicitly requested.
		if len(statusSet) > 0 {
			if !statusSet[w.Status] {
				continue
			}
		} else {
			if w.Status == domain.WorkItemDone || w.Status == domain.WorkItemArchived || w.Status == domain.WorkItemSkipped {
				continue
			}
		}
		label := fmt.Sprintf("#%d — %s", w.Seq, w.Title)
		if w.Status == domain.WorkItemInProgress {
			label += " (active)"
		}
		options = append(options, huh.NewOption(label, w.ID))
	}

	if len(options) == 0 {
		return nil
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which Item?").
				Options(options...).
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// wizardSelectNode creates a huh form to select a plan node from a project.
func wizardSelectNode(ctx context.Context, app *App, projectID string, result *string) *huh.Form {
	nodes, err := app.Nodes.ListByProject(ctx, projectID)
	if err != nil || len(nodes) == 0 {
		return nil
	}

	options := make([]huh.Option[string], 0, len(nodes))
	for _, n := range nodes {
		label := fmt.Sprintf("#%d — %s (%s)", n.Seq, n.Title, n.Kind)
		options = append(options, huh.NewOption(label, n.ID))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which Node?").
				Options(options...).
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// parsePositiveInt parses s as a positive integer, returning fallback if s is
// empty, non-numeric, or non-positive. Used after huh form validation has
// already ensured the string is valid, so this is a safe conversion.
func parsePositiveInt(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

// validatePositiveInt accepts empty or a positive integer.
func validatePositiveInt(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return fmt.Errorf("enter a positive number")
	}
	return nil
}

// validateNonNegativeInt accepts empty or a non-negative integer.
func validateNonNegativeInt(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return fmt.Errorf("enter a non-negative number")
	}
	return nil
}

// wizardInputDuration creates a huh form to enter session duration in minutes.
func wizardInputDuration(defaultMin int, result *string) *huh.Form {
	defStr := strconv.Itoa(defaultMin)
	*result = defStr

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Duration (minutes)").
				Placeholder(defStr).
				Value(result).
				Validate(validatePositiveInt),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// wizardInputText creates a huh form for a single text input.
func wizardInputText(title, placeholder string, required bool, result *string) *huh.Form {
	input := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(result)

	if required {
		input = input.Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("%s is required", title)
			}
			return nil
		})
	}

	return huh.NewForm(
		huh.NewGroup(input),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// wizardSelectNodeKind creates a huh form to select a plan node kind.
func wizardSelectNodeKind(result *string) *huh.Form {
	*result = "module" // default
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Node Kind").
				Options(
					huh.NewOption("Module", "module"),
					huh.NewOption("Week", "week"),
					huh.NewOption("Section", "section"),
					huh.NewOption("Stage", "stage"),
					huh.NewOption("Assessment", "assessment"),
					huh.NewOption("Generic", "generic"),
				).
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// wizardSelectWorkItemType creates a huh form to select a work item type.
func wizardSelectWorkItemType(result *string) *huh.Form {
	*result = "task" // default
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Work Item Type").
				Options(
					huh.NewOption("Reading", "reading"),
					huh.NewOption("Practice", "practice"),
					huh.NewOption("Review", "review"),
					huh.NewOption("Assignment", "assignment"),
					huh.NewOption("Task", "task"),
				).
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// validateOptionalDate accepts empty or a YYYY-MM-DD date string.
func validateOptionalDate(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("use YYYY-MM-DD format")
	}
	return nil
}

// wizardConfirm creates a huh form for a yes/no confirmation.
func wizardConfirm(title string, result *bool) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Affirmative("Yes").
				Negative("No").
				Value(result),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

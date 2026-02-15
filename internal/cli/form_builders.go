package cli

import "github.com/charmbracelet/huh"

// dateInput returns a huh.Input for an optional date field with YYYY-MM-DD validation.
func dateInput(title, placeholder string, value *string) *huh.Input {
	if placeholder == "" {
		placeholder = "2025-06-30"
	}
	return huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(value).
		Validate(validateOptionalDate)
}

// dueDateForm returns a themed single-field Form for collecting an optional due date.
func dueDateForm(value *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			dateInput("Due Date (YYYY-MM-DD, blank for none)", "", value),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
}

// durationInput returns a huh.Input for a positive integer duration field.
func durationInput(title, placeholder string, value *string) *huh.Input {
	if title == "" {
		title = "Planned Minutes"
	}
	if placeholder == "" {
		placeholder = "60"
	}
	return huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(value).
		Validate(validatePositiveInt)
}

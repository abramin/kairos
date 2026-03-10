package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// cmdHabits dispatches habit subcommands: list (default), add, delete.
func (c *commandBar) cmdHabits(args []string) tea.Cmd {
	if c.state.App.Habits == nil {
		return outputCmd(formatter.StyleYellow.Render("Habits service not available."))
	}

	sub := "list"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
		args = args[1:]
	}

	switch sub {
	case "list", "":
		return pushView(newHabitListView(c.state))
	case "add":
		return c.cmdHabitAdd()
	case "delete", "remove", "archive":
		if len(args) == 0 {
			return outputCmd(formatter.StyleYellow.Render("Usage: habits delete <title-prefix>"))
		}
		ref := strings.Join(args, " ")
		return asyncOutputCmd(func() string {
			result, err := execHabitDelete(context.Background(), c.state.App, ref)
			if err != nil {
				return shellError(err)
			}
			return result
		})
	default:
		return outputCmd(fmt.Sprintf("habits subcommands: list, add, delete"))
	}
}

// cmdLogHabit logs a habit session: log-habit <title-prefix> [minutes]
func (c *commandBar) cmdLogHabit(args []string) tea.Cmd {
	if c.state.App.Habits == nil {
		return outputCmd(formatter.StyleYellow.Render("Habits service not available."))
	}
	if len(args) == 0 {
		return outputCmd(formatter.StyleYellow.Render("Usage: log-habit <title-prefix> [minutes]"))
	}

	// Last arg may be an integer (minutes); everything else is the title ref.
	var ref string
	var minutesStr string
	last := args[len(args)-1]
	if _, err := strconv.Atoi(last); err == nil && len(args) > 1 {
		minutesStr = last
		ref = strings.Join(args[:len(args)-1], " ")
	} else {
		ref = strings.Join(args, " ")
	}

	return asyncOutputCmd(func() string {
		result, err := execHabitLog(context.Background(), c.state.App, ref, minutesStr)
		if err != nil {
			return shellError(err)
		}
		return result
	})
}

// cmdHabitAdd opens a wizard to create a new habit.
func (c *commandBar) cmdHabitAdd() tea.Cmd {
	ctx := context.Background()
	var title string
	var cadenceDaysStr string
	var targetMinStr string

	form := wizardHabitAdd(&title, &cadenceDaysStr, &targetMinStr)
	return startWizardCmd(c.state, "Add Habit", form, func() tea.Cmd {
		return asyncOutputCmd(func() string {
			cadence := 1
			if v, err := strconv.Atoi(cadenceDaysStr); err == nil && v > 0 {
				cadence = v
			}
			targetMin := 20
			if v, err := strconv.Atoi(targetMinStr); err == nil && v > 0 {
				targetMin = v
			}
			h, err := c.state.App.Habits.Add(ctx, service.AddHabitRequest{
				Title:       title,
				CadenceDays: cadence,
				TargetMin:   targetMin,
			})
			if err != nil {
				return shellError(err)
			}
			cadenceLabel := "daily"
			if cadence == 7 {
				cadenceLabel = "weekly"
			} else if cadence > 1 {
				cadenceLabel = fmt.Sprintf("every %d days", cadence)
			}
			return fmt.Sprintf("%s Habit added: %s (%s, %s/session)",
				formatter.StyleGreen.Render("✔"),
				formatter.Bold(h.Title),
				cadenceLabel,
				formatter.FormatMinutes(h.TargetMin),
			)
		})
	})
}

// execHabitList returns a formatted table of active habits with status.
func execHabitList(ctx context.Context, app *App) (string, error) {
	statuses, err := app.Habits.GetStatus(ctx, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if len(statuses) == 0 {
		return formatter.Dim("No habits configured. Use 'habits add' to create one."), nil
	}

	headers := []string{"TITLE", "CADENCE", "TARGET", "LAST LOGGED", "STATUS"}
	rows := make([][]string, 0, len(statuses))
	for _, s := range statuses {
		cadence := "daily"
		if s.Habit.CadenceDays == 7 {
			cadence = "weekly"
		} else if s.Habit.CadenceDays > 1 {
			cadence = fmt.Sprintf("every %dd", s.Habit.CadenceDays)
		}

		lastLogged := "never"
		if s.LastLog != nil {
			switch s.DaysSinceLog {
			case 0:
				lastLogged = "today"
			case 1:
				lastLogged = "yesterday"
			default:
				lastLogged = fmt.Sprintf("%d days ago", s.DaysSinceLog)
			}
		}

		var status string
		switch {
		case s.DaysSinceLog == 0:
			status = formatter.StyleGreen.Render("done today")
		case s.DaysSinceLog == 9999:
			status = formatter.StyleYellow.Render("never done")
		case s.DaysUntilDue < 0:
			status = formatter.StyleYellow.Render(fmt.Sprintf("overdue %d days", -s.DaysUntilDue))
		case s.DaysUntilDue == 0:
			status = formatter.StyleYellow.Render("due today")
		default:
			status = formatter.Dim(fmt.Sprintf("due in %d days", s.DaysUntilDue))
		}

		rows = append(rows, []string{
			s.Habit.Title,
			cadence,
			formatter.FormatMinutes(s.Habit.TargetMin),
			lastLogged,
			status,
		})
	}
	return formatter.RenderTable(headers, rows), nil
}

// execHabitDelete archives a habit by title prefix.
func execHabitDelete(ctx context.Context, app *App, ref string) (string, error) {
	h, err := resolveHabitByRef(ctx, app, ref)
	if err != nil {
		return "", err
	}
	if err := app.Habits.Archive(ctx, h.ID); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s Habit archived: %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(h.Title),
	), nil
}

// execHabitLog logs a session for a habit identified by title prefix.
func execHabitLog(ctx context.Context, app *App, ref, minutesStr string) (string, error) {
	h, err := resolveHabitByRef(ctx, app, ref)
	if err != nil {
		return "", err
	}

	minutes := 0
	if minutesStr != "" {
		if v, err := strconv.Atoi(minutesStr); err == nil && v > 0 {
			minutes = v
		}
	}

	log, err := app.Habits.LogSession(ctx, service.LogHabitRequest{
		HabitID: h.ID,
		Minutes: minutes,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s Logged %s for %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(formatter.FormatMinutes(log.Minutes)),
		formatter.Bold(h.Title),
	), nil
}

// resolveHabitByRef finds a habit by case-insensitive title prefix.
func resolveHabitByRef(ctx context.Context, app *App, ref string) (*habitRef, error) {
	habits, err := app.Habits.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading habits: %w", err)
	}
	lower := strings.ToLower(strings.TrimSpace(ref))
	var matches []*habitRef
	for _, h := range habits {
		if strings.HasPrefix(strings.ToLower(h.Title), lower) {
			matches = append(matches, &habitRef{ID: h.ID, Title: h.Title})
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no habit matching %q", ref)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Title
		}
		return nil, fmt.Errorf("ambiguous: %q matches multiple habits: %s", ref, strings.Join(names, ", "))
	}
}

type habitRef struct {
	ID    string
	Title string
}

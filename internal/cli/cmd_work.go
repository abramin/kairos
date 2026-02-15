package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
)

// resolveItemTitle fetches the work item title and seq, falling back to a truncated ID.
func resolveItemTitle(ctx context.Context, app *App, itemID string) (string, int) {
	if wi, err := app.WorkItems.GetByID(ctx, itemID); err == nil {
		return wi.Title, wi.Seq
	}
	return formatter.TruncID(itemID), 0
}

// ── item resolution helper ───────────────────────────────────────────────────

// resolveOrSelectItem resolves a work item ID from args, active context,
// or last recommendation; falls back to a selection wizard.
func (c *commandBar) resolveOrSelectItem(
	itemArg string,
	statusFilter []domain.WorkItemStatus,
	next func(itemID string) tea.Cmd,
) tea.Cmd {
	ctx := context.Background()

	// 1. Try explicit argument.
	if itemArg != "" {
		if resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID); err == nil {
			return next(resolved)
		}
	}
	// 2. Try active context.
	if c.state.ActiveItemID != "" {
		return next(c.state.ActiveItemID)
	}
	// 3. Try last recommended.
	if c.state.LastRecommendedItemID != "" {
		return next(c.state.LastRecommendedItemID)
	}
	// 4. Wizard fallback.
	var result string
	form := wizardSelectWorkItem(ctx, c.state.App, c.state.ActiveProjectID, statusFilter, &result)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No matching items found."))
	}
	return startWizardCmd(c.state, "Select Item", form, func() tea.Cmd {
		return next(result)
	})
}

// ── log command ──────────────────────────────────────────────────────────────

func (c *commandBar) cmdLog(args []string) tea.Cmd {
	itemArg, minutesArg := parseLogArgs(args)
	return c.ensureProject(func() tea.Cmd {
		return c.resolveOrSelectItem(itemArg, nil, func(itemID string) tea.Cmd {
			return c.logAfterItem(itemID, minutesArg)
		})
	})
}

func (c *commandBar) logAfterItem(itemID, minutesArg string) tea.Cmd {
	if minutesArg != "" {
		return c.logExecute(itemID, minutesArg)
	}

	defaultMin := 60
	if c.state.LastDuration > 0 {
		defaultMin = c.state.LastDuration
	}

	var result string
	form := wizardInputDuration(defaultMin, &result)
	return startWizardCmd(c.state, "Duration", form, func() tea.Cmd {
		if result == "" {
			result = strconv.Itoa(defaultMin)
		}
		return c.logExecute(itemID, result)
	})
}

func (c *commandBar) logExecute(itemID, minutesStr string) tea.Cmd {
	ctx := context.Background()
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		return outputCmd(formatter.StyleRed.Render("Invalid duration."))
	}

	title, seq := resolveItemTitle(ctx, c.state.App, itemID)
	c.state.SetActiveItem(itemID, title, seq)

	msg, err := execLogSession(ctx, c.state.App, c.state, LogSessionInput{
		ItemID: itemID, Title: title, Minutes: minutes,
	})
	if err != nil {
		return outputCmd(shellError(err))
	}
	return outputCmd(msg)
}

// ── start command ────────────────────────────────────────────────────────────

func (c *commandBar) cmdStart(args []string) tea.Cmd {
	var itemArg string
	if len(args) > 0 {
		itemArg = stripItemPrefix(args[0])
	}
	return c.ensureProject(func() tea.Cmd {
		return c.startAfterProject(itemArg)
	})
}

func (c *commandBar) startAfterProject(itemArg string) tea.Cmd {
	ctx := context.Background()

	// Start only accepts an explicit arg or wizard — no context/recommended
	// fallback, since those items may already be in-progress.
	if itemArg != "" {
		if resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID); err == nil {
			return c.startExecute(resolved)
		}
	}

	var result string
	form := wizardSelectWorkItem(ctx, c.state.App, c.state.ActiveProjectID,
		[]domain.WorkItemStatus{domain.WorkItemTodo}, &result)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No todo items found."))
	}
	return startWizardCmd(c.state, "Select Item", form, func() tea.Cmd {
		return c.startExecute(result)
	})
}

func (c *commandBar) startExecute(itemID string) tea.Cmd {
	ctx := context.Background()
	title, seq := resolveItemTitle(ctx, c.state.App, itemID)

	msg, err := execStartItem(ctx, c.state.App, c.state, itemID, title, seq)
	if err != nil {
		return outputCmd(shellError(err))
	}
	return outputCmd(msg)
}

// ── finish command ───────────────────────────────────────────────────────────

func (c *commandBar) cmdFinish(args []string) tea.Cmd {
	var itemArg string
	if len(args) > 0 {
		itemArg = stripItemPrefix(args[0])
	}

	// Finish checks arg and active context before ensuring a project,
	// since the active item may already identify the project.
	if itemArg != "" {
		ctx := context.Background()
		if resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID); err == nil {
			return c.finishExecute(resolved)
		}
	}
	if c.state.ActiveItemID != "" {
		return c.finishExecute(c.state.ActiveItemID)
	}

	return c.ensureProject(func() tea.Cmd {
		return c.resolveOrSelectItem("", []domain.WorkItemStatus{domain.WorkItemInProgress}, func(itemID string) tea.Cmd {
			return c.finishExecute(itemID)
		})
	})
}

func (c *commandBar) finishExecute(itemID string) tea.Cmd {
	ctx := context.Background()
	title, _ := resolveItemTitle(ctx, c.state.App, itemID)

	msg, err := execMarkDone(ctx, c.state.App, c.state, itemID, title)
	if err != nil {
		return outputCmd(shellError(err))
	}
	return outputCmd(msg)
}

// ── add command ──────────────────────────────────────────────────────────────

func (c *commandBar) cmdAdd(args []string) tea.Cmd {
	if c.state.ActiveProjectID == "" {
		return outputCmd(formatter.StyleYellow.Render("Set a project first: use <id>"))
	}
	if len(args) == 0 {
		return outputCmd(formatter.StyleYellow.Render("Usage: add [#node] <title> [duration]"))
	}

	var nodeArg string
	var titleParts []string
	minutesArg := 0

	for i, a := range args {
		if strings.HasPrefix(a, "#") {
			nodeArg = stripItemPrefix(a)
		} else if i == len(args)-1 {
			if dur, ok := parseDurationArg(a); ok {
				minutesArg = dur
			} else {
				titleParts = append(titleParts, a)
			}
		} else {
			titleParts = append(titleParts, a)
		}
	}

	title := strings.Join(titleParts, " ")
	if title == "" {
		return outputCmd(formatter.StyleYellow.Render("Usage: add [#node] <title> [duration]"))
	}

	if nodeArg != "" {
		ctx := context.Background()
		nodeID, err := resolveNodeID(ctx, c.state.App, nodeArg, c.state.ActiveProjectID)
		if err != nil {
			return outputCmd(shellError(err))
		}
		return c.addAfterNode(nodeID, title, minutesArg)
	}

	return c.addSelectNode(title, minutesArg)
}

func (c *commandBar) addSelectNode(title string, minutesArg int) tea.Cmd {
	ctx := context.Background()
	var nodeID string
	form := wizardSelectNode(ctx, c.state.App, c.state.ActiveProjectID, &nodeID)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No nodes found in this project."))
	}
	return startWizardCmd(c.state, "Select Node", form, func() tea.Cmd {
		return c.addAfterNode(nodeID, title, minutesArg)
	})
}

func (c *commandBar) addAfterNode(nodeID, title string, minutesArg int) tea.Cmd {
	if minutesArg > 0 {
		return c.addGetDueDate(nodeID, title, minutesArg)
	}

	var result string
	form := wizardInputDuration(60, &result)
	return startWizardCmd(c.state, "Duration", form, func() tea.Cmd {
		return c.addGetDueDate(nodeID, title, parsePositiveInt(result, 60))
	})
}

func (c *commandBar) addGetDueDate(nodeID, title string, minutes int) tea.Cmd {
	var dueDate string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Due Date (YYYY-MM-DD, blank for none)").
				Placeholder("2025-06-30").
				Value(&dueDate).
				Validate(validateOptionalDate),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)
	return startWizardCmd(c.state, "Due Date", form, func() tea.Cmd {
		return c.addExecute(nodeID, title, minutes, dueDate)
	})
}

func (c *commandBar) addExecute(nodeID, title string, minutes int, dueDate string) tea.Cmd {
	ctx := context.Background()

	w := &domain.WorkItem{
		ID:        uuid.New().String(),
		NodeID:    nodeID,
		Title:     title,
		Type:      "task",
		Status:    domain.WorkItemTodo,
		PlannedMin: minutes,
	}
	if dueDate != "" {
		if t, err := time.Parse("2006-01-02", dueDate); err == nil {
			w.DueDate = &t
		}
	}
	if err := c.state.App.WorkItems.Create(ctx, w); err != nil {
		return outputCmd(shellError(err))
	}

	// Try to set the new item as active context.
	if items, err := c.state.App.WorkItems.ListByNode(ctx, nodeID); err == nil && len(items) > 0 {
		newest := items[len(items)-1]
		c.state.SetActiveItem(newest.ID, newest.Title, newest.Seq)
	}

	nodeTitle := ""
	if node, err := c.state.App.Nodes.GetByID(ctx, nodeID); err == nil {
		nodeTitle = node.Title
	}

	msg := fmt.Sprintf("%s Added: %s (%s)",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(title),
		formatter.FormatMinutes(minutes),
	)
	if nodeTitle != "" {
		msg += fmt.Sprintf(" to %s", formatter.Bold(nodeTitle))
	}
	return tea.Batch(
		outputCmd(msg),
		func() tea.Msg { return refreshViewMsg{} },
	)
}

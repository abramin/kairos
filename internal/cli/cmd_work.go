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
	"github.com/google/uuid"
)

// ── log command ──────────────────────────────────────────────────────────────

func (c *commandBar) cmdLog(args []string) tea.Cmd {
	itemArg, minutesArg := parseLogArgs(args)
	return c.ensureProject(func() tea.Cmd {
		return c.logAfterProject(itemArg, minutesArg)
	})
}

func (c *commandBar) logAfterProject(itemArg, minutesArg string) tea.Cmd {
	ctx := context.Background()

	itemID := ""
	if itemArg != "" {
		resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID)
		if err == nil {
			itemID = resolved
		}
	}
	if itemID == "" && c.state.ActiveItemID != "" {
		itemID = c.state.ActiveItemID
	}
	if itemID == "" && c.state.LastRecommendedItemID != "" {
		itemID = c.state.LastRecommendedItemID
	}

	if itemID == "" {
		var result string
		form := wizardSelectWorkItem(ctx, c.state.App, c.state.ActiveProjectID, nil, &result)
		if form == nil {
			return outputCmd(formatter.StyleYellow.Render("No work items found in this project."))
		}
		return startWizardCmd(c.state, "Select Item", form, func() tea.Cmd {
			return c.logAfterItem(result, minutesArg)
		})
	}

	return c.logAfterItem(itemID, minutesArg)
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

	s := &domain.WorkSessionLog{
		ID:         uuid.New().String(),
		WorkItemID: itemID,
		StartedAt:  time.Now(),
		Minutes:    minutes,
		CreatedAt:  time.Now(),
	}
	if err := c.state.App.Sessions.LogSession(ctx, s); err != nil {
		return outputCmd(shellError(err))
	}

	c.state.ActiveItemID = itemID
	c.state.LastDuration = minutes

	title := formatter.TruncID(itemID)
	if wi, err := c.state.App.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
		c.state.SetActiveItem(wi.ID, wi.Title, wi.Seq)
	}

	return outputCmd(fmt.Sprintf("%s Logged %s to %s.",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(formatter.FormatMinutes(minutes)),
		formatter.Bold(title),
	))
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

	if itemArg != "" {
		resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID)
		if err == nil {
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
	if err := c.state.App.WorkItems.MarkInProgress(ctx, itemID); err != nil {
		return outputCmd(shellError(err))
	}

	title := formatter.TruncID(itemID)
	if wi, err := c.state.App.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
		c.state.SetActiveItem(wi.ID, wi.Title, wi.Seq)
	}

	return outputCmd(fmt.Sprintf("%s Started: %s",
		formatter.StyleGreen.Render("▶"),
		formatter.Bold(title),
	))
}

// ── finish command ───────────────────────────────────────────────────────────

func (c *commandBar) cmdFinish(args []string) tea.Cmd {
	ctx := context.Background()

	var itemArg string
	if len(args) > 0 {
		itemArg = stripItemPrefix(args[0])
	}

	if itemArg != "" {
		resolved, err := resolveWorkItemID(ctx, c.state.App, itemArg, c.state.ActiveProjectID)
		if err == nil {
			return c.finishExecute(resolved)
		}
	}

	if c.state.ActiveItemID != "" {
		return c.finishExecute(c.state.ActiveItemID)
	}

	return c.ensureProject(func() tea.Cmd {
		return c.finishAfterProject()
	})
}

func (c *commandBar) finishAfterProject() tea.Cmd {
	ctx := context.Background()
	var result string
	form := wizardSelectWorkItem(ctx, c.state.App, c.state.ActiveProjectID,
		[]domain.WorkItemStatus{domain.WorkItemInProgress}, &result)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No in-progress items found."))
	}
	return startWizardCmd(c.state, "Select Item", form, func() tea.Cmd {
		return c.finishExecute(result)
	})
}

func (c *commandBar) finishExecute(itemID string) tea.Cmd {
	ctx := context.Background()
	if err := c.state.App.WorkItems.MarkDone(ctx, itemID); err != nil {
		return outputCmd(shellError(err))
	}

	title := formatter.TruncID(itemID)
	if wi, err := c.state.App.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
	}

	if c.state.ActiveItemID == itemID {
		c.state.ClearItemContext()
	}

	return outputCmd(fmt.Sprintf("%s Finished: %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(title),
	))
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
		return c.addExecute(nodeID, title, minutesArg)
	}

	var result string
	form := wizardInputDuration(60, &result)
	return startWizardCmd(c.state, "Duration", form, func() tea.Cmd {
		dur := 60
		if v, err := strconv.Atoi(result); err == nil && v > 0 {
			dur = v
		}
		return c.addExecute(nodeID, title, dur)
	})
}

func (c *commandBar) addExecute(nodeID, title string, minutes int) tea.Cmd {
	ctx := context.Background()

	cobraArgs := []string{"work", "add",
		"--node", nodeID,
		"--title", title,
		"--type", "task",
		"--planned-min", strconv.Itoa(minutes),
	}
	output := c.cobraCapture(cobraArgs)

	if strings.Contains(output, "Error") {
		return outputCmd(output)
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
	return outputCmd(msg)
}

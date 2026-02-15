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

// ── entity group commands (node/work/session/project) ────────────────────────

func (c *commandBar) cmdEntityGroup(parts []string) tea.Cmd {
	if len(parts) < 2 {
		group := strings.ToLower(parts[0])
		return outputCmd(entityGroupHelp(group))
	}

	group := strings.ToLower(parts[0])
	sub := strings.ToLower(parts[1])

	// Route "project draft" to the draft view.
	if group == "project" && sub == "draft" {
		description := ""
		if len(parts) > 2 {
			description = strings.Join(parts[2:], " ")
		}
		return pushView(newDraftView(c.state, description))
	}

	// Bare creation commands → launch wizard.
	if c.shouldStartEntityWizard(group, sub, parts) {
		return c.cmdEntityWizard(group, sub)
	}

	// Destructive commands → confirmation.
	if subs, ok := destructiveCommands[group]; ok && subs[sub] {
		return c.cmdDestructive(parts, group, sub)
	}

	// Commands that mutate project data need a dashboard refresh.
	mutating := map[string]bool{"import": true, "add": true, "update": true, "init": true, "archive": true, "unarchive": true}
	if mutating[sub] {
		return tea.Batch(
			c.dispatchEntityCommand(group, sub, parts[2:]),
			func() tea.Msg { return refreshViewMsg{} },
		)
	}

	return c.dispatchEntityCommand(group, sub, parts[2:])
}

func (c *commandBar) shouldStartEntityWizard(group, sub string, parts []string) bool {
	if len(parts) != 2 {
		return false
	}
	switch group {
	case "work":
		return sub == "add"
	case "session":
		return sub == "log"
	case "node":
		return sub == "add"
	}
	return false
}

func (c *commandBar) cmdEntityWizard(group, sub string) tea.Cmd {
	switch group + " " + sub {
	case "session log":
		return c.cmdLog(nil)
	case "work add":
		return c.wizardWorkAdd()
	case "node add":
		return c.wizardNodeAdd()
	}
	return nil
}

func (c *commandBar) wizardWorkAdd() tea.Cmd {
	return c.ensureProject(func() tea.Cmd {
		return c.workAddSelectNode()
	})
}

func (c *commandBar) workAddSelectNode() tea.Cmd {
	ctx := context.Background()
	var nodeID string
	form := wizardSelectNode(ctx, c.state.App, c.state.ActiveProjectID, &nodeID)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No nodes found in this project."))
	}
	return startWizardCmd(c.state, "Select Node", form, func() tea.Cmd {
		return c.workAddGetTitle(nodeID)
	})
}

func (c *commandBar) workAddGetTitle(nodeID string) tea.Cmd {
	var title string
	form := wizardInputText("Title", "Work item title", true, &title)
	return startWizardCmd(c.state, "Title", form, func() tea.Cmd {
		return c.workAddGetType(nodeID, title)
	})
}

func (c *commandBar) workAddGetType(nodeID, title string) tea.Cmd {
	var wiType string
	form := wizardSelectWorkItemType(&wiType)
	return startWizardCmd(c.state, "Type", form, func() tea.Cmd {
		return c.workAddGetMinutes(nodeID, title, wiType)
	})
}

func (c *commandBar) workAddGetMinutes(nodeID, title, wiType string) tea.Cmd {
	var minutes string
	form := wizardInputDuration(60, &minutes)
	return startWizardCmd(c.state, "Duration", form, func() tea.Cmd {
		return c.workAddGetDueDate(nodeID, title, wiType, minutes)
	})
}

func (c *commandBar) workAddGetDueDate(nodeID, title, wiType, minutes string) tea.Cmd {
	var dueDate string
	return startWizardCmd(c.state, "Due Date", dueDateForm(&dueDate), func() tea.Cmd {
		ctx := context.Background()
		w := &domain.WorkItem{
			ID:        uuid.New().String(),
			NodeID:    nodeID,
			Title:     title,
			Type:      wiType,
			Status:    domain.WorkItemTodo,
		}
		if v, err := strconv.Atoi(minutes); err == nil && v > 0 {
			w.PlannedMin = v
		}
		if dueDate != "" {
			if t, err := time.Parse("2006-01-02", dueDate); err == nil {
				w.DueDate = &t
			}
		}
		if err := c.state.App.WorkItems.Create(ctx, w); err != nil {
			return outputCmd(shellError(err))
		}
		return tea.Batch(
			outputCmd(fmt.Sprintf("%s Created: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(title))),
			func() tea.Msg { return refreshViewMsg{} },
		)
	})
}

func (c *commandBar) wizardNodeAdd() tea.Cmd {
	return c.ensureProject(func() tea.Cmd {
		return c.nodeAddGetTitle()
	})
}

func (c *commandBar) nodeAddGetTitle() tea.Cmd {
	var title string
	form := wizardInputText("Title", "Node title", true, &title)
	return startWizardCmd(c.state, "Title", form, func() tea.Cmd {
		return c.nodeAddGetKind(title)
	})
}

func (c *commandBar) nodeAddGetKind(title string) tea.Cmd {
	var kind string
	form := wizardSelectNodeKind(&kind)
	return startWizardCmd(c.state, "Kind", form, func() tea.Cmd {
		ctx := context.Background()
		n := &domain.PlanNode{
			ID:        uuid.New().String(),
			ProjectID: c.state.ActiveProjectID,
			Title:     title,
			Kind:      domain.NodeKind(kind),
		}
		if err := c.state.App.Nodes.Create(ctx, n); err != nil {
			return outputCmd(shellError(err))
		}
		return tea.Batch(
			outputCmd(fmt.Sprintf("%s Created node: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(title))),
			func() tea.Msg { return refreshViewMsg{} },
		)
	})
}

// ── destructive command confirmation ─────────────────────────────────────────

func (c *commandBar) cmdDestructive(parts []string, group, sub string) tea.Cmd {
	// If --yes or --force is present, skip confirmation.
	for _, a := range parts[2:] {
		if a == "--yes" || a == "-y" || a == "--force" {
			return tea.Batch(
				c.dispatchEntityCommand(group, sub, parts[2:]),
				func() tea.Msg { return refreshViewMsg{} },
			)
		}
	}

	target := ""
	if len(parts) > 2 {
		target = parts[2]
	}
	desc := fmt.Sprintf("%s %s", group, sub)
	if target != "" {
		desc += " " + target
	}

	var confirmed bool
	form := wizardConfirm(desc+"?", &confirmed)
	return startWizardCmd(c.state, "Confirm", form, func() tea.Cmd {
		if confirmed {
			return tea.Batch(
				c.dispatchEntityCommand(group, sub, parts[2:]),
				func() tea.Msg { return refreshViewMsg{} },
			)
		}
		return outputCmd(formatter.Dim("Cancelled."))
	})
}

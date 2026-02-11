package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	tea "github.com/charmbracelet/bubbletea"
)

// ── navigation & info commands ───────────────────────────────────────────────

func (c *commandBar) cmdProjects() tea.Cmd {
	ctx := context.Background()
	projects, err := c.state.App.Projects.List(ctx, false)
	if err != nil {
		return outputCmd(shellError(err))
	}
	if len(projects) == 0 {
		return outputCmd(formatter.Dim("No projects found."))
	}
	return outputCmd(formatter.FormatProjectList(projects))
}

func (c *commandBar) cmdUse(args []string) tea.Cmd {
	if len(args) == 0 {
		c.state.ClearProjectContext()
		return outputCmd(formatter.Dim("Cleared active project."))
	}

	ctx := context.Background()
	projectID, err := resolveProjectID(ctx, c.state.App, args[0])
	if err != nil {
		return outputCmd(shellError(err))
	}

	project, err := c.state.App.Projects.GetByID(ctx, projectID)
	if err != nil {
		return outputCmd(shellError(err))
	}

	c.state.SetActiveProjectFrom(project)
	c.state.ClearItemContext()

	return outputCmd(fmt.Sprintf("Active project: %s %s",
		formatter.Bold(project.Name),
		formatter.Dim(c.state.ActiveShortID),
	))
}

func (c *commandBar) cmdInspect(args []string) tea.Cmd {
	ctx := context.Background()
	projectID := c.state.ActiveProjectID

	if len(args) > 0 {
		resolved, err := resolveProjectID(ctx, c.state.App, args[0])
		if err != nil {
			return outputCmd(shellError(err))
		}
		projectID = resolved
	}

	if projectID == "" {
		return outputCmd(formatter.StyleYellow.Render(
			"No active project. Use 'use <id>' to select one, or 'inspect <id>'."))
	}

	c.state.SetActiveProject(ctx, projectID)
	c.state.ClearItemContext()
	return pushView(newTaskListView(c.state))
}

func (c *commandBar) cmdStatus() tea.Cmd {
	ctx := context.Background()
	req := contract.NewStatusRequest()
	if c.state.ActiveProjectID != "" {
		req.ProjectScope = []string{c.state.ActiveProjectID}
	}
	resp, err := c.state.App.Status.GetStatus(ctx, req)
	if err != nil {
		return outputCmd(shellError(err))
	}
	return outputCmd(formatter.FormatStatus(resp))
}

func (c *commandBar) cmdWhatNow(args []string) tea.Cmd {
	minutes := 60
	if len(args) > 0 {
		if m, err := strconv.Atoi(args[0]); err == nil && m > 0 {
			minutes = m
		}
	}

	ctx := context.Background()
	req := contract.NewWhatNowRequest(minutes)
	resp, err := c.state.App.WhatNow.Recommend(ctx, req)
	if err != nil {
		return outputCmd(shellError(err))
	}
	return outputCmd(formatter.FormatWhatNow(resp))
}

func (c *commandBar) cmdContext(args []string) tea.Cmd {
	if len(args) == 0 {
		if c.state.ActiveProjectID == "" {
			return outputCmd(formatter.Dim("No active context."))
		}
		out := fmt.Sprintf("Project: %s %s",
			formatter.Bold(c.state.ActiveProjectName),
			formatter.Dim(c.state.ActiveShortID),
		)
		if c.state.ActiveItemID != "" {
			out += fmt.Sprintf("\nItem: #%d %s", c.state.ActiveItemSeq, c.state.ActiveItemTitle)
		}
		return outputCmd(out)
	}

	switch strings.ToLower(args[0]) {
	case "clear":
		c.state.ClearProjectContext()
		return outputCmd(formatter.Dim("Context cleared."))
	case "project":
		c.state.ClearProjectContext()
		return outputCmd(formatter.Dim("Project context cleared."))
	case "item":
		c.state.ClearItemContext()
		return outputCmd(formatter.Dim("Item context cleared."))
	default:
		return outputCmd(shellError(fmt.Errorf("unknown context subcommand: %s", args[0])))
	}
}

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/repository"
)

// Deps holds the repository dependencies needed by the MCP server tools.
type Deps struct {
	Projects  repository.ProjectRepo
	WorkItems repository.WorkItemRepo
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) registerTools() {
	s.tools = []toolDef{
		{
			Name:        "kairos_list_due_items",
			Description: "List Kairos work items that have due dates within the next N days. Returns title, project, due date, status, and estimated minutes. Use this to find tasks to add to a calendar.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"days_ahead": map[string]any{
						"type":        "integer",
						"description": "Number of days ahead to look for due items (default 14, max 365)",
					},
				},
			},
		},
		{
			Name:        "kairos_list_projects",
			Description: "List all active Kairos projects with their names, short IDs, domains, and target dates.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "kairos_get_project_items",
			Description: "Get all active work items for a specific Kairos project, including due dates and time estimates. Use the project short ID (e.g. PHI01, MATH02).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project_short_id": map[string]any{
						"type":        "string",
						"description": "The project short ID (e.g. PHI01, MATH02). Use kairos_list_projects to find short IDs.",
					},
				},
				"required": []string{"project_short_id"},
			},
		},
	}
}

func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "kairos_list_due_items":
		return s.handleListDueItems(ctx, args)
	case "kairos_list_projects":
		return s.handleListProjects(ctx)
	case "kairos_get_project_items":
		return s.handleGetProjectItems(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) handleListDueItems(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		DaysAhead int `json:"days_ahead"`
	}
	params.DaysAhead = 14
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.DaysAhead <= 0 {
		params.DaysAhead = 14
	}

	candidates, err := s.deps.WorkItems.ListSchedulable(ctx, false)
	if err != nil {
		return "", fmt.Errorf("listing work items: %w", err)
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, params.DaysAhead)

	type itemOut struct {
		Seq        int    `json:"seq"`
		Title      string `json:"title"`
		Type       string `json:"type"`
		Project    string `json:"project"`
		ProjectID  string `json:"project_id"`
		DueDate    string `json:"due_date"`
		Status     string `json:"status"`
		PlannedMin int    `json:"planned_min"`
	}

	var items []itemOut
	for _, c := range candidates {
		// Prefer item-level due date, fall back to node due date.
		due := c.WorkItem.DueDate
		if due == nil {
			due = c.NodeDueDate
		}
		if due == nil || due.After(cutoff) {
			continue
		}
		items = append(items, itemOut{
			Seq:        c.WorkItem.Seq,
			Title:      c.WorkItem.Title,
			Type:       c.WorkItem.Type,
			Project:    c.ProjectName,
			ProjectID:  c.ProjectID,
			DueDate:    due.Format("2006-01-02"),
			Status:     string(c.WorkItem.Status),
			PlannedMin: c.WorkItem.PlannedMin,
		})
	}

	if len(items) == 0 {
		return fmt.Sprintf("No items with due dates in the next %d days.", params.DaysAhead), nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleListProjects(ctx context.Context) (string, error) {
	projects, err := s.deps.Projects.List(ctx, false)
	if err != nil {
		return "", fmt.Errorf("listing projects: %w", err)
	}

	type projOut struct {
		ShortID    string  `json:"short_id"`
		Name       string  `json:"name"`
		Domain     string  `json:"domain"`
		Status     string  `json:"status"`
		TargetDate *string `json:"target_date,omitempty"`
	}

	out := make([]projOut, 0, len(projects))
	for _, p := range projects {
		var td *string
		if p.TargetDate != nil {
			formatted := p.TargetDate.Format("2006-01-02")
			td = &formatted
		}
		out = append(out, projOut{
			ShortID:    p.ShortID,
			Name:       p.Name,
			Domain:     p.Domain,
			Status:     string(p.Status),
			TargetDate: td,
		})
	}

	if len(out) == 0 {
		return "No active projects found.", nil
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetProjectItems(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		ProjectShortID string `json:"project_short_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil || strings.TrimSpace(params.ProjectShortID) == "" {
		return "", fmt.Errorf("project_short_id is required")
	}

	proj, err := s.deps.Projects.GetByShortID(ctx, params.ProjectShortID)
	if err != nil {
		return "", fmt.Errorf("project %q not found: %w", params.ProjectShortID, err)
	}

	candidates, err := s.deps.WorkItems.ListSchedulable(ctx, false)
	if err != nil {
		return "", fmt.Errorf("listing work items: %w", err)
	}

	type itemOut struct {
		Seq        int     `json:"seq"`
		Title      string  `json:"title"`
		Type       string  `json:"type"`
		Node       string  `json:"node,omitempty"`
		Status     string  `json:"status"`
		PlannedMin int     `json:"planned_min"`
		DueDate    *string `json:"due_date,omitempty"`
	}

	var items []itemOut
	for _, c := range candidates {
		if c.ProjectID != proj.ID {
			continue
		}
		var dd *string
		if c.WorkItem.DueDate != nil {
			formatted := c.WorkItem.DueDate.Format("2006-01-02")
			dd = &formatted
		} else if c.NodeDueDate != nil {
			formatted := c.NodeDueDate.Format("2006-01-02")
			dd = &formatted
		}
		items = append(items, itemOut{
			Seq:        c.WorkItem.Seq,
			Title:      c.WorkItem.Title,
			Type:       c.WorkItem.Type,
			Node:       c.NodeTitle,
			Status:     string(c.WorkItem.Status),
			PlannedMin: c.WorkItem.PlannedMin,
			DueDate:    dd,
		})
	}

	if len(items) == 0 {
		return fmt.Sprintf("No active work items found for project %s.", params.ProjectShortID), nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

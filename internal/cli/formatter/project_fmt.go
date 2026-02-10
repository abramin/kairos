package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// ProjectInspectData holds all data needed to render a project inspect view.
type ProjectInspectData struct {
	Project   *domain.Project
	RootNodes []*domain.PlanNode
	ChildMap  map[string][]*domain.PlanNode  // parentID -> children
	WorkItems map[string][]*domain.WorkItem  // nodeID -> work items
}

// FormatProjectList renders a styled project list inside a bordered box.
func FormatProjectList(projects []*domain.Project) string {
	headers := []string{"ID", "NAME", "DOMAIN", "STATUS", "DUE"}
	rows := make([][]string, 0, len(projects))

	for _, p := range projects {
		id := p.ShortID
		if strings.TrimSpace(id) == "" {
			id = TruncID(p.ID)
		}
		if strings.TrimSpace(id) == "" {
			id = "--"
		}

		dueStr := Dim("--")
		if p.TargetDate != nil {
			dueStr = RelativeDateStyled(*p.TargetDate)
		}

		rows = append(rows, []string{
			id,
			Bold(p.Name),
			DomainBadge(p.Domain),
			StatusPill(p.Status),
			dueStr,
		})
	}

	table := RenderTable(headers, rows)
	return RenderBox("Projects", table)
}

// FormatProjectInspect renders a styled project inspect card with side-by-side layout.
func FormatProjectInspect(data ProjectInspectData) string {
	// Build left panel (metadata)
	leftPanel := buildMetadataPanel(data.Project)

	// Build right panel (tree)
	rightPanel := buildTreePanel(data.RootNodes, data.ChildMap, data.WorkItems)

	// Join panels horizontally with spacing
	spacing := "    "
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, spacing, rightPanel)

	return RenderBox("", combined)
}

// buildMetadataPanel creates the left panel with project metadata.
func buildMetadataPanel(p *domain.Project) string {
	var b strings.Builder

	// Title + Domain Badge
	b.WriteString(StyleBold.Render(p.Name) + "\n")
	b.WriteString(DomainBadge(p.Domain) + "\n\n")

	// Metadata fields
	b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("STATUS"), StatusPill(p.Status)))
	b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("ID    "), Dim(p.ShortID)))
	b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("UUID  "), TruncID(p.ID)))
	b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("START "), StyleFg.Render(HumanDate(p.StartDate))))

	if p.TargetDate != nil {
		dueRelative := RelativeDateStyled(*p.TargetDate)
		dueAbsolute := p.TargetDate.Format("Jan 2, 2006")
		b.WriteString(fmt.Sprintf("%s  %s %s\n", StyleDim.Render("DUE   "), dueRelative, Dim("("+dueAbsolute+")")))
	}

	if p.ArchivedAt != nil {
		b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("ARCHVD"), HumanTimestamp(*p.ArchivedAt)))
	}

	b.WriteString(fmt.Sprintf("%s  %s\n", StyleDim.Render("UPDATED"), HumanTimestamp(p.UpdatedAt)))

	// Constrain to fixed width for consistent left panel
	panel := lipgloss.NewStyle().Width(45).Render(b.String())
	return panel
}

// buildTreePanel creates the right panel with the plan tree.
func buildTreePanel(rootNodes []*domain.PlanNode, childMap map[string][]*domain.PlanNode, workItems map[string][]*domain.WorkItem) string {
	if len(rootNodes) == 0 {
		return StyleDim.Render("No plan nodes")
	}

	var b strings.Builder

	// Compute progress from work item statuses.
	totalCount := 0
	doneCount := 0
	for _, items := range workItems {
		for _, wi := range items {
			totalCount++
			if wi.Status == domain.WorkItemDone {
				doneCount++
			}
		}
	}

	// Render header with optional progress bar on the same line.
	headerText := StyleHeader.Render("PLAN")
	if totalCount > 0 {
		pct := float64(doneCount) / float64(totalCount)
		headerText += "  " + RenderProgress(pct, 12)
	}
	underline := StyleDim.Render(strings.Repeat("─", 4))
	b.WriteString(headerText + "\n" + underline + "\n")

	items := buildProjectTree(rootNodes, childMap, workItems, 0)
	if len(items) > 0 {
		b.WriteString(RenderTree(items))
	}

	return b.String()
}

// buildProjectTree recursively converts nodes and work items into TreeItems.
func buildProjectTree(
	nodes []*domain.PlanNode,
	childMap map[string][]*domain.PlanNode,
	workItems map[string][]*domain.WorkItem,
	level int,
) []TreeItem {
	var items []TreeItem

	// Sort nodes by OrderIndex for deterministic output.
	sorted := make([]*domain.PlanNode, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OrderIndex < sorted[j].OrderIndex
	})

	for i, node := range sorted {
		children := childMap[node.ID]
		nodeWorkItems := workItems[node.ID]
		isLastNode := i == len(sorted)-1

		// Collapse: single work item + no child nodes → merge into one line.
		if len(nodeWorkItems) == 1 && len(children) == 0 {
			wi := nodeWorkItems[0]
			detail := ""
			if wi.PlannedMin > 0 {
				detail = FormatMinutes(wi.PlannedMin)
			} else if node.DueDate != nil {
				detail = "DUE " + RelativeDate(*node.DueDate)
			} else if node.PlannedMinBudget != nil {
				detail = FormatMinutes(*node.PlannedMinBudget)
			}

			items = append(items, TreeItem{
				Title:  node.Title,
				Seq:    node.Seq,
				Level:  level + 1,
				IsLast: isLastNode,
				Status: string(wi.Status),
				Detail: detail,
			})
			continue
		}

		hasChildren := len(children) > 0 || len(nodeWorkItems) > 0

		// Build detail badge
		detail := ""
		if node.DueDate != nil {
			detail = "DUE " + RelativeDate(*node.DueDate)
		} else if node.PlannedMinBudget != nil {
			detail = FormatMinutes(*node.PlannedMinBudget)
		}

		items = append(items, TreeItem{
			Title:  node.Title,
			Seq:    node.Seq,
			Level:  level + 1,
			IsLast: isLastNode && !hasChildren,
			Detail: detail,
		})

		// Recurse into child nodes
		if len(children) > 0 {
			childItems := buildProjectTree(children, childMap, workItems, level+1)
			items = append(items, childItems...)
		}

		// Add work items under this node
		for j, wi := range nodeWorkItems {
			wiDetail := ""
			if wi.PlannedMin > 0 {
				wiDetail = FormatMinutes(wi.PlannedMin)
			}

			items = append(items, TreeItem{
				Title:  wi.Title,
				Seq:    wi.Seq,
				Level:  level + 2,
				IsLast: j == len(nodeWorkItems)-1,
				Status: string(wi.Status),
				Detail: wiDetail,
			})
		}
	}

	return items
}

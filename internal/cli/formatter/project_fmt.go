package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
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
		dueStr := Dim("--")
		if p.TargetDate != nil {
			dueStr = RelativeDateStyled(*p.TargetDate)
		}

		rows = append(rows, []string{
			Dim(p.ShortID),
			Bold(p.Name),
			DomainBadge(p.Domain),
			StatusPill(p.Status),
			dueStr,
		})
	}

	table := RenderTable(headers, rows)
	return RenderBox("Projects", table)
}

// FormatProjectInspect renders a styled project inspect card with tree view.
func FormatProjectInspect(data ProjectInspectData) string {
	p := data.Project
	var b strings.Builder

	// Title line: Project Name + Domain Badge
	titleLine := fmt.Sprintf("%s  %s", StyleBold.Render(p.Name), DomainBadge(p.Domain))
	b.WriteString(titleLine + "\n\n")

	// Metadata section
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("STATUS"), StatusPill(p.Status)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("ID    "), Dim(p.ShortID)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("UUID  "), TruncID(p.ID)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("START "), StyleFg.Render(HumanDate(p.StartDate))))

	if p.TargetDate != nil {
		dueRelative := RelativeDateStyled(*p.TargetDate)
		dueAbsolute := p.TargetDate.Format("Jan 2, 2006")
		b.WriteString(fmt.Sprintf("  %s  %s %s\n", StyleDim.Render("DUE   "), dueRelative, Dim("("+dueAbsolute+")")))
	}

	if p.ArchivedAt != nil {
		b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("ARCHVD"), HumanTimestamp(*p.ArchivedAt)))
	}

	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("UPDATED"), HumanTimestamp(p.UpdatedAt)))

	// Tree section: nodes and work items
	if len(data.RootNodes) > 0 {
		b.WriteString("\n")
		b.WriteString(Header("Plan"))
		b.WriteString("\n")

		items := buildProjectTree(data.RootNodes, data.ChildMap, data.WorkItems, 0)
		if len(items) > 0 {
			b.WriteString(RenderTree(items))
		}
	}

	return RenderBox("", b.String())
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
		// Determine if this is the last sibling at this level.
		children := childMap[node.ID]
		nodeWorkItems := workItems[node.ID]
		hasChildren := len(children) > 0 || len(nodeWorkItems) > 0

		// Check if this is the last node AND it has no siblings after it
		isLastNode := i == len(sorted)-1

		// Build detail badge
		detail := ""
		if node.DueDate != nil {
			detail = "DUE " + RelativeDate(*node.DueDate)
		} else if node.PlannedMinBudget != nil {
			detail = FormatMinutes(*node.PlannedMinBudget)
		}

		items = append(items, TreeItem{
			Title:  node.Title,
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

			isLastWI := j == len(nodeWorkItems)-1 && isLastNode && len(children) == 0

			items = append(items, TreeItem{
				Title:  wi.Title,
				Level:  level + 2,
				IsLast: j == len(nodeWorkItems)-1,
				Status: string(wi.Status),
				Detail: wiDetail,
			})
			_ = isLastWI // used for potential future tree connector logic
		}
	}

	return items
}

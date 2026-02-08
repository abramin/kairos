package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/service"
)

// FormatDraftTurn renders the LLM's conversational message and a compact
// summary of the current draft state.
func FormatDraftTurn(conv *intelligence.DraftConversation) string {
	var b strings.Builder

	// Show the LLM's message.
	b.WriteString("\n" + StyleFg.Render(conv.LLMMessage) + "\n")

	// Show compact draft summary if a draft exists.
	if conv.Draft != nil && conv.Draft.Project.Name != "" {
		summary := draftSummaryLine(conv.Draft)
		b.WriteString(Dim(summary) + "\n")
	}

	return b.String()
}

// FormatDraftPreview renders the current ImportSchema draft as a tree view.
func FormatDraftPreview(conv *intelligence.DraftConversation) string {
	if conv.Draft == nil {
		return RenderBox("DRAFT PREVIEW", Dim("No draft yet."))
	}

	return RenderBox("DRAFT PREVIEW", renderDraftBody(conv.Draft))
}

// FormatDraftReview renders the complete draft for review when status is "ready".
func FormatDraftReview(conv *intelligence.DraftConversation) string {
	if conv.Draft == nil {
		return RenderBox("DRAFT REVIEW", Dim("No draft yet."))
	}

	var b strings.Builder
	b.WriteString(renderDraftBody(conv.Draft))
	b.WriteString("\n")
	b.WriteString(StyleGreen.Render("Draft is ready for review."))

	return RenderBox("DRAFT REVIEW", b.String())
}

// FormatDraftValidationErrors renders validation errors in a styled list.
func FormatDraftValidationErrors(errs []error) string {
	var b strings.Builder
	b.WriteString(StyleRed.Render(fmt.Sprintf("Validation failed (%d errors):", len(errs))))
	b.WriteString("\n")
	for _, e := range errs {
		b.WriteString(StyleRed.Render("  - ") + e.Error() + "\n")
	}
	return b.String()
}

// FormatDraftAccepted renders the success message after import.
func FormatDraftAccepted(result *service.ImportResult) string {
	var b strings.Builder

	name := result.Project.Name
	shortID := result.Project.ShortID

	b.WriteString(StyleGreen.Render("Project created successfully!") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s  %s [%s]\n", StyleDim.Render("PROJECT"), Bold(name), shortID))
	b.WriteString(fmt.Sprintf("  %s  %d nodes, %d work items, %d dependencies\n",
		StyleDim.Render("CONTENT"),
		result.NodeCount, result.WorkItemCount, result.DependencyCount))

	return RenderBox("", b.String())
}

func renderDraftBody(schema *importer.ImportSchema) string {
	var b strings.Builder
	p := schema.Project

	// Title line.
	titleLine := Bold(p.Name)
	if p.Domain != "" {
		titleLine += "  " + DomainBadge(p.Domain)
	}
	b.WriteString(titleLine + "\n\n")

	// Metadata.
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("ID   "), p.ShortID))
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("START"), p.StartDate))
	if p.TargetDate != nil {
		b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("DUE  "), *p.TargetDate))
	}

	// Defaults summary.
	if schema.Defaults != nil && schema.Defaults.SessionPolicy != nil {
		sp := schema.Defaults.SessionPolicy
		parts := []string{}
		if sp.MinSessionMin != nil {
			parts = append(parts, fmt.Sprintf("min %dm", *sp.MinSessionMin))
		}
		if sp.MaxSessionMin != nil {
			parts = append(parts, fmt.Sprintf("max %dm", *sp.MaxSessionMin))
		}
		if sp.DefaultSessionMin != nil {
			parts = append(parts, fmt.Sprintf("default %dm", *sp.DefaultSessionMin))
		}
		if len(parts) > 0 {
			b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("SESS "), Dim(strings.Join(parts, ", "))))
		}
	}

	// Tree of nodes and work items.
	if len(schema.Nodes) > 0 {
		b.WriteString("\n")
		b.WriteString(Header("Plan"))
		b.WriteString("\n")

		items := buildDraftTree(schema.Nodes, schema.WorkItems)
		if len(items) > 0 {
			b.WriteString(RenderTree(items))
		}
	}

	// Summary counts.
	b.WriteString("\n")
	b.WriteString(Dim(draftSummaryLine(schema)))

	return b.String()
}

func draftSummaryLine(schema *importer.ImportSchema) string {
	parts := []string{}
	if n := len(schema.Nodes); n > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes", n))
	}
	if n := len(schema.WorkItems); n > 0 {
		parts = append(parts, fmt.Sprintf("%d work items", n))
	}
	if n := len(schema.Dependencies); n > 0 {
		parts = append(parts, fmt.Sprintf("%d dependencies", n))
	}

	totalMin := 0
	for _, wi := range schema.WorkItems {
		if wi.PlannedMin != nil {
			totalMin += *wi.PlannedMin
		}
	}
	if totalMin > 0 {
		parts = append(parts, fmt.Sprintf("%s total", FormatMinutes(totalMin)))
	}

	if len(parts) == 0 {
		return "Draft: " + schema.Project.Name
	}
	return "Draft: " + strings.Join(parts, "  •  ")
}

func buildDraftTree(nodes []importer.NodeImport, workItems []importer.WorkItemImport) []TreeItem {
	// Build parent→children map and identify roots.
	childMap := map[string][]importer.NodeImport{}
	rootNodes := []importer.NodeImport{}
	for _, n := range nodes {
		if n.ParentRef != nil {
			childMap[*n.ParentRef] = append(childMap[*n.ParentRef], n)
		} else {
			rootNodes = append(rootNodes, n)
		}
	}

	// Build node_ref → work items map.
	wiMap := map[string][]importer.WorkItemImport{}
	for _, wi := range workItems {
		wiMap[wi.NodeRef] = append(wiMap[wi.NodeRef], wi)
	}

	return buildDraftTreeRecursive(rootNodes, childMap, wiMap, 0)
}

func buildDraftTreeRecursive(
	nodes []importer.NodeImport,
	childMap map[string][]importer.NodeImport,
	wiMap map[string][]importer.WorkItemImport,
	level int,
) []TreeItem {
	var items []TreeItem

	// Sort by order.
	sorted := make([]importer.NodeImport, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	for i, node := range sorted {
		children := childMap[node.Ref]
		nodeWIs := wiMap[node.Ref]
		isLastNode := i == len(sorted)-1

		detail := ""
		if node.PlannedMinBudget != nil {
			detail = FormatMinutes(*node.PlannedMinBudget)
		} else if node.DueDate != nil {
			detail = "DUE " + *node.DueDate
		}

		items = append(items, TreeItem{
			Title:  node.Title,
			Level:  level + 1,
			IsLast: isLastNode && len(children) == 0 && len(nodeWIs) == 0,
			Detail: detail,
		})

		// Recurse into child nodes.
		if len(children) > 0 {
			childItems := buildDraftTreeRecursive(children, childMap, wiMap, level+1)
			items = append(items, childItems...)
		}

		// Add work items under this node.
		for j, wi := range nodeWIs {
			wiDetail := ""
			if wi.PlannedMin != nil {
				wiDetail = FormatMinutes(*wi.PlannedMin)
			}

			items = append(items, TreeItem{
				Title:  wi.Title,
				Level:  level + 2,
				IsLast: j == len(nodeWIs)-1,
				Detail: wiDetail,
			})
		}
	}

	return items
}

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// taskRow represents a flattened row in the task tree.
type taskRow struct {
	isNode    bool
	nodeID    string
	itemID    string
	title     string
	seq       int
	status    domain.WorkItemStatus
	kind      domain.NodeKind
	isDefault bool
	planned   int
	logged    int
	dueDate   *string
	depth     int
}

// taskListLoadedMsg signals that task tree data has been loaded.
type taskListLoadedMsg struct {
	rows []taskRow
	err  error
}

// taskListView shows a project's plan tree with navigable nodes and items.
type taskListView struct {
	state   *SharedState
	rows    []taskRow
	cursor  int
	loading bool
	err     error
}

func newTaskListView(state *SharedState) *taskListView {
	return &taskListView{
		state:   state,
		loading: true,
	}
}

func (v *taskListView) ID() ViewID { return ViewTaskList }
func (v *taskListView) Title() string {
	if v.state.ActiveProjectName != "" {
		return v.state.ActiveProjectName
	}
	return "Tasks"
}

func (v *taskListView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle done")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "log")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *taskListView) Init() tea.Cmd {
	return v.loadTasks()
}

func (v *taskListView) loadTasks() tea.Cmd {
	app := v.state.App
	projectID := v.state.ActiveProjectID
	return func() tea.Msg {
		ctx := context.Background()
		rows, err := buildTaskRows(ctx, app, projectID)
		return taskListLoadedMsg{rows: rows, err: err}
	}
}

func (v *taskListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskListLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.rows = msg.rows
		return v, nil

	case tea.KeyMsg:
		visible := v.visibleRows()
		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(visible)-1 {
				v.cursor++
			}
		case "space":
			// Toggle done/todo for work items
			if v.cursor < len(visible) {
				row := visible[v.cursor]
				if !row.isNode && row.itemID != "" {
					return v, v.toggleDone(row)
				}
			}
		case "r":
			v.loading = true
			return v, v.loadTasks()
		}
	}
	return v, nil
}

func (v *taskListView) toggleDone(row taskRow) tea.Cmd {
	app := v.state.App
	return func() tea.Msg {
		ctx := context.Background()
		item, err := app.WorkItems.GetByID(ctx, row.itemID)
		if err != nil {
			return taskListLoadedMsg{err: err}
		}
		if item.Status == domain.WorkItemDone {
			item.Status = domain.WorkItemTodo
		} else {
			item.Status = domain.WorkItemDone
		}
		if err := app.WorkItems.Update(ctx, item); err != nil {
			return taskListLoadedMsg{err: err}
		}
		// Reload the task list
		rows, err := buildTaskRows(ctx, app, v.state.ActiveProjectID)
		return taskListLoadedMsg{rows: rows, err: err}
	}
}

func (v *taskListView) visibleRows() []taskRow {
	// Filter out default nodes — show their items at depth 0
	var visible []taskRow
	for _, r := range v.rows {
		if r.isNode && r.isDefault {
			continue
		}
		visible = append(visible, r)
	}
	return visible
}

const (
	twoColMinWidth = 40 // minimum usable width per column
	twoColGap      = 4  // spaces between columns
)

func (v *taskListView) View() string {
	if v.loading {
		return "\n  " + formatter.Dim("Loading tasks...")
	}
	if v.err != nil {
		return "\n  " + formatter.StyleRed.Render("Error: "+v.err.Error())
	}

	visible := v.visibleRows()
	if len(visible) == 0 {
		return "\n  " + formatter.Dim("No tasks in this project.")
	}

	groups := groupNodeRows(visible)
	threshold := twoColMinWidth*2 + twoColGap
	useTwoCol := v.state.Width >= threshold && len(groups) >= 2 && len(visible) > v.state.ContentHeight()

	if !useTwoCol {
		return v.renderSingleColumn(visible)
	}

	colWidth := (v.state.Width - twoColGap) / 2
	splitAt := splitGroups(groups)
	leftGroups := groups[:splitAt]
	rightGroups := groups[splitAt:]

	leftLines := v.renderGroupLines(leftGroups, colWidth)
	rightLines := v.renderGroupLines(rightGroups, colWidth)

	// Pad the shorter column so JoinHorizontal aligns correctly.
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	for len(leftLines) < maxLines {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < maxLines {
		rightLines = append(rightLines, "")
	}

	leftCol := lipgloss.NewStyle().Width(colWidth).Render(strings.Join(leftLines, "\n"))
	rightCol := lipgloss.NewStyle().Width(colWidth).Render(strings.Join(rightLines, "\n"))
	gap := strings.Repeat(" ", twoColGap)

	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftCol, gap, rightCol)
}

// renderSingleColumn is the original single-column layout.
func (v *taskListView) renderSingleColumn(visible []taskRow) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, row := range visible {
		b.WriteString(v.renderRow(row, i == v.cursor, 0))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderRow renders a single taskRow. If colWidth > 0, the output is truncated.
func (v *taskListView) renderRow(row taskRow, isCursor bool, colWidth int) string {
	cursor := "  "
	if isCursor {
		cursor = formatter.StyleGreen.Render("▸ ")
	}

	indent := strings.Repeat("  ", row.depth)
	var line string

	if row.isNode {
		line = fmt.Sprintf("%s%s%s",
			cursor, indent,
			formatter.StyleBold.Render(row.title)+" "+formatter.Dim(string(row.kind)),
		)
	} else {
		statusIcon := " "
		switch row.status {
		case domain.WorkItemDone:
			statusIcon = formatter.StyleGreen.Render("✓")
		case domain.WorkItemInProgress:
			statusIcon = formatter.StyleYellow.Render("▶")
		case domain.WorkItemSkipped:
			statusIcon = formatter.Dim("—")
		}

		progress := ""
		if row.planned > 0 {
			pct := float64(row.logged) / float64(row.planned)
			progress = " " + formatter.RenderProgress(pct, 6)
		}

		seqStr := ""
		if row.seq > 0 {
			seqStr = formatter.Dim(fmt.Sprintf("#%d ", row.seq))
		}

		line = fmt.Sprintf("%s%s%s %s%s%s",
			cursor, indent, statusIcon, seqStr, row.title, progress,
		)
	}

	if colWidth > 0 {
		line = lipgloss.NewStyle().MaxWidth(colWidth).Render(line)
	}
	return line
}

// renderGroupLines renders a slice of node groups into individual lines.
func (v *taskListView) renderGroupLines(groups []nodeGroup, colWidth int) []string {
	var lines []string
	for _, g := range groups {
		for i, row := range g.rows {
			globalIdx := g.startIdx + i
			lines = append(lines, v.renderRow(row, globalIdx == v.cursor, colWidth))
		}
	}
	return lines
}

// ── two-column helpers ──────────────────────────────────────────────────────

// nodeGroup is a contiguous slice of visible rows that belong together
// (a node header plus its work items).
type nodeGroup struct {
	startIdx int
	rows     []taskRow
}

// groupNodeRows segments visible rows into node groups.
// A new group starts at each node row, or when a work item's nodeID
// differs from the current group.
func groupNodeRows(visible []taskRow) []nodeGroup {
	if len(visible) == 0 {
		return nil
	}
	var groups []nodeGroup
	cur := nodeGroup{startIdx: 0}
	curNodeID := visible[0].nodeID

	for i, row := range visible {
		startNew := false
		if row.isNode {
			startNew = i > 0
		} else if row.nodeID != curNodeID {
			startNew = true
		}
		if startNew {
			groups = append(groups, cur)
			cur = nodeGroup{startIdx: i}
			curNodeID = row.nodeID
		}
		cur.rows = append(cur.rows, row)
		if row.isNode {
			curNodeID = row.nodeID
		}
	}
	groups = append(groups, cur)
	return groups
}

// splitGroups finds the group boundary index that best balances line counts
// between left and right columns.
func splitGroups(groups []nodeGroup) int {
	totalLines := 0
	for _, g := range groups {
		totalLines += len(g.rows)
	}

	half := totalLines / 2
	leftLines := 0
	bestSplit := 1
	bestDiff := totalLines

	for i, g := range groups {
		leftLines += len(g.rows)
		rightLines := totalLines - leftLines
		diff := leftLines - rightLines
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestSplit = i + 1
		}
		if leftLines >= half {
			break
		}
	}
	return bestSplit
}

// buildTaskRows constructs a flattened tree of task rows for a project.
func buildTaskRows(ctx context.Context, app *App, projectID string) ([]taskRow, error) {
	rootNodes, err := app.Nodes.ListRoots(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing root nodes: %w", err)
	}

	var rows []taskRow
	var walk func(nodes []*domain.PlanNode, depth int) error
	walk = func(nodes []*domain.PlanNode, depth int) error {
		for _, n := range nodes {
			rows = append(rows, taskRow{
				isNode:    true,
				nodeID:    n.ID,
				title:     n.Title,
				kind:      n.Kind,
				isDefault: n.IsDefault,
				depth:     depth,
			})

			// Work items under this node
			items, err := app.WorkItems.ListByNode(ctx, n.ID)
			if err != nil {
				return err
			}
			itemDepth := depth + 1
			if n.IsDefault {
				itemDepth = depth // items of default nodes appear at node's depth
			}
			for _, item := range items {
				var dueStr *string
				if item.DueDate != nil {
					s := formatter.RelativeDate(*item.DueDate)
					dueStr = &s
				}
				rows = append(rows, taskRow{
					isNode:  false,
					nodeID:  n.ID,
					itemID:  item.ID,
					title:   item.Title,
					seq:     item.Seq,
					status:  item.Status,
					planned: item.PlannedMin,
					logged:  item.LoggedMin,
					dueDate: dueStr,
					depth:   itemDepth,
				})
			}

			// Recurse into child nodes
			children, err := app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				return err
			}
			if err := walk(children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(rootNodes, 0); err != nil {
		return nil, err
	}
	return rows, nil
}

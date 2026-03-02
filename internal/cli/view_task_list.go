package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	// Collapse state (set at render time for node rows).
	collapsed  bool
	childCount int
}

// taskListLoadedMsg signals that task tree data has been loaded.
type taskListLoadedMsg struct {
	rows []taskRow
	err  error
}

// jumpTimeoutMsg clears the digit-jump buffer after a pause.
type jumpTimeoutMsg struct{ seq int }

// taskListView shows a project's plan tree with navigable nodes and items.
type taskListView struct {
	state          *SharedState
	rows           []taskRow
	cursor         int
	scrollTop      int
	loading        bool
	err            error
	collapsedNodes map[string]bool // nodeID -> collapsed
	jumpBuf        string          // accumulated digit keys for jump-to-seq
	jumpSeq        int             // incremented per digit press; stale timeouts are ignored
}

func newTaskListView(state *SharedState) *taskListView {
	return &taskListView{
		state:          state,
		loading:        true,
		collapsedNodes: make(map[string]bool),
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
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/collapse")),
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle done")),
		key.NewBinding(key.WithKeys("1"), key.WithHelp("#", "jump to item")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add item")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
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
		v.clampCursor()
		return v, nil

	case refreshViewMsg:
		v.loading = true
		return v, v.loadTasks()

	case jumpTimeoutMsg:
		if msg.seq == v.jumpSeq {
			v.jumpBuf = ""
		}
		return v, nil

	case tea.KeyMsg:
		visible := v.visibleRows()

		// Digit keys: accumulate and jump to matching seq number.
		if k := msg.String(); len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
			v.jumpBuf += k
			v.jumpSeq++
			if target, err := strconv.Atoi(v.jumpBuf); err == nil {
				for i, row := range visible {
					if !row.isNode && row.seq == target {
						v.cursor = i
						break
					}
				}
			}
			seq := v.jumpSeq
			return v, tea.Tick(time.Second, func(time.Time) tea.Msg {
				return jumpTimeoutMsg{seq: seq}
			})
		}

		// Any non-digit key clears the jump buffer.
		v.jumpBuf = ""

		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(visible)-1 {
				v.cursor++
			}
		case "enter":
			if v.cursor < len(visible) {
				row := visible[v.cursor]
				if row.isNode {
					v.collapsedNodes[row.nodeID] = !v.collapsedNodes[row.nodeID]
				} else if row.itemID != "" {
					v.state.SetActiveItem(row.itemID, row.title, row.seq)
					return v, pushView(newActionMenuView(v.state, row.itemID, row.title, row.seq))
				}
			}
		case "space":
			// Toggle done/todo for work items
			if v.cursor < len(visible) {
				row := visible[v.cursor]
				if !row.isNode && row.itemID != "" {
					return v, v.toggleDone(row)
				}
			}
		case "a":
			// Add work item: infer nodeID from cursor position.
			if v.cursor < len(visible) {
				nodeID := visible[v.cursor].nodeID
				if nodeID != "" {
					return v, pushView(newAddWorkItemView(v.state, nodeID))
				}
			}
		case "x":
			// Delete: on item row → open action menu (which has delete);
			// on node row → confirm and delete node.
			if v.cursor < len(visible) {
				row := visible[v.cursor]
				if row.isNode {
					return v, v.deleteNode(row)
				} else if row.itemID != "" {
					return v, v.deleteItem(row)
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
		now := time.Now().UTC()
		if item.Status == domain.WorkItemDone {
			if err := item.Reopen(now); err != nil {
				return taskListLoadedMsg{err: err}
			}
		} else {
			if err := item.MarkDone(now); err != nil {
				return taskListLoadedMsg{err: err}
			}
		}
		if err := app.WorkItems.Update(ctx, item); err != nil {
			return taskListLoadedMsg{err: err}
		}
		// Reload the task list
		rows, err := buildTaskRows(ctx, app, v.state.ActiveProjectID)
		return taskListLoadedMsg{rows: rows, err: err}
	}
}

func (v *taskListView) deleteItem(row taskRow) tea.Cmd {
	state := v.state
	itemID, title := row.itemID, row.title
	var confirmed bool
	form := wizardConfirm(fmt.Sprintf("Delete %q?", title), &confirmed)
	return pushView(newWizardView(state, "Confirm Delete", form, func() tea.Cmd {
		if !confirmed {
			return nil
		}
		return func() tea.Msg {
			if err := state.App.WorkItems.Delete(context.Background(), itemID); err != nil {
				return formErrorOutput(err)
			}
			if state.ActiveItemID == itemID {
				state.ClearItemContext()
			}
			return nil
		}
	}))
}

func (v *taskListView) deleteNode(row taskRow) tea.Cmd {
	state := v.state
	title, nodeID := row.title, row.nodeID
	prompt := fmt.Sprintf("Delete %q", title)
	if row.childCount > 0 {
		prompt += fmt.Sprintf(" and %d item(s)", row.childCount)
	}
	prompt += "?"
	var confirmed bool
	form := wizardConfirm(prompt, &confirmed)
	return pushView(newWizardView(state, "Confirm Delete", form, func() tea.Cmd {
		if !confirmed {
			return nil
		}
		return func() tea.Msg {
			if err := state.App.Nodes.Delete(context.Background(), nodeID); err != nil {
				return formErrorOutput(err)
			}
			return nil
		}
	}))
}

func (v *taskListView) visibleRows() []taskRow {
	var visible []taskRow
	// Track collapsed ancestor depth for recursive hiding.
	collapsedDepth := -1
	for _, r := range v.rows {
		// Skip default nodes (their items appear at parent depth).
		if r.isNode && r.isDefault {
			continue
		}
		// If we are inside a collapsed subtree, skip until depth goes back up.
		if collapsedDepth >= 0 {
			if r.depth > collapsedDepth {
				continue
			}
			collapsedDepth = -1
		}
		// Skip work items belonging to a collapsed node.
		if !r.isNode && v.collapsedNodes[r.nodeID] {
			continue
		}
		// Copy collapse state onto node rows for rendering.
		if r.isNode {
			r.collapsed = v.collapsedNodes[r.nodeID]
			if r.collapsed {
				collapsedDepth = r.depth
			}
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
		v.cursor = 0
		v.scrollTop = 0
		return "\n  " + formatter.Dim("No tasks in this project.")
	}
	v.clampCursor()

	var jumpHint string
	if v.jumpBuf != "" {
		jumpHint = "  " + formatter.Dim("jump: #"+v.jumpBuf)
	}

	groups := groupNodeRows(visible)
	threshold := twoColMinWidth*2 + twoColGap
	useTwoCol := v.state.Width >= threshold && len(groups) >= 2 && len(visible) > v.state.ContentHeight()
	linesAvail := v.state.ContentHeight()
	if jumpHint != "" {
		linesAvail--
	}
	if linesAvail < 1 {
		linesAvail = 1
	}

	layout := v.renderSingleColumnLines(visible)
	if useTwoCol {
		colWidth := (v.state.Width - twoColGap) / 2
		layout = v.renderTwoColumnLines(visible, groups, colWidth)
	}
	body := v.renderWindow(layout.lines, layout.rowToLine, linesAvail)

	if jumpHint != "" {
		if body == "" {
			return jumpHint
		}
		return jumpHint + "\n" + body
	}
	if body == "" {
		return "\n"
	}
	return "\n" + body
}

func (v *taskListView) clampCursor() {
	visibleCount := len(v.visibleRows())
	if visibleCount == 0 {
		v.cursor = 0
		v.scrollTop = 0
		return
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= visibleCount {
		v.cursor = visibleCount - 1
	}
}

type renderedTaskLayout struct {
	lines     []string
	rowToLine []int
}

func (v *taskListView) renderSingleColumnLines(visible []taskRow) renderedTaskLayout {
	lines := make([]string, 0, len(visible))
	rowToLine := make([]int, len(visible))
	for i, row := range visible {
		lines = append(lines, v.renderRow(row, i == v.cursor, 0))
		rowToLine[i] = i
	}
	return renderedTaskLayout{
		lines:     lines,
		rowToLine: rowToLine,
	}
}

func (v *taskListView) renderWindow(lines []string, rowToLine []int, height int) string {
	if len(lines) == 0 {
		v.scrollTop = 0
		return ""
	}

	cursorLine := v.cursor
	if v.cursor >= 0 && v.cursor < len(rowToLine) && rowToLine[v.cursor] >= 0 {
		cursorLine = rowToLine[v.cursor]
	}

	if cursorLine < v.scrollTop {
		v.scrollTop = cursorLine
	}
	if cursorLine >= v.scrollTop+height {
		v.scrollTop = cursorLine - height + 1
	}

	maxTop := len(lines) - height
	if maxTop < 0 {
		maxTop = 0
	}
	if v.scrollTop > maxTop {
		v.scrollTop = maxTop
	}
	if v.scrollTop < 0 {
		v.scrollTop = 0
	}

	end := v.scrollTop + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[v.scrollTop:end], "\n")
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
		indicator := "▾ "
		if row.collapsed {
			indicator = fmt.Sprintf("▸ (%d) ", row.childCount)
		}
		line = fmt.Sprintf("%s%s%s%s",
			cursor, indent,
			formatter.Dim(indicator),
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
			if (row.status == domain.WorkItemDone || row.status == domain.WorkItemSkipped) && pct < 1.0 {
				pct = 1.0
			}
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

type renderedGroupLine struct {
	text     string
	rowIndex int // -1 means padding / no associated row.
}

// renderGroupLines renders a slice of node groups into individual lines.
func (v *taskListView) renderGroupLines(groups []nodeGroup, colWidth int) []renderedGroupLine {
	var lines []renderedGroupLine
	for _, g := range groups {
		for i, row := range g.rows {
			globalIdx := g.startIdx + i
			lines = append(lines, renderedGroupLine{
				text:     v.renderRow(row, globalIdx == v.cursor, colWidth),
				rowIndex: globalIdx,
			})
		}
	}
	return lines
}

func (v *taskListView) renderTwoColumnLines(visible []taskRow, groups []nodeGroup, colWidth int) renderedTaskLayout {
	splitAt := splitGroups(groups)
	leftGroups := groups[:splitAt]
	rightGroups := groups[splitAt:]
	leftLines := v.renderGroupLines(leftGroups, colWidth)
	rightLines := v.renderGroupLines(rightGroups, colWidth)

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	for len(leftLines) < maxLines {
		leftLines = append(leftLines, renderedGroupLine{text: "", rowIndex: -1})
	}
	for len(rightLines) < maxLines {
		rightLines = append(rightLines, renderedGroupLine{text: "", rowIndex: -1})
	}

	rowToLine := make([]int, len(visible))
	for i := range rowToLine {
		rowToLine[i] = -1
	}
	gap := strings.Repeat(" ", twoColGap)
	leftStyle := lipgloss.NewStyle().Width(colWidth)
	rightStyle := lipgloss.NewStyle().Width(colWidth)

	lines := make([]string, 0, maxLines)
	for i := 0; i < maxLines; i++ {
		lines = append(lines, leftStyle.Render(leftLines[i].text)+gap+rightStyle.Render(rightLines[i].text))
		if leftLines[i].rowIndex >= 0 {
			rowToLine[leftLines[i].rowIndex] = i
		}
		if rightLines[i].rowIndex >= 0 {
			rowToLine[rightLines[i].rowIndex] = i
		}
	}

	return renderedTaskLayout{
		lines:     lines,
		rowToLine: rowToLine,
	}
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
			nodeRowIdx := len(rows)
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
			// Set the child count on the node row.
			rows[nodeRowIdx].childCount = len(items)

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

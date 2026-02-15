package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── data types ───────────────────────────────────────────────────────────────

// dashboardData holds the loaded data for the dashboard view.
type dashboardData struct {
	projects []*domain.Project
	status   *contract.StatusResponse
}

// dashboardDetailData holds per-project detail for the right pane.
type dashboardDetailData struct {
	project    *domain.Project
	statusView *contract.ProjectStatusView
	itemCounts struct{ total, done, inProgress, todo int }
	taskRows   []taskRow // flattened task tree for preview in detail pane
}

// ── messages ─────────────────────────────────────────────────────────────────

// dashboardLoadedMsg signals that dashboard data has been loaded.
type dashboardLoadedMsg struct {
	data dashboardData
	err  error
}

// dashboardDetailLoadedMsg signals that per-project detail has been loaded.
type dashboardDetailLoadedMsg struct {
	data *dashboardDetailData
	err  error
}

// ── view ─────────────────────────────────────────────────────────────────────

// dashboardView is the home screen of the TUI.
// It shows a split-pane layout: left pane (selectable project list)
// and right pane (detail for the selected project).
type dashboardView struct {
	state   *SharedState
	data    *dashboardData
	loading bool
	err     error

	// Project selection
	cursor        int
	cachedActive  []*domain.Project // recomputed on data load
	detailData    *dashboardDetailData
	detailLoading bool
}

func newDashboardView(state *SharedState) *dashboardView {
	return &dashboardView{
		state:   state,
		loading: true,
	}
}

func (v *dashboardView) ID() ViewID    { return ViewDashboard }
func (v *dashboardView) Title() string { return "Dashboard" }

func (v *dashboardView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "what now")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "draft")),
		key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "help")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

func (v *dashboardView) Init() tea.Cmd {
	return v.loadData()
}

// ── data loading ─────────────────────────────────────────────────────────────

func (v *dashboardView) loadData() tea.Cmd {
	app := v.state.App
	return func() tea.Msg {
		ctx := context.Background()

		projects, err := app.Projects.List(ctx, false)
		if err != nil {
			return dashboardLoadedMsg{err: err}
		}

		req := contract.NewStatusRequest()
		status, err := app.Status.GetStatus(ctx, req)
		if err != nil {
			return dashboardLoadedMsg{err: err}
		}

		return dashboardLoadedMsg{
			data: dashboardData{
				projects: projects,
				status:   status,
			},
		}
	}
}

func (v *dashboardView) loadSelectedDetail() tea.Cmd {
	active := v.activeProjects()
	if v.cursor >= len(active) {
		return nil
	}
	projectID := active[v.cursor].ID
	app := v.state.App

	return func() tea.Msg {
		ctx := context.Background()

		project, err := app.Projects.GetByID(ctx, projectID)
		if err != nil {
			return dashboardDetailLoadedMsg{err: err}
		}

		// Project-scoped status
		req := contract.NewStatusRequest()
		req.ProjectScope = []string{projectID}
		statusResp, err := app.Status.GetStatus(ctx, req)
		if err != nil {
			return dashboardDetailLoadedMsg{err: err}
		}

		var sv *contract.ProjectStatusView
		if len(statusResp.Projects) > 0 {
			sv = &statusResp.Projects[0]
		}

		// Item counts
		items, _ := app.WorkItems.ListByProject(ctx, projectID)
		var counts struct{ total, done, inProgress, todo int }
		counts.total = len(items)
		for _, item := range items {
			switch item.Status {
			case domain.WorkItemDone:
				counts.done++
			case domain.WorkItemInProgress:
				counts.inProgress++
			case domain.WorkItemTodo:
				counts.todo++
			}
		}

		// Build flattened task tree for the detail pane preview.
		taskRows, _ := buildTaskRows(ctx, app, projectID)

		return dashboardDetailLoadedMsg{
			data: &dashboardDetailData{
				project:    project,
				statusView: sv,
				itemCounts: counts,
				taskRows:   taskRows,
			},
		}
	}
}

// activeProjects returns the cached list of active projects.
// Recomputed when data is loaded (dashboardLoadedMsg / refreshViewMsg).
func (v *dashboardView) activeProjects() []*domain.Project {
	return v.cachedActive
}

// recomputeActive filters data.projects to only active projects
// and stores the result in cachedActive.
func (v *dashboardView) recomputeActive() {
	v.cachedActive = nil
	if v.data == nil {
		return
	}
	for _, p := range v.data.projects {
		if p.Status == domain.ProjectActive {
			v.cachedActive = append(v.cachedActive, p)
		}
	}
}

// ── update ───────────────────────────────────────────────────────────────────

func (v *dashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.data = &msg.data
		v.recomputeActive()
		// Clamp cursor and load detail for the first project.
		active := v.activeProjects()
		if v.cursor >= len(active) {
			v.cursor = max(0, len(active)-1)
		}
		if len(active) > 0 {
			v.detailLoading = true
			return v, v.loadSelectedDetail()
		}
		return v, nil

	case dashboardDetailLoadedMsg:
		v.detailLoading = false
		if msg.err != nil {
			v.detailData = nil
			return v, nil
		}
		v.detailData = msg.data
		return v, nil

	case refreshViewMsg:
		v.loading = true
		v.err = nil
		return v, v.loadData()

	case tea.KeyMsg:
		active := v.activeProjects()
		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
				v.detailLoading = true
				return v, v.loadSelectedDetail()
			}
		case "down", "j":
			if v.cursor < len(active)-1 {
				v.cursor++
				v.detailLoading = true
				return v, v.loadSelectedDetail()
			}
		case "enter":
			if v.cursor < len(active) {
				p := active[v.cursor]
				v.state.SetActiveProjectFrom(p)
				v.state.ClearItemContext()
				return v, pushView(newTaskListView(v.state))
			}
		case "p":
			return v, pushView(newProjectListView(v.state))
		case "d":
			return v, pushView(newDraftView(v.state, ""))
		case "h":
			return v, pushView(newHelpChatView(v.state))
		case "r":
			v.loading = true
			v.err = nil
			return v, v.loadData()
		}
	}

	return v, nil
}

// ── view rendering ───────────────────────────────────────────────────────────

const dashLeftPaneWidth = 44

// Column widths for project list rows.
const (
	colIndicatorW = 2  // ▸ (selected) or risk glyph
	colShortIDW   = 8  // e.g. "PHI01" padded
	colNameW      = 15 // truncated with …
	colBarW       = 10 // compact progress blocks
	colPctW       = 5  // e.g. " 40%"
)

func (v *dashboardView) View() string {
	if v.loading {
		return "\n  " + formatter.Dim("Loading...")
	}
	if v.err != nil {
		return "\n  " + formatter.StyleRed.Render("Error: "+v.err.Error())
	}
	if v.data == nil {
		return ""
	}

	var b strings.Builder

	// Mode badge
	if v.data.status != nil {
		b.WriteString("\n  " + formatter.ModeBadge(v.data.status.Summary.GlobalModeIfNow))
		b.WriteString("\n\n")
	}

	active := v.activeProjects()
	if len(active) == 0 {
		b.WriteString("  " + formatter.Dim("No projects yet. Press 'd' to create one."))
		b.WriteString("\n")
		return b.String()
	}

	// Decide layout: split pane vs. single column.
	useSplit := v.state.Width >= 80
	contentHeight := v.state.ContentHeight()
	badgeLines := strings.Count(b.String(), "\n")
	paneHeight := contentHeight - badgeLines
	if paneHeight < 5 {
		paneHeight = 5
	}

	rightWidth := v.state.Width - dashLeftPaneWidth - 3
	if rightWidth < 20 {
		rightWidth = 20
	}

	leftPane := v.renderLeftPane(active)
	rightPane := v.renderRightPane(paneHeight, rightWidth)

	if !useSplit {
		b.WriteString(leftPane)
		b.WriteString("\n")
		b.WriteString(rightPane)
		return b.String()
	}

	leftCol := lipgloss.NewStyle().Width(dashLeftPaneWidth).Height(paneHeight).Render(leftPane)

	// Full-height divider
	divLines := make([]string, paneHeight)
	for i := range divLines {
		divLines[i] = " " + formatter.Dim("│") + " "
	}
	divider := strings.Join(divLines, "\n")

	rightCol := lipgloss.NewStyle().Width(rightWidth).Height(paneHeight).Render(rightPane)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftCol, divider, rightCol))

	return b.String()
}

// ── left pane: selectable project list ───────────────────────────────────────

func (v *dashboardView) renderLeftPane(projects []*domain.Project) string {
	riskMap := make(map[string]domain.RiskLevel)
	progressMap := make(map[string]float64)
	if v.data.status != nil {
		for _, ps := range v.data.status.Projects {
			riskMap[ps.ProjectID] = ps.RiskLevel
			progressMap[ps.ProjectID] = ps.ProgressTimePct / 100.0
		}
	}

	var b strings.Builder
	b.WriteString(formatter.StyleHeader.Render("PROJECTS") + "\n\n")

	for i, p := range projects {
		row := v.renderProjectRow(p, i == v.cursor, riskMap[p.ID], progressMap[p.ID])
		b.WriteString(row + "\n")
	}

	return b.String()
}

// renderProjectRow renders a single fixed-width project row for the sidebar.
func (v *dashboardView) renderProjectRow(
	p *domain.Project, selected bool, risk domain.RiskLevel, progress float64,
) string {
	// Indicator (2 chars): selection marker when selected, risk glyph otherwise.
	var indicator string
	if selected {
		indicator = formatter.StyleGreen.Render("▸ ")
	} else {
		switch risk {
		case domain.RiskOnTrack:
			indicator = formatter.StyleGreen.Render("✔ ")
		case domain.RiskAtRisk:
			indicator = formatter.StyleYellow.Render("○ ")
		case domain.RiskCritical:
			indicator = formatter.StyleRed.Render("▲ ")
		default:
			indicator = formatter.Dim("· ")
		}
	}
	indicatorCol := lipgloss.NewStyle().Width(colIndicatorW).Render(indicator)

	// ShortID (8 chars, always dim).
	shortID := p.DisplayID()
	shortIDCol := lipgloss.NewStyle().Foreground(formatter.ColorDim).Width(colShortIDW).Render(shortID)

	// Name (15 chars, truncated with ellipsis, bold when selected).
	name := p.Name
	if len(name) > colNameW {
		name = name[:colNameW-1] + "…"
	}
	nameStyle := lipgloss.NewStyle().Foreground(formatter.ColorFg).Width(colNameW)
	if selected {
		nameStyle = nameStyle.Bold(true)
	}
	nameCol := nameStyle.Render(name)

	// Compact progress bar (10 chars, dimmed when not selected).
	barCol := formatter.RenderCompactBar(progress, colBarW, !selected)

	// Percentage (5 chars, dimmed when not selected).
	pctVal := progress * 100
	if pctVal > 999 {
		pctVal = 999
	}
	pctStr := fmt.Sprintf("%3.0f%%", pctVal)
	pctStyle := lipgloss.NewStyle().Width(colPctW).Foreground(formatter.ColorDim)
	if selected {
		pctStyle = pctStyle.Foreground(formatter.ColorFg)
	}
	pctCol := pctStyle.Render(pctStr)

	// Assemble row with lipgloss for ANSI-aware alignment.
	row := lipgloss.JoinHorizontal(lipgloss.Left,
		indicatorCol, shortIDCol, nameCol, barCol, pctCol,
	)

	if selected {
		row = lipgloss.NewStyle().Background(formatter.ColorBg2).Width(dashLeftPaneWidth).Render(row)
	}

	return row
}

// ── right pane: project detail ───────────────────────────────────────────────

func (v *dashboardView) renderRightPane(contentHeight, rightWidth int) string {
	if v.detailLoading {
		return formatter.Dim("Loading details...")
	}
	if v.detailData == nil {
		return formatter.Dim("Select a project to see details.")
	}

	d := v.detailData
	var b strings.Builder

	// Project name + status
	b.WriteString(formatter.StyleBold.Render(d.project.Name) + "\n")
	b.WriteString(formatter.StatusPill(d.project.Status) + "\n\n")

	// Progress
	if d.statusView != nil {
		pct := d.statusView.ProgressTimePct / 100.0
		b.WriteString(formatter.Dim("Progress  "))
		b.WriteString(formatter.RenderProgress(pct, 16) + "\n")
		b.WriteString(fmt.Sprintf("          %s logged / %s planned\n\n",
			formatter.Bold(formatter.FormatMinutes(d.statusView.LoggedMinTotal)),
			formatter.Dim(formatter.FormatMinutes(d.statusView.PlannedMinTotal)),
		))

		// Risk
		b.WriteString(formatter.Dim("Risk      "))
		b.WriteString(formatter.RiskIndicator(d.statusView.RiskLevel) + "\n")

		// Due date
		if d.statusView.DueDate != nil {
			b.WriteString(formatter.Dim("Due       "))
			if parsed, err := time.Parse("2006-01-02", *d.statusView.DueDate); err == nil {
				b.WriteString(formatter.RelativeDateStyled(parsed))
			}
			if d.statusView.DaysLeft != nil {
				b.WriteString(formatter.Dim(fmt.Sprintf(" (%dd left)", *d.statusView.DaysLeft)))
			}
			b.WriteString("\n")
		}

		// Pace
		if d.statusView.RequiredDailyMin > 0 {
			b.WriteString(fmt.Sprintf("\n%s %s/day needed\n",
				formatter.Dim("Pace     "),
				formatter.Bold(formatter.FormatMinutes(int(d.statusView.RequiredDailyMin))),
			))
			b.WriteString(fmt.Sprintf("%s %s/day recent\n",
				formatter.Dim("         "),
				formatter.FormatMinutes(int(d.statusView.RecentDailyMin)),
			))
		}
		b.WriteString("\n")
	}

	// Item counts
	b.WriteString(fmt.Sprintf("%s %d total  %s done  %s active  %s todo\n",
		formatter.Dim("Items    "),
		d.itemCounts.total,
		formatter.StyleGreen.Render(fmt.Sprintf("%d", d.itemCounts.done)),
		formatter.StyleYellow.Render(fmt.Sprintf("%d", d.itemCounts.inProgress)),
		formatter.StyleBlue.Render(fmt.Sprintf("%d", d.itemCounts.todo)),
	))

	// Task tree preview
	statsLines := strings.Count(b.String(), "\n")
	availForTasks := contentHeight - statsLines - 4 // header + truncation hint + padding
	if availForTasks > 0 {
		b.WriteString(v.renderTaskPreview(availForTasks, rightWidth))
	}

	return b.String()
}

// ── task tree preview (read-only) ────────────────────────────────────────────

func (v *dashboardView) renderTaskPreview(maxLines, maxWidth int) string {
	if v.detailData == nil || len(v.detailData.taskRows) == 0 {
		return ""
	}

	// Filter out default node rows (same as taskListView.visibleRows).
	var visible []taskRow
	for _, r := range v.detailData.taskRows {
		if r.isNode && r.isDefault {
			continue
		}
		visible = append(visible, r)
	}
	if len(visible) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n" + formatter.StyleHeader.Render("TASKS") + "\n\n")

	truncated := false
	if len(visible) > maxLines {
		visible = visible[:maxLines]
		truncated = true
	}

	for _, row := range visible {
		b.WriteString(v.renderPreviewRow(row, maxWidth))
		b.WriteByte('\n')
	}

	if truncated {
		total := len(v.detailData.taskRows)
		b.WriteString(formatter.Dim(fmt.Sprintf("  ... %d more (enter to view all)", total-maxLines)))
		b.WriteByte('\n')
	}

	return b.String()
}

func (v *dashboardView) renderPreviewRow(row taskRow, maxWidth int) string {
	indent := strings.Repeat("  ", row.depth)

	var line string
	if row.isNode {
		line = fmt.Sprintf("  %s%s %s",
			indent,
			formatter.Dim("▾"),
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

		line = fmt.Sprintf("  %s%s %s%s%s",
			indent, statusIcon, seqStr, row.title, progress,
		)
	}

	if maxWidth > 0 {
		line = lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
	}
	return line
}

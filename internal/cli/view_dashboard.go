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

func (v *dashboardView) ID() ViewID { return ViewDashboard }
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

		return dashboardDetailLoadedMsg{
			data: &dashboardDetailData{
				project:    project,
				statusView: sv,
				itemCounts: counts,
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

const dashLeftPaneWidth = 36

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

	leftPane := v.renderLeftPane(active)
	rightPane := v.renderRightPane()

	if !useSplit {
		b.WriteString(leftPane)
		b.WriteString("\n")
		b.WriteString(rightPane)
		return b.String()
	}

	rightWidth := v.state.Width - dashLeftPaneWidth - 3
	if rightWidth < 20 {
		rightWidth = 20
	}

	leftCol := lipgloss.NewStyle().Width(dashLeftPaneWidth).Render(leftPane)
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color(formatter.ColorDim)).
		Render("│")
	rightCol := lipgloss.NewStyle().Width(rightWidth).Render(rightPane)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftCol, " "+divider+" ", rightCol))

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
		cursor := "  "
		nameStyle := formatter.StyleFg
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
			nameStyle = formatter.StyleBold
		}

		shortID := domain.DisplayID(p.ShortID, p.ID)
		name := p.Name
		if len(name) > 16 {
			name = name[:15] + "…"
		}

		progress := progressMap[p.ID]
		progressBar := formatter.RenderProgress(progress, 6)

		risk := riskMap[p.ID]
		riskDot := formatter.Dim("·")
		switch risk {
		case domain.RiskCritical:
			riskDot = formatter.StyleRed.Render("●")
		case domain.RiskAtRisk:
			riskDot = formatter.StyleYellow.Render("●")
		case domain.RiskOnTrack:
			riskDot = formatter.StyleGreen.Render("●")
		}

		b.WriteString(fmt.Sprintf("%s%-7s %s %s %s\n",
			cursor,
			formatter.StyleGreen.Render(shortID),
			nameStyle.Render(padRight(name, 16)),
			progressBar,
			riskDot,
		))
	}

	return b.String()
}

// ── right pane: project detail ───────────────────────────────────────────────

func (v *dashboardView) renderRightPane() string {
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

	return b.String()
}

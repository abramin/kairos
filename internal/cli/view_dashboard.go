package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// dashboardData holds the loaded data for the dashboard view.
type dashboardData struct {
	projects []*domain.Project
	status   *contract.StatusResponse
}

// dashboardLoadedMsg signals that dashboard data has been loaded.
type dashboardLoadedMsg struct {
	data dashboardData
	err  error
}

// dashboardView is the home screen of the TUI.
type dashboardView struct {
	state   *SharedState
	data    *dashboardData
	loading bool
	err     error
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
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "projects")),
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

func (v *dashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.data = &msg.data
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			return v, pushView(newProjectListView(v.state))
		case "?":
			return v, pushView(newRecommendationView(v.state, 60))
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

	// Summary line
	b.WriteString("\n")
	if v.data.status != nil {
		b.WriteString("  " + v.renderModeBadge(v.data.status.Summary.GlobalModeIfNow))
		b.WriteString("\n\n")
	}

	// Project table
	if len(v.data.projects) == 0 {
		b.WriteString("  " + formatter.Dim("No projects yet. Use 'draft' to create one."))
		b.WriteString("\n")
	} else {
		b.WriteString(v.renderProjectTable())
	}

	return b.String()
}

func (v *dashboardView) renderModeBadge(mode domain.PlanMode) string {
	if mode == domain.ModeCritical {
		return formatter.StyleRed.Render("▲ CRITICAL MODE") +
			formatter.Dim(" — focus on critical work only")
	}
	return formatter.StyleGreen.Render("● BALANCED") +
		formatter.Dim(" — all projects available")
}

func (v *dashboardView) renderProjectTable() string {
	// Build a risk map from status response
	riskMap := make(map[string]domain.RiskLevel)
	progressMap := make(map[string]float64)
	if v.data.status != nil {
		for _, ps := range v.data.status.Projects {
			riskMap[ps.ProjectID] = ps.RiskLevel
			progressMap[ps.ProjectID] = ps.ProgressTimePct / 100.0
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(formatter.ColorHeader))

	var b strings.Builder
	b.WriteString("  " + headerStyle.Render("ACTIVE PROJECTS") + "\n\n")

	// Column headers
	b.WriteString(fmt.Sprintf("  %-8s %-20s %-18s %-12s %s\n",
		formatter.Dim("ID"),
		formatter.Dim("Name"),
		formatter.Dim("Progress"),
		formatter.Dim("Risk"),
		formatter.Dim("Due"),
	))

	for _, p := range v.data.projects {
		if p.Status != domain.ProjectActive {
			continue
		}

		shortID := p.ShortID
		if shortID == "" && len(p.ID) >= 6 {
			shortID = p.ID[:6]
		}

		name := p.Name
		if len(name) > 18 {
			name = name[:17] + "…"
		}

		progress := progressMap[p.ID]
		progressBar := formatter.RenderProgress(progress, 8)

		risk := riskMap[p.ID]
		var riskStr string
		switch risk {
		case domain.RiskCritical:
			riskStr = formatter.StyleRed.Render("● crit")
		case domain.RiskAtRisk:
			riskStr = formatter.StyleYellow.Render("● risk")
		case domain.RiskOnTrack:
			riskStr = formatter.StyleGreen.Render("● ok")
		default:
			riskStr = formatter.Dim("—")
		}

		dueStr := formatter.Dim("—")
		if p.TargetDate != nil {
			dueStr = formatter.RelativeDateStyled(*p.TargetDate)
		}

		b.WriteString(fmt.Sprintf("  %-8s %-20s %s  %-12s %s\n",
			formatter.StyleGreen.Render(shortID),
			name,
			progressBar,
			riskStr,
			dueStr,
		))
	}

	return b.String()
}

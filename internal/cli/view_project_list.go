package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// projectsLoadedMsg signals that project list data has been loaded.
type projectsLoadedMsg struct {
	projects []*domain.Project
	err      error
}

// projectListView shows an interactive, navigable list of projects.
type projectListView struct {
	state    *SharedState
	projects []*domain.Project
	cursor   int
	loading  bool
	err      error

	// Filtering
	filtering bool
	filter    string
}

func newProjectListView(state *SharedState) *projectListView {
	return &projectListView{
		state:   state,
		loading: true,
	}
}

func (v *projectListView) ID() ViewID { return ViewProjectList }
func (v *projectListView) Title() string { return "Projects" }

func (v *projectListView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *projectListView) Init() tea.Cmd {
	return v.loadProjects()
}

func (v *projectListView) loadProjects() tea.Cmd {
	app := v.state.App
	return func() tea.Msg {
		ctx := context.Background()
		projects, err := app.Projects.List(ctx, false)
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (v *projectListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.projects = msg.projects
		return v, nil

	case tea.KeyMsg:
		if v.filtering {
			return v.updateFilter(msg)
		}
		return v.updateNormal(msg)
	}
	return v, nil
}

func (v *projectListView) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := v.visibleProjects()

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
			p := visible[v.cursor]
			v.state.SetActiveProjectFrom(p)
			v.state.ClearItemContext()
			return v, pushView(newTaskListView(v.state))
		}
	case "/":
		v.filtering = true
		v.filter = ""
	}
	return v, nil
}

func (v *projectListView) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		v.filtering = false
		v.filter = ""
		v.cursor = 0
		return v, nil
	case tea.KeyEnter:
		v.filtering = false
		return v, nil
	case tea.KeyBackspace:
		if len(v.filter) > 0 {
			v.filter = v.filter[:len(v.filter)-1]
			v.cursor = 0
		}
	default:
		if len(msg.String()) == 1 {
			v.filter += msg.String()
			v.cursor = 0
		}
	}
	return v, nil
}

func (v *projectListView) visibleProjects() []*domain.Project {
	if v.filter == "" {
		return v.projects
	}
	lf := strings.ToLower(v.filter)
	var filtered []*domain.Project
	for _, p := range v.projects {
		if strings.Contains(strings.ToLower(p.Name), lf) ||
			strings.Contains(strings.ToLower(p.ShortID), lf) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (v *projectListView) View() string {
	if v.loading {
		return "\n  " + formatter.Dim("Loading projects...")
	}
	if v.err != nil {
		return "\n  " + formatter.StyleRed.Render("Error: "+v.err.Error())
	}

	visible := v.visibleProjects()

	var b strings.Builder
	b.WriteString("\n")

	if v.filtering {
		b.WriteString("  " + formatter.StyleYellow.Render("/") + " " + v.filter + "█\n\n")
	}

	if len(visible) == 0 {
		b.WriteString("  " + formatter.Dim("No projects found.") + "\n")
		return b.String()
	}

	for i, p := range visible {
		shortID := p.ShortID
		if shortID == "" && len(p.ID) >= 6 {
			shortID = p.ID[:6]
		}

		cursor := "  "
		nameStyle := formatter.StyleFg
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
			nameStyle = formatter.StyleBold
		}

		status := formatter.StatusPill(p.Status)
		due := formatter.Dim("—")
		if p.TargetDate != nil {
			due = formatter.RelativeDateStyled(*p.TargetDate)
		}

		b.WriteString(fmt.Sprintf("%s%-7s %s  %s  %s\n",
			cursor,
			formatter.StyleGreen.Render(shortID),
			nameStyle.Render(padRight(p.Name, 22)),
			status,
			due,
		))
	}

	return b.String()
}

// padRight pads a string to a minimum width, truncating if needed.
func padRight(s string, width int) string {
	if len(s) > width {
		return s[:width-1] + "…"
	}
	return s + strings.Repeat(" ", width-len(s))
}

package cli

import (
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appModel is the root bubbletea Model for the TUI.
// It manages a view stack and a persistent command bar.
type appModel struct {
	state     *SharedState
	viewStack []View
	cmdBar    commandBar
	quitting  bool

	// Transient output from the command bar, displayed in content area.
	lastOutput string
}

func newAppModel(app *App) appModel {
	state := &SharedState{
		App:   app,
		Cache: newShellProjectCache(),
	}
	cb := newCommandBar(state)

	m := appModel{
		state:  state,
		cmdBar: cb,
	}

	// Start with the dashboard as the home view.
	m.viewStack = []View{newDashboardView(state)}

	return m
}

// activeView returns the top view on the stack, or nil.
func (m *appModel) activeView() View {
	if len(m.viewStack) == 0 {
		return nil
	}
	return m.viewStack[len(m.viewStack)-1]
}

// ── bubbletea interface ──────────────────────────────────────────────────────

func (m appModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if v := m.activeView(); v != nil {
		cmds = append(cmds, v.Init())
	}
	return tea.Batch(cmds...)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height
		m.cmdBar.SetWidth(msg.Width)
		// Forward to active view
		if v := m.activeView(); v != nil {
			updated, cmd := v.Update(msg)
			m.viewStack[len(m.viewStack)-1] = updated.(View)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	// Navigation messages from views or command bar
	case pushViewMsg:
		m.cmdBar.Blur()
		m.lastOutput = ""
		m.viewStack = append(m.viewStack, msg.view)
		return m, msg.view.Init()

	case popViewMsg:
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
		}
		return m, nil

	case replaceViewMsg:
		m.cmdBar.Blur()
		m.lastOutput = ""
		if len(m.viewStack) > 0 {
			m.viewStack[len(m.viewStack)-1] = msg.view
		} else {
			m.viewStack = append(m.viewStack, msg.view)
		}
		return m, msg.view.Init()

	case cmdOutputMsg:
		m.lastOutput = msg.output
		return m, nil

	case wizardCompleteMsg:
		// Atomically pop the wizard view and execute the follow-up command.
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
		}
		m.lastOutput = ""
		m.cmdBar.Focus()
		return m, msg.nextCmd

	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	// Forward other messages to command bar (e.g., cursor blink)
	if m.cmdBar.Focused() {
		cmd := m.cmdBar.UpdateNonKey(msg)
		return m, cmd
	}

	// Forward to active view
	if v := m.activeView(); v != nil {
		updated, cmd := v.Update(msg)
		m.viewStack[len(m.viewStack)-1] = updated.(View)
		return m, cmd
	}

	return m, nil
}

func (m appModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// If command bar is focused, route keys there
	if m.cmdBar.Focused() {
		cmd := m.cmdBar.Update(msg)
		return m, cmd
	}

	// If active view captures input (has its own text input), forward directly.
	// This bypasses global keybindings so views like draft and help chat can
	// receive all characters including 'q', ':', etc.
	if v := m.activeView(); v != nil && viewCapturesInput(v) {
		updated, cmd := v.Update(msg)
		m.viewStack[len(m.viewStack)-1] = updated.(View)
		return m, cmd
	}

	// Global keys when command bar is NOT focused
	switch {
	case msg.String() == ":":
		// Focus the command bar
		m.cmdBar.Focus()
		return m, nil

	case msg.String() == "q":
		m.quitting = true
		return m, tea.Quit

	case msg.Type == tea.KeyEsc:
		// Pop view stack (go back)
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
			m.lastOutput = ""
			return m, nil
		}
		return m, nil
	}

	// Forward to active view
	if v := m.activeView(); v != nil {
		updated, cmd := v.Update(msg)
		m.viewStack[len(m.viewStack)-1] = updated.(View)
		return m, cmd
	}

	return m, nil
}

func (m appModel) View() string {
	if m.quitting {
		return ""
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Content area: active view or command output
	if m.lastOutput != "" {
		sections = append(sections, m.lastOutput)
	} else if v := m.activeView(); v != nil {
		sections = append(sections, v.View())
	}

	// Status/shortcut bar
	sections = append(sections, m.renderStatusBar())

	// Command bar
	sections = append(sections, m.cmdBar.View())

	return strings.Join(sections, "\n")
}

// ── rendering helpers ────────────────────────────────────────────────────────

func (m *appModel) renderHeader() string {
	title := formatter.StylePurple.Render("kairos")

	// Breadcrumb from view stack
	var crumbs []string
	for _, v := range m.viewStack {
		if t := v.Title(); t != "" {
			crumbs = append(crumbs, t)
		}
	}
	breadcrumb := ""
	if len(crumbs) > 0 {
		breadcrumb = " " + formatter.Dim("›") + " " + formatter.Dim(strings.Join(crumbs, " › "))
	}

	header := title + breadcrumb

	// Right-align active project info
	if m.state.ActiveProjectID != "" {
		proj := formatter.StyleGreen.Render(m.state.ActiveShortID)
		header += "  " + formatter.Dim("[") + proj + formatter.Dim("]")
	}

	sep := formatter.Dim(strings.Repeat("─", max(m.state.Width, 20)))
	return header + "\n" + sep
}

func (m *appModel) renderStatusBar() string {
	var hints []string
	if v := m.activeView(); v != nil {
		for _, b := range v.ShortHelp() {
			hints = append(hints, formatter.Dim(b.Help().Key+": "+b.Help().Desc))
		}
	}
	// Show navigation hints
	if !m.cmdBar.Focused() {
		if len(m.viewStack) > 1 {
			hints = append(hints, formatter.Dim("esc: back"))
		}
		hints = append(hints, formatter.Dim(": command"))
	}

	bar := strings.Join(hints, "  ")
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(formatter.ColorDim))
	sep := sepStyle.Render(strings.Repeat("─", max(m.state.Width, 20)))
	return sep + "\n" + bar
}

// viewCapturesInput returns true if the active view has its own text input
// and should receive all key events (bypassing global keybindings like q/:/Esc).
func viewCapturesInput(v View) bool {
	if v == nil {
		return false
	}
	switch v.ID() {
	case ViewDraft, ViewHelpChat, ViewForm:
		return true
	}
	return false
}

package cli

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
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

	// Scrollable viewport for command output that exceeds terminal height.
	outputVP     viewport.Model
	outputActive bool // true when lastOutput is being displayed in the viewport
}

func newAppModel(app *App) appModel {
	state := &SharedState{
		App:   app,
		Cache: newShellProjectCache(),
	}
	cb := newCommandBar(state)

	vp := viewport.New(0, 0)
	vp.KeyMap = outputViewportKeyMap()
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	m := appModel{
		state:    state,
		cmdBar:   cb,
		outputVP: vp,
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

// setActiveView replaces the top of the view stack.
// If the stack is empty, this is a no-op.
func (m *appModel) setActiveView(v View) {
	if len(m.viewStack) > 0 {
		m.viewStack[len(m.viewStack)-1] = v
	}
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
		// Resize the output viewport if active.
		if m.outputActive {
			m.outputVP.Width = msg.Width
			m.outputVP.Height = m.state.ContentHeight()
		}
		// Forward to active view
		if v := m.activeView(); v != nil {
			updated, cmd := v.Update(msg)
			m.setActiveView(updated.(View))
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.outputActive {
			var cmd tea.Cmd
			m.outputVP, cmd = m.outputVP.Update(msg)
			return m, cmd
		}

	// Navigation messages from views or command bar
	case pushViewMsg:
		m.cmdBar.Blur()
		m.clearOutput()
		m.viewStack = append(m.viewStack, msg.view)
		return m, msg.view.Init()

	case popViewMsg:
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
		}
		return m, nil

	case replaceViewMsg:
		m.cmdBar.Blur()
		m.clearOutput()
		if len(m.viewStack) > 0 {
			m.viewStack[len(m.viewStack)-1] = msg.view
		} else {
			m.viewStack = append(m.viewStack, msg.view)
		}
		return m, msg.view.Init()

	case refreshViewMsg:
		// Broadcast to ALL views in the stack so underlying views (e.g. task list)
		// reload data after mutations made in views above them (e.g. action menu forms).
		var cmds []tea.Cmd
		for i, v := range m.viewStack {
			updated, cmd := v.Update(msg)
			m.viewStack[i] = updated.(View)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case cmdOutputMsg:
		m.lastOutput = msg.output
		m.outputActive = true
		m.outputVP.SetContent(msg.output)
		m.outputVP.Width = m.state.Width
		m.outputVP.Height = m.state.ContentHeight()
		m.outputVP.GotoTop()
		return m, nil

	case cmdLoadingMsg:
		m.lastOutput = "\n  " + formatter.Dim(msg.message)
		m.outputActive = true
		m.outputVP.SetContent(m.lastOutput)
		m.outputVP.Width = m.state.Width
		m.outputVP.Height = m.state.ContentHeight()
		m.outputVP.GotoTop()
		return m, nil

	case wizardCompleteMsg:
		// Atomically pop the wizard view and execute the follow-up command.
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
		}
		m.clearOutput()
		m.cmdBar.Focus()
		// Batch the follow-up command with a refresh so the underlying view reloads.
		return m, tea.Batch(msg.nextCmd, func() tea.Msg { return refreshViewMsg{} })

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
		m.setActiveView(updated.(View))
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
		if msg.Type == tea.KeyEnter {
			m.clearOutput() // Clear stale output before new command runs
		}
		cmd := m.cmdBar.Update(msg)
		return m, cmd
	}

	// When output is displayed, intercept scroll keys for the viewport.
	// Non-scroll keys dismiss the output, then fall through to normal handling.
	if m.outputActive {
		if isOutputScrollKey(msg) {
			var cmd tea.Cmd
			m.outputVP, cmd = m.outputVP.Update(msg)
			return m, cmd
		}
		m.clearOutput()
	}

	// If active view captures input (has its own text input), forward directly.
	// This bypasses global keybindings so views like draft and help chat can
	// receive all characters including 'q', ':', etc.
	if v := m.activeView(); v != nil && viewCapturesInput(v) {
		updated, cmd := v.Update(msg)
		m.setActiveView(updated.(View))
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

	case msg.String() == "?":
		// Global what-now: push recommendation view from any view.
		if v := m.activeView(); v != nil && v.ID() == ViewRecommendation {
			break // already on recommendation view, let it handle
		}
		m.cmdBar.Blur()
		m.clearOutput()
		v := newRecommendationView(m.state, 60)
		m.viewStack = append(m.viewStack, v)
		return m, v.Init()

	case msg.Type == tea.KeyEsc:
		// Pop view stack (go back)
		if len(m.viewStack) > 1 {
			m.viewStack = m.viewStack[:len(m.viewStack)-1]
			m.clearOutput()
			return m, nil
		}
		return m, nil
	}

	// Forward to active view
	if v := m.activeView(); v != nil {
		updated, cmd := v.Update(msg)
		m.setActiveView(updated.(View))
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

	// Content area: active view or scrollable command output
	if m.lastOutput != "" {
		if m.outputActive && m.state.Height > 0 {
			sections = append(sections, m.outputVP.View())
		} else {
			sections = append(sections, m.lastOutput)
		}
	} else if v := m.activeView(); v != nil {
		sections = append(sections, v.View())
	}

	// Status/shortcut bar
	sections = append(sections, m.renderStatusBar())

	// Command bar
	sections = append(sections, m.cmdBar.View())

	result := strings.Join(sections, "\n")

	// Pad to terminal height to prevent stale line artifacts from
	// bubbletea's line-diff renderer in alt-screen mode.
	if m.state.Height > 0 {
		lines := strings.Count(result, "\n") + 1
		if lines < m.state.Height {
			result += strings.Repeat("\n", m.state.Height-lines)
		}
	}

	return result
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

	if m.outputActive && m.outputVP.TotalLineCount() > m.outputVP.Height {
		// Scrollable output: show scroll position and controls.
		hints = append(hints, scrollIndicator(m.outputVP))
		hints = append(hints, formatter.Dim("↑↓ pgup/pgdn: scroll"))
		hints = append(hints, formatter.Dim("esc: dismiss"))
	} else if v := m.activeView(); v != nil && !m.outputActive {
		for _, b := range v.ShortHelp() {
			hints = append(hints, formatter.Dim(b.Help().Key+": "+b.Help().Desc))
		}
	}

	// Show navigation hints
	if !m.cmdBar.Focused() && !m.outputActive {
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

// clearOutput dismisses the transient command output and deactivates the viewport.
func (m *appModel) clearOutput() {
	m.lastOutput = ""
	m.outputActive = false
}

// outputViewportKeyMap returns a restricted keymap for the output viewport.
// Only arrow/page keys scroll — letter keys (q, j, k, etc.) are left free
// so they can dismiss the output or trigger global shortcuts.
func outputViewportKeyMap() viewport.KeyMap {
	return viewport.KeyMap{
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		Up:           key.NewBinding(key.WithKeys("up")),
		Down:         key.NewBinding(key.WithKeys("down")),
	}
}

// isOutputScrollKey returns true if the key should scroll the output viewport
// rather than dismissing the output.
func isOutputScrollKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown,
		tea.KeyHome, tea.KeyEnd, tea.KeyCtrlU, tea.KeyCtrlD:
		return true
	}
	return false
}

// scrollIndicator returns a dim scroll position string for the status bar.
func scrollIndicator(vp viewport.Model) string {
	if vp.AtTop() {
		return formatter.Dim("[TOP]")
	}
	if vp.AtBottom() {
		return formatter.Dim("[END]")
	}
	pct := int(vp.ScrollPercent() * 100)
	return formatter.Dim(fmt.Sprintf("[%d%%]", pct))
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

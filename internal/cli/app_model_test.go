package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubView struct {
	id         ViewID
	title      string
	viewText   string
	shortHelp  []key.Binding
	initCmd    tea.Cmd
	updateCmd  tea.Cmd
	updateSeen []tea.Msg
}

func (v *stubView) Init() tea.Cmd { return v.initCmd }

func (v *stubView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	v.updateSeen = append(v.updateSeen, msg)
	return v, v.updateCmd
}

func (v *stubView) View() string             { return v.viewText }
func (v *stubView) ID() ViewID               { return v.id }
func (v *stubView) ShortHelp() []key.Binding { return v.shortHelp }
func (v *stubView) Title() string            { return v.title }
func newStubView(id ViewID, title, text string) *stubView {
	return &stubView{id: id, title: title, viewText: text}
}

func TestNewAppModelStartsAtDashboard(t *testing.T) {
	m := newAppModel(testApp(t))

	require.Len(t, m.viewStack, 1)
	assert.Equal(t, ViewDashboard, m.activeView().ID())
}

func TestAppModel_NavigationMessages(t *testing.T) {
	m := newAppModel(testApp(t))
	v2 := newStubView(ViewProjectList, "Projects", "projects view")
	v3 := newStubView(ViewTaskList, "Tasks", "tasks view")

	model, cmd := m.Update(pushViewMsg{view: v2})
	m = model.(appModel)
	require.Nil(t, cmd)
	require.Len(t, m.viewStack, 2)
	assert.Equal(t, v2, m.activeView())

	model, cmd = m.Update(replaceViewMsg{view: v3})
	m = model.(appModel)
	require.Nil(t, cmd)
	require.Len(t, m.viewStack, 2)
	assert.Equal(t, v3, m.activeView())

	model, cmd = m.Update(popViewMsg{})
	m = model.(appModel)
	require.Nil(t, cmd)
	require.Len(t, m.viewStack, 1)
	assert.Equal(t, ViewDashboard, m.activeView().ID())
}

func TestAppModel_WindowResizeForwardsToActiveView(t *testing.T) {
	m := newAppModel(testApp(t))
	v := newStubView(ViewProjectList, "Projects", "projects")
	m.viewStack = []View{v}

	model, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(appModel)
	require.Nil(t, cmd)

	assert.Equal(t, 100, m.state.Width)
	assert.Equal(t, 30, m.state.Height)
	assert.NotZero(t, m.cmdBar.input.Width)
	require.Len(t, v.updateSeen, 1)
	_, ok := v.updateSeen[0].(tea.WindowSizeMsg)
	assert.True(t, ok)
}

func TestAppModel_KeyHandling_GlobalAndCaptured(t *testing.T) {
	t.Run("colon focuses command bar", func(t *testing.T) {
		m := newAppModel(testApp(t))
		require.False(t, m.cmdBar.Focused())

		model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
		m = model.(appModel)
		require.Nil(t, cmd)
		assert.True(t, m.cmdBar.Focused())
	})

	t.Run("q quits when active view does not capture input", func(t *testing.T) {
		m := newAppModel(testApp(t))
		m.viewStack = []View{newStubView(ViewDashboard, "Dashboard", "dashboard")}

		model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		m = model.(appModel)
		require.NotNil(t, cmd)
		assert.True(t, m.quitting)
		assert.IsType(t, tea.QuitMsg{}, cmd())
	})

	t.Run("capturing view receives q and does not quit", func(t *testing.T) {
		m := newAppModel(testApp(t))
		v := newStubView(ViewDraft, "Draft", "draft")
		m.viewStack = []View{v}

		model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		m = model.(appModel)
		require.Nil(t, cmd)
		assert.False(t, m.quitting)
		require.Len(t, v.updateSeen, 1)
		assert.Equal(t, "q", v.updateSeen[0].(tea.KeyMsg).String())
	})

	t.Run("esc pops back stack", func(t *testing.T) {
		m := newAppModel(testApp(t))
		m.viewStack = []View{
			newStubView(ViewDashboard, "Dashboard", "dashboard"),
			newStubView(ViewProjectList, "Projects", "projects"),
		}
		m.lastOutput = "stale output"
		m.outputActive = true

		model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = model.(appModel)
		require.Nil(t, cmd)
		require.Len(t, m.viewStack, 1)
		assert.Empty(t, m.lastOutput)
		assert.False(t, m.outputActive)
	})
}

func TestAppModel_WizardCompleteAndOutput(t *testing.T) {
	m := newAppModel(testApp(t))
	m.viewStack = []View{
		newStubView(ViewDashboard, "Dashboard", "dashboard"),
		newStubView(ViewForm, "Wizard", "wizard"),
	}

	m.cmdBar.Blur()
	next := func() tea.Msg { return cmdOutputMsg{output: "done"} }

	model, cmd := m.Update(wizardCompleteMsg{nextCmd: next})
	m = model.(appModel)
	require.NotNil(t, cmd)
	assert.True(t, m.cmdBar.Focused())
	require.Len(t, m.viewStack, 1)
	// wizardCompleteMsg now returns a tea.Batch (nextCmd + refreshViewMsg).
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	require.True(t, ok, "expected tea.BatchMsg, got %T", batchMsg)
	// Collect all messages from the batch.
	var gotOutput, gotRefresh bool
	for _, c := range batch {
		if c == nil {
			continue
		}
		switch c().(type) {
		case cmdOutputMsg:
			gotOutput = true
		case refreshViewMsg:
			gotRefresh = true
		}
	}
	assert.True(t, gotOutput, "batch should contain cmdOutputMsg")
	assert.True(t, gotRefresh, "batch should contain refreshViewMsg")

	model, cmd = m.Update(cmdOutputMsg{output: "hello"})
	m = model.(appModel)
	require.Nil(t, cmd)
	assert.Contains(t, m.View(), "hello")
}

func TestAppModel_LoadingMessage(t *testing.T) {
	m := newAppModel(testApp(t))

	// cmdLoadingMsg sets lastOutput to a dimmed loading indicator.
	model, cmd := m.Update(cmdLoadingMsg{message: "Thinking..."})
	m = model.(appModel)
	require.Nil(t, cmd)
	assert.Contains(t, m.lastOutput, "Thinking...")

	// Subsequent cmdOutputMsg replaces the loading message.
	model, cmd = m.Update(cmdOutputMsg{output: "result text"})
	m = model.(appModel)
	require.Nil(t, cmd)
	assert.Equal(t, "result text", m.lastOutput)
	assert.NotContains(t, m.lastOutput, "Thinking...")
}

func TestViewCapturesInput(t *testing.T) {
	assert.False(t, viewCapturesInput(nil))
	assert.True(t, viewCapturesInput(newStubView(ViewDraft, "Draft", "")))
	assert.True(t, viewCapturesInput(newStubView(ViewHelpChat, "Help", "")))
	assert.True(t, viewCapturesInput(newStubView(ViewForm, "Form", "")))
	assert.False(t, viewCapturesInput(newStubView(ViewDashboard, "Dash", "")))
}

func TestAppModel_OutputViewportScroll(t *testing.T) {
	m := newAppModel(testApp(t))
	m.viewStack = []View{newStubView(ViewDashboard, "Dashboard", "dashboard")}

	// Set terminal size (height 10 → content height = 5).
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = model.(appModel)

	// Generate output that exceeds viewport height.
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	content := strings.Join(lines, "\n")

	model, _ = m.Update(cmdOutputMsg{output: content})
	m = model.(appModel)
	assert.True(t, m.outputActive)

	// View should start at top — first line visible.
	view := m.View()
	assert.Contains(t, view, "line 1")

	// Scroll down: output stays active.
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(appModel)
	assert.True(t, m.outputActive)

	// Non-scroll key dismisses.
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = model.(appModel)
	assert.False(t, m.outputActive)
	assert.Empty(t, m.lastOutput)
}

func TestAppModel_OutputShortContentNoScroll(t *testing.T) {
	m := newAppModel(testApp(t))
	m.viewStack = []View{newStubView(ViewDashboard, "Dashboard", "dashboard")}

	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = model.(appModel)

	model, _ = m.Update(cmdOutputMsg{output: "short output"})
	m = model.(appModel)
	assert.True(t, m.outputActive)

	view := m.View()
	assert.Contains(t, view, "short output")
	// Status bar should NOT show scroll hints for short content.
	assert.NotContains(t, view, "pgup/pgdn")
}

func TestIsOutputScrollKey(t *testing.T) {
	scrollKeys := []tea.KeyType{
		tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown,
		tea.KeyHome, tea.KeyEnd, tea.KeyCtrlU, tea.KeyCtrlD,
	}
	for _, k := range scrollKeys {
		assert.True(t, isOutputScrollKey(tea.KeyMsg{Type: k}), "expected scroll key: %v", k)
	}

	nonScrollKeys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyRunes, Runes: []rune{':'}},
		{Type: tea.KeyEsc},
		{Type: tea.KeyEnter},
	}
	for _, k := range nonScrollKeys {
		assert.False(t, isOutputScrollKey(k), "expected non-scroll key: %v", k)
	}
}

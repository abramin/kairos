package cli

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/teatest"
)

// TestDriver wraps teatest.Driver with Kairos-specific inspection methods.
// It provides access to appModel internals (view stack, shared state,
// command bar focus) that the generic driver can't see.
type TestDriver struct {
	*teatest.Driver
}

// NewTestDriver creates a TestDriver from a test App.
// It constructs the appModel, sets terminal size, and drains Init()
// (which loads dashboard data synchronously via in-memory SQLite).
func NewTestDriver(t *testing.T, app *App) *TestDriver {
	t.Helper()

	m := newAppModel(app)
	d := teatest.New(t, m, teatest.WithSize(120, 40))
	d.DrainInit()

	return &TestDriver{Driver: d}
}

// ── High-level helpers ───────────────────────────────────────────────────────

// Command focuses the command bar with ':', types the command, and presses Enter.
// After execution, it blurs the command bar (via Esc) so subsequent key presses
// route to the active view rather than the text input.
// Commands that push a view (inspect, draft, help chat) auto-blur via pushViewMsg,
// but output-only commands (use, status) leave the bar focused — this helper
// normalizes to blurred so tests can interact with views immediately after.
func (d *TestDriver) Command(input string) {
	d.T.Helper()
	d.PressKey(':')
	d.Type(input)
	d.PressEnter()
	// Blur if the bar is still focused (output-only commands don't auto-blur).
	if d.CmdBarFocused() {
		d.PressEsc()
	}
}

// ── Kairos-specific inspection ───────────────────────────────────────────────

func (d *TestDriver) appModel() appModel {
	return d.Model.(appModel)
}

// ActiveViewID returns the ViewID of the top view on the stack.
func (d *TestDriver) ActiveViewID() ViewID {
	m := d.appModel()
	v := m.activeView()
	if v == nil {
		return ViewID(-1)
	}
	return v.ID()
}

// ActiveViewTitle returns the Title() of the top view on the stack.
func (d *TestDriver) ActiveViewTitle() string {
	m := d.appModel()
	v := m.activeView()
	if v == nil {
		return ""
	}
	return v.Title()
}

// ViewStackLen returns the number of views on the stack.
func (d *TestDriver) ViewStackLen() int {
	return len(d.appModel().viewStack)
}

// ViewStackIDs returns the ViewIDs of all views on the stack, bottom to top.
func (d *TestDriver) ViewStackIDs() []ViewID {
	m := d.appModel()
	ids := make([]ViewID, len(m.viewStack))
	for i, v := range m.viewStack {
		ids[i] = v.ID()
	}
	return ids
}

// State returns the shared state for inspection.
func (d *TestDriver) State() *SharedState {
	return d.appModel().state
}

// IsQuitting returns whether the app has signaled a quit.
// Checks model.quitting (q/Ctrl+C/quitMsg) and the driver's Quitting flag
// (tea.QuitMsg from exit/quit commands via tea.Quit).
func (d *TestDriver) IsQuitting() bool {
	return d.appModel().quitting || d.Quitting
}

// CmdBarFocused returns whether the command bar currently has focus.
func (d *TestDriver) CmdBarFocused() bool {
	m := d.appModel()
	return m.cmdBar.Focused()
}

// LastOutput returns the last command output displayed in the content area.
func (d *TestDriver) LastOutput() string {
	return d.appModel().lastOutput
}

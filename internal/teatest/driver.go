// Package teatest provides a synchronous test driver for bubbletea models.
//
// It replaces tea.Program in tests by calling Update() directly and
// synchronously draining returned Cmds. This enables deterministic,
// goroutine-free testing of tea.Model implementations.
//
// Cursor blink Cmds (which block on timer channels) are executed with a
// short timeout and skipped if they don't return promptly.
package teatest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MaxDrainDepth is the safety limit for command draining to prevent infinite loops.
const MaxDrainDepth = 100

// cmdTimeout is how long to wait for a Cmd to return before skipping it.
// Legitimate Cmds (DB queries, message factories) complete in microseconds.
// Cursor blink Cmds block for ~530ms, so 10ms safely separates the two.
const cmdTimeout = 10 * time.Millisecond

// Driver is a synchronous test harness for any tea.Model.
type Driver struct {
	T     *testing.T
	Model tea.Model

	// Quitting is set when tea.QuitMsg is seen during drain.
	// tea.QuitMsg is normally intercepted by the bubbletea runtime,
	// so the model may not handle it — the driver detects it explicitly.
	Quitting bool
}

// New creates a Driver for the given model and applies options.
// Call DrainInit() after construction to process the model's Init() command.
func New(t *testing.T, model tea.Model, opts ...Option) *Driver {
	t.Helper()
	d := &Driver{T: t, Model: model}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures the Driver during construction.
type Option func(*Driver)

// WithSize sends an initial WindowSizeMsg before any other processing.
func WithSize(w, h int) Option {
	return func(d *Driver) {
		d.T.Helper()
		updated, _ := d.Model.Update(tea.WindowSizeMsg{Width: w, Height: h})
		d.Model = updated
	}
}

// DrainInit executes the model's Init() command and drains all resulting messages.
func (d *Driver) DrainInit() {
	d.T.Helper()
	d.drainCmd(d.Model.Init(), 0)
}

// ── Core send methods ────────────────────────────────────────────────────────

// Send dispatches a message through Update and drains all resulting Cmds.
func (d *Driver) Send(msg tea.Msg) {
	d.T.Helper()
	if d.Quitting {
		return
	}
	updated, cmd := d.Model.Update(msg)
	d.Model = updated
	d.drainCmd(cmd, 0)
}

// ── Key event helpers ────────────────────────────────────────────────────────

// SendKey sends a tea.KeyMsg through the model.
func (d *Driver) SendKey(msg tea.KeyMsg) {
	d.T.Helper()
	d.Send(msg)
}

// PressKey sends a character key (rune).
func (d *Driver) PressKey(r rune) {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// PressEnter sends the Enter key.
func (d *Driver) PressEnter() {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyEnter})
}

// PressEsc sends the Escape key.
func (d *Driver) PressEsc() {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyEsc})
}

// PressCtrlC sends Ctrl+C.
func (d *Driver) PressCtrlC() {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyCtrlC})
}

// PressUp sends the Up arrow key.
func (d *Driver) PressUp() {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyUp})
}

// PressDown sends the Down arrow key.
func (d *Driver) PressDown() {
	d.T.Helper()
	d.SendKey(tea.KeyMsg{Type: tea.KeyDown})
}

// Type sends a string character by character as individual key events.
func (d *Driver) Type(s string) {
	d.T.Helper()
	for _, r := range s {
		d.PressKey(r)
	}
}

// View returns the full rendered output of the model.
func (d *Driver) View() string {
	return d.Model.View()
}

// ── Command draining ─────────────────────────────────────────────────────────

func (d *Driver) drainCmd(cmd tea.Cmd, depth int) {
	d.T.Helper()
	if cmd == nil || depth >= MaxDrainDepth {
		if depth >= MaxDrainDepth {
			d.T.Logf("teatest.Driver: drain depth limit (%d) reached", MaxDrainDepth)
		}
		return
	}

	msg := execCmdWithTimeout(cmd)
	if msg == nil {
		return
	}

	// Skip cursor blink messages that made it through.
	if isCursorBlink(msg) {
		return
	}

	// Handle BatchMsg: execute each sub-Cmd.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd == nil {
				continue
			}
			d.drainCmd(subCmd, depth+1)
		}
		return
	}

	// Detect tea.QuitMsg (produced by tea.Quit).
	if _, isQuit := msg.(tea.QuitMsg); isQuit {
		d.Quitting = true
		updated, _ := d.Model.Update(msg)
		d.Model = updated
		return
	}

	// Normal message: feed through Update and drain the result.
	updated, nextCmd := d.Model.Update(msg)
	d.Model = updated
	d.drainCmd(nextCmd, depth+1)
}

// execCmdWithTimeout runs a tea.Cmd in a goroutine with a timeout.
// Returns nil if the Cmd doesn't complete within cmdTimeout.
// This prevents blocking Cmds (like cursor.BlinkCmd, which waits on
// a timer channel for ~530ms) from hanging the test.
func execCmdWithTimeout(cmd tea.Cmd) tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(cmdTimeout):
		return nil
	}
}

// isCursorBlink detects cursor blink messages from the bubbles/cursor package.
// These are unexported types (initialBlinkMsg, BlinkMsg) that can chain
// into blocking timer Cmds when processed.
func isCursorBlink(msg tea.Msg) bool {
	t := fmt.Sprintf("%T", msg)
	return strings.Contains(t, "Blink") || strings.Contains(t, "blink")
}

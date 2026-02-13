package cli

import (
	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// wizardView wraps a huh.Form as a View on the navigation stack.
// When the form completes, it sends a wizardCompleteMsg with the
// done callback's result, allowing chained multi-step wizards.
type wizardView struct {
	state    *SharedState
	form     *huh.Form
	titleStr string
	done     func() tea.Cmd
}

func newWizardView(state *SharedState, title string, form *huh.Form, done func() tea.Cmd) *wizardView {
	return &wizardView{
		state:    state,
		form:     form,
		titleStr: title,
		done:     done,
	}
}

func (v *wizardView) Init() tea.Cmd {
	return v.form.Init()
}

func (v *wizardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Escape cancels the wizard.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		return v, func() tea.Msg { return wizardCompleteOutput(formatter.Dim("Cancelled.")) }
	}

	form, cmd := v.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		v.form = f
	}

	if v.form.State == huh.StateCompleted {
		var doneCmd tea.Cmd
		if v.done != nil {
			doneCmd = v.done()
		}
		return v, func() tea.Msg {
			return wizardCompleteMsg{nextCmd: tea.Batch(cmd, doneCmd)}
		}
	}

	return v, cmd
}

func (v *wizardView) View() string {
	return v.form.View()
}

func (v *wizardView) ID() ViewID            { return ViewForm }
func (v *wizardView) Title() string          { return v.titleStr }
func (v *wizardView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// startWizardCmd is a helper that creates a tea.Cmd to push a wizardView.
// If form is nil (no options available), it calls done() directly.
func startWizardCmd(state *SharedState, title string, form *huh.Form, done func() tea.Cmd) tea.Cmd {
	if form == nil {
		if done != nil {
			return done()
		}
		return nil
	}
	wv := newWizardView(state, title, form, done)
	return pushView(wv)
}

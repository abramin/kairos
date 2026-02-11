package cli

import tea "github.com/charmbracelet/bubbletea"

// Navigation messages used by views to request view transitions.
// The appModel handles these in its Update method.

// pushViewMsg pushes a new view onto the navigation stack.
type pushViewMsg struct {
	view View
}

// popViewMsg pops the current view off the navigation stack,
// returning to the previous view.
type popViewMsg struct{}

// replaceViewMsg replaces the current top view with a new one.
type replaceViewMsg struct {
	view View
}

// cmdOutputMsg carries text output from a command execution
// to be displayed transiently in the current view.
type cmdOutputMsg struct {
	output string
}

// wizardCompleteMsg is sent when a wizard form completes or is cancelled.
// The appModel handles it atomically: pop the wizard view, then run nextCmd.
type wizardCompleteMsg struct {
	nextCmd tea.Cmd
}

// pushView returns a tea.Cmd that pushes a view onto the stack.
func pushView(v View) tea.Cmd {
	return func() tea.Msg { return pushViewMsg{view: v} }
}

// popView returns a tea.Cmd that pops the current view.
func popView() tea.Cmd {
	return func() tea.Msg { return popViewMsg{} }
}

// replaceView returns a tea.Cmd that replaces the top view.
func replaceView(v View) tea.Cmd {
	return func() tea.Msg { return replaceViewMsg{view: v} }
}

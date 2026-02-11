package formatter

import "testing"

import "github.com/stretchr/testify/assert"

func TestFormatShellWelcome(t *testing.T) {
	out := FormatShellWelcome()
	assert.Contains(t, out, "kairos")
	assert.NotContains(t, out, "interactive shell")
	assert.Contains(t, out, "use <id>")
	assert.Contains(t, out, "projects")
	assert.Contains(t, out, "what-now 60")
	assert.Contains(t, out, "log")
	assert.Contains(t, out, "start")
	assert.Contains(t, out, "help")
	assert.Contains(t, out, "Tab for autocomplete")
}

func TestFormatShellHelp(t *testing.T) {
	out := FormatShellHelp()
	assert.Contains(t, out, "COMMANDS")
	assert.NotContains(t, out, "SHELL COMMANDS")
	assert.NotContains(t, out, "All CLI commands work directly")

	// Category headers
	assert.Contains(t, out, "NAVIGATION")
	assert.Contains(t, out, "PLANNING")
	assert.Contains(t, out, "QUICK ACTIONS")
	assert.Contains(t, out, "TRACKING")
	assert.Contains(t, out, "CREATION")
	assert.Contains(t, out, "INTELLIGENCE")
	assert.Contains(t, out, "UTILITIES")

	// Key commands present
	assert.Contains(t, out, "projects")
	assert.Contains(t, out, "what-now [min]")
	assert.Contains(t, out, "help chat [question]")
	assert.Contains(t, out, "draft [desc]")
	assert.Contains(t, out, "replan")
	assert.Contains(t, out, "exit / quit")
	assert.Contains(t, out, "session log")
	assert.Contains(t, out, "work add")
	assert.Contains(t, out, "node add")
	// New shortcut commands
	assert.Contains(t, out, "add [#node]")
	assert.Contains(t, out, "log [min]")
	assert.Contains(t, out, "start [id]")
	assert.Contains(t, out, "finish [id]")
	assert.Contains(t, out, "context")
}

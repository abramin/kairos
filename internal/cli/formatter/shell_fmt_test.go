package formatter

import "testing"

import "github.com/stretchr/testify/assert"

func TestFormatShellWelcome(t *testing.T) {
	out := FormatShellWelcome()
	assert.Contains(t, out, "interactive shell")
	assert.Contains(t, out, "Type 'help' for commands")
	assert.Contains(t, out, "Use Tab for autocomplete")
}

func TestFormatShellHelp(t *testing.T) {
	out := FormatShellHelp()
	assert.Contains(t, out, "SHELL COMMANDS")
	assert.Contains(t, out, "projects")
	assert.Contains(t, out, "what-now [min]")
	assert.Contains(t, out, "help chat [question]")
	assert.Contains(t, out, "All other CLI commands work directly")
}

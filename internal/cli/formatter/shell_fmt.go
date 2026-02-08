package formatter

import (
	"strings"
)

// FormatShellWelcome renders the welcome banner for the interactive shell.
func FormatShellWelcome() string {
	var b strings.Builder

	logo := StylePurple.Render("  kairos") + StyleDim.Render(" interactive shell")
	b.WriteString("\n")
	b.WriteString(logo + "\n")
	b.WriteString(StyleDim.Render("  ─────────────────────────────") + "\n")
	b.WriteString("\n")
	b.WriteString(StyleDim.Render("  Type 'help' for commands, 'exit' to quit.") + "\n")
	b.WriteString(StyleDim.Render("  Use Tab for autocomplete.") + "\n")
	b.WriteString("\n")

	return b.String()
}

// FormatShellHelp renders the help output for shell commands.
func FormatShellHelp() string {
	commands := [][]string{
		{"projects", "List all active projects"},
		{"use <id>", "Set active project (use with no args to clear)"},
		{"inspect [id]", "Show project details and plan tree"},
		{"status", "Show project status overview"},
		{"what-now [min]", "Get session recommendations (default: 60 min)"},
		{"help chat [question]", "Open interactive help chat or ask one-shot"},
		{"clear", "Clear the screen"},
		{"help", "Show this help message"},
		{"exit", "Quit the shell"},
	}

	headers := []string{"COMMAND", "DESCRIPTION"}
	rows := make([][]string, len(commands))
	for i, c := range commands {
		rows[i] = []string{StyleGreen.Render(c[0]), StyleDim.Render(c[1])}
	}

	note := "\n" + StyleDim.Render("All other CLI commands work directly (e.g. project add, node list, session log).")

	return RenderBox("Shell Commands", RenderTable(headers, rows)+note)
}

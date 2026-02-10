package formatter

import (
	"fmt"
	"strings"
)

// FormatShellWelcome renders the welcome banner shown on shell startup.
func FormatShellWelcome() string {
	var b strings.Builder

	logo := StylePurple.Render("  kairos")
	b.WriteString("\n")
	b.WriteString(logo + "\n")
	b.WriteString(StyleDim.Render("  ─────────────────────────────") + "\n")
	b.WriteString("\n")
	b.WriteString(StyleDim.Render("  Pick a project with 'use <id>', then commands apply to it automatically.") + "\n")
	b.WriteString("\n")
	b.WriteString("  " + StyleGreen.Render("projects") + StyleDim.Render("       List your projects") + "\n")
	b.WriteString("  " + StyleGreen.Render("use <id>") + StyleDim.Render("       Set active project") + "\n")
	b.WriteString("  " + StyleGreen.Render("what-now 60") + StyleDim.Render("    Get a 60-minute plan") + "\n")
	b.WriteString("  " + StyleGreen.Render("log") + StyleDim.Render("            Log a work session") + "\n")
	b.WriteString("  " + StyleGreen.Render("start") + StyleDim.Render("          Start a work item") + "\n")
	b.WriteString("  " + StyleGreen.Render("help") + StyleDim.Render("           Show all commands") + "\n")
	b.WriteString("\n")
	b.WriteString(StyleDim.Render("  Tab for autocomplete. Type 'help' for all commands.") + "\n")
	b.WriteString("\n")

	return b.String()
}

// helpCategory groups commands under a section header for the help display.
type helpCategory struct {
	title    string
	commands [][]string
}

// renderHelpCategory renders a single category section with header and command rows.
func renderHelpCategory(cat helpCategory) string {
	var b strings.Builder
	b.WriteString("\n " + StyleHeader.Render(strings.ToUpper(cat.title)) + "\n")
	for _, c := range cat.commands {
		b.WriteString(fmt.Sprintf("  %-24s %s\n",
			StyleGreen.Render(c[0]),
			StyleDim.Render(c[1])))
	}
	return b.String()
}

// FormatShellHelp renders the categorized command reference.
func FormatShellHelp() string {
	categories := []helpCategory{
		{
			title: "Navigation",
			commands: [][]string{
				{"projects", "List all active projects"},
				{"use <id>", "Set active project (no args to clear)"},
				{"inspect [id]", "Show project details and plan tree"},
			},
		},
		{
			title: "Planning",
			commands: [][]string{
				{"what-now [min]", "Get session recommendations (default: 60 min)"},
				{"status", "Show progress overview"},
				{"replan", "Rebalance project schedules"},
			},
		},
		{
			title: "Quick Actions",
			commands: [][]string{
				{"log [min]", "Log a work session (wizard for missing args)"},
				{"start [id]", "Start a work item (mark in-progress)"},
				{"finish [id]", "Finish a work item (mark done)"},
				{"context", "Show/set active project, item, and duration"},
			},
		},
		{
			title: "Tracking",
			commands: [][]string{
				{"session log", "Log a work session (wizard if flags omitted)"},
				{"work done <id>", "Mark a work item as done"},
				{"work update <id>", "Update a work item"},
			},
		},
		{
			title: "Creation",
			commands: [][]string{
				{"draft [desc]", "Create a new project (wizard or AI draft)"},
				{"project add", "Add a project manually"},
				{"project import <file>", "Import project from JSON"},
				{"node add", "Add a plan node (wizard if flags omitted)"},
				{"work add", "Add a work item (wizard if flags omitted)"},
			},
		},
		{
			title: "Intelligence",
			commands: [][]string{
				{"ask <question>", "Natural language command (requires LLM)"},
				{"explain now", "Explain current recommendations"},
				{"explain why-not", "Explain why an item was excluded"},
				{"review weekly", "Weekly progress review"},
			},
		},
		{
			title: "Utilities",
			commands: [][]string{
				{"help", "Show this command reference"},
				{"help chat [question]", "Interactive help (LLM or fuzzy match)"},
				{"clear", "Clear the screen"},
				{"exit / quit", "Quit kairos"},
			},
		},
	}

	var b strings.Builder
	for _, cat := range categories {
		b.WriteString(renderHelpCategory(cat))
	}

	b.WriteString("\n" + StyleDim.Render(
		"All project/node/work/session subcommands are available.\n"+
			"Tab for autocomplete. Active project context is auto-applied."))

	return RenderBox("Commands", b.String())
}

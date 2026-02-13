package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/intelligence"
	tea "github.com/charmbracelet/bubbletea"
)

// quitMsg signals the app to quit.
type quitMsg struct{}

// executeCommand dispatches a text command and returns a tea.Cmd.
// Commands may return cmdOutputMsg for display, navigation messages
// for view transitions, or quitMsg for exit.
func (c *commandBar) executeCommand(input string) tea.Cmd {
	parts, err := splitShellArgs(input)
	if err != nil {
		return outputCmd(shellError(err))
	}
	if len(parts) == 0 {
		return nil
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "projects":
		return c.cmdProjects()
	case "use":
		return c.cmdUse(args)
	case "inspect":
		return c.cmdInspect(args)
	case "status":
		return c.cmdStatus()
	case "what-now":
		return c.cmdWhatNow(args)
	case "log":
		return c.cmdLog(args)
	case "start":
		return c.cmdStart(args)
	case "finish":
		return c.cmdFinish(args)
	case "add":
		return c.cmdAdd(args)
	case "ask":
		return c.cmdAsk(args)
	case "explain":
		return c.cmdExplain(args)
	case "review":
		return c.cmdReview(args)
	case "replan":
		return outputCmd(c.cobraCapture(append([]string{"replan"}, args...)))
	case "context":
		return c.cmdContext(args)
	case "draft":
		description := ""
		if len(args) > 0 {
			description = strings.Join(args, " ")
		}
		return pushView(newDraftView(c.state, description))
	case "help":
		if len(args) > 0 && args[0] == "chat" {
			question := ""
			if len(args) > 1 {
				question = strings.Join(args[1:], " ")
			}
			if question != "" {
				return pushView(newHelpChatViewWithQuestion(c.state, question))
			}
			return pushView(newHelpChatView(c.state))
		}
		return outputCmd(formatter.FormatShellHelp())
	case "clear":
		return nil
	case "exit", "quit":
		return tea.Quit
	case "project":
		return c.cmdEntityGroup(parts)
	case "node", "work", "session":
		return c.cmdEntityGroup(parts)
	default:
		// Fall through to cobra capture for unrecognized commands
		output := c.cobraCapture(parts)
		return outputCmd(output)
	}
}

// outputCmd returns a tea.Cmd that sends a cmdOutputMsg.
func outputCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return func() tea.Msg { return cmdOutputMsg{output: s} }
}

// asyncOutputCmd wraps a blocking function in a tea.Cmd that runs
// asynchronously. The function's string result is delivered as a cmdOutputMsg.
// Use with tea.Batch(loadingCmd(...), asyncOutputCmd(fn)) to show a loading
// indicator while the work runs in a goroutine.
func asyncOutputCmd(fn func() string) tea.Cmd {
	return func() tea.Msg {
		result := fn()
		if result == "" {
			return nil
		}
		return cmdOutputMsg{output: result}
	}
}

// ── argument parsing helpers ─────────────────────────────────────────────────

// stripItemPrefix removes a leading "#" from an item reference (e.g. "#5" → "5").
func stripItemPrefix(s string) string {
	return strings.TrimPrefix(s, "#")
}

// parseLogArgs separates a mixed arg list into an item reference and a duration.
// Supports: "log 60", "log #5 45", "log #5", "log myitem 30".
func parseLogArgs(args []string) (itemArg, minutesArg string) {
	for _, a := range args {
		if v, err := strconv.Atoi(a); err == nil && v > 0 {
			if minutesArg == "" {
				minutesArg = a
			}
		} else if strings.HasPrefix(a, "#") {
			itemArg = a[1:]
		} else {
			itemArg = a
		}
	}
	return
}

// ── wizard chain helpers ─────────────────────────────────────────────────────

// ensureProject guarantees an active project is set before calling next.
// If no project is active, it launches a project-selection wizard.
func (c *commandBar) ensureProject(next func() tea.Cmd) tea.Cmd {
	if c.state.ActiveProjectID != "" {
		return next()
	}
	ctx := context.Background()
	var result string
	form := wizardSelectProject(ctx, c.state.App, &result)
	if form == nil {
		return outputCmd(formatter.StyleYellow.Render("No projects found. Create one first with 'draft'."))
	}
	return startWizardCmd(c.state, "Select Project", form, func() tea.Cmd {
		c.state.SetActiveProject(ctx, result)
		return next()
	})
}

// explainWithFallback tries the LLM explain service, falling back to a
// deterministic function if the service is nil or returns an error.
func (c *commandBar) explainWithFallback(
	llmFn func() (*intelligence.LLMExplanation, error),
	fallback func() *intelligence.LLMExplanation,
) *intelligence.LLMExplanation {
	if c.state.App.Explain != nil {
		if explanation, err := llmFn(); err == nil {
			return explanation
		}
	}
	return fallback()
}

// ── cobra passthrough ────────────────────────────────────────────────────────

// cobraCapture runs a command through the Cobra tree and captures output.
func (c *commandBar) cobraCapture(args []string) string {
	return captureCobraOutput(c.state.App, args, c.state.ActiveProjectID, c.state.ActiveShortID)
}

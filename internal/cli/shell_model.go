package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// shellMode tracks which interaction mode the shell is in.
type shellMode int

const (
	modePrompt   shellMode = iota // Normal command input.
	modeWizard                    // huh form is active.
	modeConfirm                   // Awaiting y/n for destructive command.
	modeDraft                     // Project draft conversation.
	modeHelpChat                  // Interactive help chat.
)

// shellModel is the bubbletea Model for the interactive shell REPL.
type shellModel struct {
	// bubbletea components
	input textinput.Model
	form  *huh.Form // active wizard form (nil when not in wizard mode)
	width int

	// shell state
	app               *App
	activeProjectID   string
	activeShortID     string
	activeProjectName string
	activeItemID      string
	activeItemTitle   string
	activeItemSeq     int
	lastDuration      int
	cache             *shellProjectCache

	// mode management
	mode       shellMode
	wizardDone func(m *shellModel) tea.Cmd // called when wizard form completes

	// confirmation
	pendingConfirm *pendingConfirmation

	// help chat
	helpSpecJSON string
	helpCmdInfos []intelligence.HelpCommandInfo
	helpConv     *intelligence.HelpConversation

	// draft
	draft *draftWizardState

	// transient context
	lastRecommendedItemID    string
	lastRecommendedItemTitle string
	lastInspectedProjectID   string

	// history
	history    []string
	historyIdx int

	// lifecycle
	quitting bool
}

// newShellModel creates a new bubbletea shell model.
func newShellModel(app *App) shellModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = ""
	ti.ShowSuggestions = true
	ti.CharLimit = 500
	// Use Tab for suggestion acceptance, reserve Up/Down for history.
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+n"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+p"))

	hist := loadShellHistory()

	return shellModel{
		input:      ti,
		app:        app,
		cache:      newShellProjectCache(),
		history:    hist,
		historyIdx: len(hist),
	}
}

// ── bubbletea interface ──────────────────────────────────────────────────────

func (m shellModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.Println(formatter.FormatShellWelcome()),
	)
}

func (m shellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = msg.Width - len(m.promptPrefix()) - 1
		if m.form != nil {
			m.form = m.form.WithWidth(msg.Width)
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit.
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.mode {
		case modeWizard:
			return m.updateWizard(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeDraft:
			return m.updateLineMode(msg, (*shellModel).handleDraftInput)
		case modeHelpChat:
			return m.updateLineMode(msg, (*shellModel).handleHelpChatInput)
		default:
			return m.updatePrompt(msg)
		}
	}

	// When in wizard mode, forward non-key messages to the huh form
	// (e.g. init messages, focus transitions) so it can function properly.
	if m.mode == modeWizard && m.form != nil {
		return m.updateWizard(msg)
	}

	// Pass other messages to textinput.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m shellModel) View() string {
	if m.quitting {
		return formatter.Dim("Goodbye.") + "\n"
	}

	if m.mode == modeWizard && m.form != nil {
		return m.form.View()
	}

	prefix := m.promptPrefix()
	return prefix + m.input.View()
}

// ── prompt prefix ────────────────────────────────────────────────────────────

func (m *shellModel) promptPrefix() string {
	switch m.mode {
	case modeConfirm:
		return formatter.StyleYellow.Render("confirm (y/n)") + " " + formatter.Dim("❯") + " "
	case modeHelpChat:
		return formatter.StylePurple.Render("help") + formatter.Dim("> ")
	case modeDraft:
		return formatter.StylePurple.Render("draft") + formatter.Dim("> ")
	default:
		if m.activeProjectID == "" {
			return formatter.StylePurple.Render("kairos") + " " + formatter.Dim("❯") + " "
		}
		return formatter.StylePurple.Render("kairos") + " " +
			formatter.Dim("(") + formatter.StyleGreen.Render(m.activeShortID) + formatter.Dim(")") +
			" " + formatter.Dim("❯") + " "
	}
}

// ── prompt mode ──────────────────────────────────────────────────────────────

func (m shellModel) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		input := strings.TrimSpace(m.input.Value())
		m.input.Reset()
		m.input.SetSuggestions(nil)
		if input == "" {
			return m, nil
		}
		m.addHistory(input)
		output, cmd := m.executeCommand(input)
		var cmds []tea.Cmd
		if output != "" {
			cmds = append(cmds, tea.Println(output))
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyUp:
		m.historyUp()
		return m, nil

	case tea.KeyDown:
		m.historyDown()
		return m, nil

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.updateSuggestions()
		return m, cmd
	}
}

// ── wizard mode ──────────────────────────────────────────────────────────────

// startWizard switches to wizard mode with the given form and completion callback.
func (m *shellModel) startWizard(form *huh.Form, done func(m *shellModel) tea.Cmd) tea.Cmd {
	if form == nil {
		// No options available — skip this wizard step.
		if done != nil {
			return done(m)
		}
		return nil
	}
	m.mode = modeWizard
	m.form = form
	m.wizardDone = done
	return m.form.Init()
}

func (m shellModel) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Escape cancels the wizard.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		m.mode = modePrompt
		m.form = nil
		m.wizardDone = nil
		return m, tea.Println(formatter.Dim("Cancelled."))
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		m.mode = modePrompt
		done := m.wizardDone
		m.form = nil
		m.wizardDone = nil
		if done != nil {
			doneCmd := done(&m)
			return m, tea.Batch(cmd, doneCmd)
		}
		return m, cmd
	}

	return m, cmd
}

// ── confirm mode ─────────────────────────────────────────────────────────────

func (m shellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		input := strings.TrimSpace(m.input.Value())
		m.input.Reset()
		pending := m.pendingConfirm
		m.pendingConfirm = nil
		m.mode = modePrompt

		switch strings.ToLower(input) {
		case "y", "yes":
			output := m.execCobraCapture(pending.args)
			return m, tea.Println(output)
		default:
			return m, tea.Println(formatter.Dim("Cancelled."))
		}
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// ── line-based modes (draft, help chat) ──────────────────────────────────────

type lineHandler func(m *shellModel, input string) (string, tea.Cmd)

func (m shellModel) updateLineMode(msg tea.KeyMsg, handler lineHandler) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		input := m.input.Value()
		m.input.Reset()
		output, cmd := handler(&m, input)
		var cmds []tea.Cmd
		if output != "" {
			cmds = append(cmds, tea.Println(output))
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// ── history ──────────────────────────────────────────────────────────────────

func (m *shellModel) addHistory(line string) {
	if line == "" {
		return
	}
	m.history = append(m.history, line)
	m.historyIdx = len(m.history)
	appendShellHistory(line)
}

func (m *shellModel) historyUp() {
	if m.historyIdx > 0 {
		m.historyIdx--
		m.input.SetValue(m.history[m.historyIdx])
		m.input.CursorEnd()
	}
}

func (m *shellModel) historyDown() {
	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.input.SetValue(m.history[m.historyIdx])
		m.input.CursorEnd()
	} else {
		m.historyIdx = len(m.history)
		m.input.SetValue("")
	}
}

// ── suggestions ──────────────────────────────────────────────────────────────

func (m *shellModel) updateSuggestions() {
	text := m.input.Value()
	if text == "" {
		m.input.SetSuggestions(nil)
		return
	}

	parts := strings.Fields(text)
	trailingSpace := strings.HasSuffix(text, " ")

	// First word — suggest commands.
	if len(parts) <= 1 && !trailingSpace {
		m.input.SetSuggestions(filterSuggestions(allCommandNames(), parts[0]))
		return
	}

	cmd := strings.ToLower(parts[0])

	// Second word — suggest subcommands or special args.
	if len(parts) <= 2 && (!trailingSpace || len(parts) == 1) {
		prefix := ""
		if len(parts) == 2 {
			prefix = parts[1]
		}

		switch cmd {
		case "use", "inspect":
			m.input.SetSuggestions(m.projectSuggestionsPlain(prefix))
			return
		case "context":
			m.input.SetSuggestions(filterSuggestions([]string{"clear", "project", "item"}, prefix))
			return
		case "what-now":
			m.input.SetSuggestions(filterSuggestions([]string{"30", "45", "60", "90", "120"}, prefix))
			return
		case "help":
			m.input.SetSuggestions(filterSuggestions([]string{"chat"}, prefix))
			return
		}

		if subs, ok := subcommandNames()[cmd]; ok {
			m.input.SetSuggestions(filterSuggestions(subs, prefix))
			return
		}
	}

	m.input.SetSuggestions(nil)
}

// projectSuggestionsPlain returns project ShortIDs matching a prefix.
func (m *shellModel) projectSuggestionsPlain(prefix string) []string {
	projects := m.cache.get(m.app)
	var suggestions []string
	for _, p := range projects {
		id := p.ShortID
		if id == "" && len(p.ID) >= 8 {
			id = p.ID[:8]
		}
		if prefix == "" || strings.HasPrefix(strings.ToLower(id), strings.ToLower(prefix)) {
			suggestions = append(suggestions, id)
		}
	}
	return suggestions
}

// allCommandNames returns all top-level shell command names.
func allCommandNames() []string {
	return []string{
		"projects", "use", "inspect",
		"status", "what-now", "replan",
		"log", "start", "finish", "context",
		"project", "node", "work", "session",
		"draft", "template",
		"ask", "explain", "review",
		"clear", "help", "exit", "quit",
	}
}

// subcommandNames returns subcommand lists by parent command.
func subcommandNames() map[string][]string {
	return map[string][]string{
		"project":  {"add", "list", "inspect", "update", "archive", "unarchive", "remove", "init", "import", "draft"},
		"node":     {"add", "inspect", "update", "remove"},
		"work":     {"add", "inspect", "update", "done", "archive", "remove"},
		"session":  {"log", "list", "remove"},
		"template": {"list", "show", "draft"},
		"explain":  {"now", "why-not"},
		"review":   {"weekly"},
	}
}

// filterSuggestions returns items from pool that start with prefix (case-insensitive).
func filterSuggestions(pool []string, prefix string) []string {
	if prefix == "" {
		return pool
	}
	lp := strings.ToLower(prefix)
	var result []string
	for _, s := range pool {
		if strings.HasPrefix(strings.ToLower(s), lp) {
			result = append(result, s)
		}
	}
	return result
}

// ── command dispatch ─────────────────────────────────────────────────────────

func (m *shellModel) executeCommand(input string) (string, tea.Cmd) {
	parts, err := splitShellArgs(input)
	if err != nil {
		return shellError(err), nil
	}
	if len(parts) == 0 {
		return "", nil
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "projects":
		return m.execProjects(), nil
	case "use":
		return m.execUse(args), nil
	case "inspect":
		return m.execInspect(args), nil
	case "status":
		return m.execStatus(), nil
	case "what-now":
		return m.execWhatNow(args), nil
	case "replan":
		return m.execCobraCapture(parts), nil
	case "log":
		return m.execLog(args)
	case "start":
		return m.execStart(args)
	case "finish":
		return m.execFinish(args)
	case "context":
		return m.execContext(args), nil
	case "draft":
		return m.execDraft(args), nil
	case "clear":
		return "\033[H\033[2J", nil
	case "help":
		if len(args) > 0 && args[0] == "chat" {
			return m.execHelpChat(args[1:])
		}
		return formatter.FormatShellHelp(), nil
	case "exit", "quit":
		m.quitting = true
		return "", tea.Quit
	case "shell":
		return formatter.StyleYellow.Render("Already in shell mode."), nil
	case "project":
		if len(args) > 0 && args[0] == "draft" {
			return m.execDraft(args[1:]), nil
		}
		return m.execMaybeDestructive(parts), nil
	case "node", "work", "session":
		if m.shouldStartWizard(parts) {
			return m.execWizardForCommand(parts)
		}
		return m.execMaybeDestructive(parts), nil
	default:
		return m.execCobraCapture(parts), nil
	}
}

// ── cobra pass-through ───────────────────────────────────────────────────────

// execCobraCapture runs a command through the Cobra tree and captures output.
func (m *shellModel) execCobraCapture(args []string) string {
	var buf strings.Builder
	root := NewRootCmd(m.app)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(prepareShellCobraArgs(args, m.activeProjectID))
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		errMsg := err.Error()
		buf.WriteString(shellError(err))
		if hint := m.hintForMissingFlag(errMsg); hint != "" {
			buf.WriteString("\n" + hint)
		}
		if strings.Contains(errMsg, "unknown command") && len(args) > 0 {
			buf.WriteString("\n" + m.suggestAlternatives(args[0]))
		}
	}
	return buf.String()
}

// ── destructive commands ─────────────────────────────────────────────────────

func (m *shellModel) execMaybeDestructive(parts []string) string {
	if len(parts) < 2 {
		return m.execCobraCapture(parts)
	}

	group := strings.ToLower(parts[0])
	sub := strings.ToLower(parts[1])

	subs, ok := destructiveCommands[group]
	if !ok || !subs[sub] {
		return m.execCobraCapture(parts)
	}

	for _, a := range parts[2:] {
		if a == "--yes" || a == "-y" || a == "--force" {
			return m.execCobraCapture(parts)
		}
	}

	target := ""
	if len(parts) > 2 {
		target = parts[2]
	}
	desc := fmt.Sprintf("%s %s", group, sub)
	if target != "" {
		desc += " " + target
	}

	m.mode = modeConfirm
	m.pendingConfirm = &pendingConfirmation{
		description: desc,
		args:        parts,
	}

	return fmt.Sprintf("%s %s\n%s",
		formatter.StyleYellow.Render("Confirm:"),
		desc+"?",
		formatter.Dim("Enter y to confirm, anything else to cancel."))
}

// ── hint and suggestion helpers ──────────────────────────────────────────────

func (m *shellModel) hintForMissingFlag(errMsg string) string {
	if !strings.Contains(errMsg, "required flag") {
		return ""
	}
	flagKeywords := []string{"project", "node", "work-item"}
	for _, kw := range flagKeywords {
		if strings.Contains(errMsg, kw) {
			if m.activeProjectID != "" {
				return formatter.Dim(fmt.Sprintf(
					"Hint: active project is %s — try adding --%s %s",
					m.activeShortID, kw, m.activeShortID,
				))
			}
			return formatter.Dim("Hint: set an active project with 'use <id>'")
		}
	}
	return ""
}

func (m *shellModel) suggestAlternatives(input string) string {
	root := NewRootCmd(m.app)
	spec := m.app.getCommandSpec(root)
	matches := spec.FuzzyMatch(input, 3)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(formatter.Dim("Did you mean:"))
	for _, match := range matches {
		path := strings.TrimPrefix(match.FullPath, "kairos ")
		b.WriteString(fmt.Sprintf("\n  %s  %s",
			formatter.StyleGreen.Render(path),
			formatter.Dim(match.Short),
		))
	}
	return b.String()
}

// ── help context ─────────────────────────────────────────────────────────────

func (m *shellModel) ensureHelpContext() {
	if m.helpSpecJSON != "" && len(m.helpCmdInfos) > 0 {
		return
	}
	root := NewRootCmd(m.app)
	spec := m.app.getCommandSpec(root)
	m.helpSpecJSON = SerializeCommandSpec(spec)
	m.helpCmdInfos = buildHelpCommandInfos(spec)
}

// ── help chat ────────────────────────────────────────────────────────────────

func (m *shellModel) execHelpChat(args []string) (string, tea.Cmd) {
	m.ensureHelpContext()

	if len(args) > 0 {
		question := strings.Join(args, " ")
		answer := resolveHelpAnswer(m.app, question, m.helpSpecJSON, m.helpCmdInfos)
		return formatter.FormatHelpAnswer(answer), nil
	}

	m.mode = modeHelpChat
	m.helpConv = nil
	return formatter.FormatHelpChatWelcome(), nil
}

func (m *shellModel) handleHelpChatInput(input string) (string, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "/quit", "/exit", "/q", "quit", "exit":
		m.mode = modePrompt
		m.helpConv = nil
		return "", nil
	case "/commands":
		return formatter.FormatCommandList(m.helpCmdInfos), nil
	}

	if strings.TrimSpace(input) == "" {
		return "", nil
	}

	if m.app.Help != nil {
		var answer *intelligence.HelpAnswer
		var err error
		if m.helpConv == nil {
			m.helpConv, answer, err = m.app.Help.StartChat(context.Background(), input, m.helpSpecJSON)
		} else {
			answer, err = m.app.Help.NextTurn(context.Background(), m.helpConv, input)
		}
		if err != nil {
			answer = intelligence.DeterministicHelp(input, m.helpCmdInfos)
		}
		return formatter.FormatHelpAnswer(answer), nil
	}

	answer := intelligence.DeterministicHelp(input, m.helpCmdInfos)
	return formatter.FormatHelpAnswer(answer), nil
}

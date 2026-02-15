package cli

import (
	"context"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// helpChatView is a dedicated view for the interactive help chat.
// It supports multi-turn conversation via the LLM HelpService
// or deterministic fallback when LLM is disabled.
type helpChatView struct {
	state *SharedState
	input textinput.Model

	// Chat state.
	conv     *intelligence.HelpConversation
	messages []string

	// Pre-computed help context.
	specJSON string
	cmdInfos []intelligence.HelpCommandInfo
}

func newHelpChatView(state *SharedState) *helpChatView {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 500

	// Build command spec for deterministic help.
	spec := state.App.getCommandSpec()
	specJSON := SerializeCommandSpec(spec)
	cmdInfos := buildHelpCommandInfos(spec)

	v := &helpChatView{
		state:    state,
		input:    ti,
		specJSON: specJSON,
		cmdInfos: cmdInfos,
	}

	v.messages = append(v.messages, formatter.FormatHelpChatWelcome())

	return v
}

// newHelpChatViewWithQuestion creates a help chat view and immediately
// answers a one-shot question, then displays the chat interface.
func newHelpChatViewWithQuestion(state *SharedState, question string) *helpChatView {
	v := newHelpChatView(state)

	answer := resolveHelpAnswer(state.App, question, v.specJSON, v.cmdInfos)
	v.messages = append(v.messages,
		formatter.Dim("You: ")+question,
		formatter.FormatHelpAnswer(answer),
	)

	return v
}

// ── tea.Model interface ──────────────────────────────────────────────────────

func (v *helpChatView) Init() tea.Cmd {
	return textinput.Blink
}

func (v *helpChatView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			return v, func() tea.Msg {
				return wizardCompleteMsg{nextCmd: nil}
			}
		}

		if msg.Type == tea.KeyEnter {
			input := strings.TrimSpace(v.input.Value())
			v.input.Reset()
			if input == "" {
				return v, nil
			}
			return v.handleInput(input)
		}

		var cmd tea.Cmd
		v.input, cmd = v.input.Update(msg)
		return v, cmd
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

func (v *helpChatView) View() string {
	var b strings.Builder

	for _, msg := range v.messages {
		b.WriteString(msg)
		b.WriteString("\n")
	}

	prompt := formatter.StylePurple.Render("help") + formatter.Dim("> ")
	b.WriteString(prompt)
	b.WriteString(v.input.View())

	return b.String()
}

// ── View interface ───────────────────────────────────────────────────────────

func (v *helpChatView) ID() ViewID   { return ViewHelpChat }
func (v *helpChatView) Title() string { return "Help" }
func (v *helpChatView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "ask")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// ── input handling ───────────────────────────────────────────────────────────

func (v *helpChatView) handleInput(input string) (tea.Model, tea.Cmd) {
	lower := strings.ToLower(input)
	switch lower {
	case "/quit", "/exit", "/q", "quit", "exit":
		return v, func() tea.Msg {
			return wizardCompleteMsg{nextCmd: nil}
		}
	case "/commands":
		v.messages = append(v.messages, formatter.FormatCommandList(v.cmdInfos))
		return v, nil
	}

	v.messages = append(v.messages, formatter.Dim("You: ")+input)

	if v.state.App.Help != nil {
		var answer *intelligence.HelpAnswer
		var err error
		if v.conv == nil {
			v.conv, answer, err = v.state.App.Help.StartChat(context.Background(), input, v.specJSON)
		} else {
			answer, err = v.state.App.Help.NextTurn(context.Background(), v.conv, input)
		}
		if err != nil {
			answer = intelligence.DeterministicHelp(input, v.cmdInfos)
		}
		v.messages = append(v.messages, formatter.FormatHelpAnswer(answer))
	} else {
		answer := intelligence.DeterministicHelp(input, v.cmdInfos)
		v.messages = append(v.messages, formatter.FormatHelpAnswer(answer))
	}

	return v, nil
}

// resolveHelpAnswer gets a help answer using LLM with fallback to deterministic.
func resolveHelpAnswer(app *App, question, specJSON string, cmdInfos []intelligence.HelpCommandInfo) *intelligence.HelpAnswer {
	if app.Help == nil {
		return intelligence.DeterministicHelp(question, cmdInfos)
	}

	ctx := context.Background()
	stopSpinner := formatter.StartSpinner("Thinking...")
	answer, err := app.Help.Ask(ctx, question, specJSON)
	stopSpinner()
	if err != nil {
		return intelligence.DeterministicHelp(question, cmdInfos)
	}

	return answer
}

// buildHelpCommandInfos converts CommandSpec entries into HelpCommandInfo
// for use by the intelligence layer (avoids import cycle).
func buildHelpCommandInfos(spec *CommandSpec) []intelligence.HelpCommandInfo {
	infos := make([]intelligence.HelpCommandInfo, len(spec.Commands))
	for i, cmd := range spec.Commands {
		infos[i] = intelligence.HelpCommandInfo{
			FullPath: cmd.FullPath,
			Short:    cmd.Short,
		}
	}
	return infos
}

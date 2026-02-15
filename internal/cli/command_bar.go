package cli

import (
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// commandBar is the persistent text input at the bottom of the TUI.
// It handles command entry, autocomplete suggestions, and history navigation.
type commandBar struct {
	input   textinput.Model
	state   *SharedState
	focused bool

	// history
	history    []string
	historyIdx int
}

func newCommandBar(state *SharedState) commandBar {
	ti := textinput.New()
	ti.Prompt = ""
	ti.ShowSuggestions = true
	ti.CharLimit = 500
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+n"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+p"))

	hist := loadShellHistory()

	return commandBar{
		input:      ti,
		state:      state,
		focused:    false,
		history:    hist,
		historyIdx: len(hist),
	}
}

// Focus gives focus to the command bar.
func (c *commandBar) Focus() {
	c.focused = true
	c.input.Focus()
}

// Blur removes focus from the command bar.
func (c *commandBar) Blur() {
	c.focused = false
	c.input.Blur()
}

// Focused returns whether the command bar has focus.
func (c *commandBar) Focused() bool {
	return c.focused
}

// SetWidth updates the input width for terminal resizing.
func (c *commandBar) SetWidth(w int) {
	promptLen := len(c.promptPrefixPlain())
	c.input.Width = w - promptLen - 1
}

// Update handles key messages when the command bar is focused.
// Returns a tea.Cmd that may include navigation or output messages.
func (c *commandBar) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		input := strings.TrimSpace(c.input.Value())
		c.input.Reset()
		c.input.SetSuggestions(nil)
		if input == "" {
			return nil
		}
		c.addHistory(input)
		return c.executeCommand(input)

	case tea.KeyUp:
		c.historyUp()
		return nil

	case tea.KeyDown:
		c.historyDown()
		return nil

	case tea.KeyEsc:
		c.Blur()
		return nil

	default:
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		c.updateSuggestions()
		return cmd
	}
}

// UpdateNonKey handles non-key messages (e.g., cursor blink).
func (c *commandBar) UpdateNonKey(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// View renders the command bar.
func (c *commandBar) View() string {
	if !c.focused {
		return c.promptPrefix() + formatter.Dim("press : to type a command")
	}
	return c.promptPrefix() + c.input.View()
}

// promptPrefix returns the styled prompt string.
func (c *commandBar) promptPrefix() string {
	if c.state.ActiveProjectID == "" {
		return formatter.StylePurple.Render("kairos") + " " + formatter.Dim("❯") + " "
	}
	return formatter.StylePurple.Render("kairos") + " " +
		formatter.Dim("(") + formatter.StyleGreen.Render(c.state.ActiveShortID) + formatter.Dim(")") +
		" " + formatter.Dim("❯") + " "
}

// promptPrefixPlain returns the plain-text prompt length for width calculations.
func (c *commandBar) promptPrefixPlain() string {
	if c.state.ActiveProjectID == "" {
		return "kairos > "
	}
	return "kairos (" + c.state.ActiveShortID + ") > "
}

// ── history ──────────────────────────────────────────────────────────────────

func (c *commandBar) addHistory(line string) {
	if line == "" {
		return
	}
	c.history = append(c.history, line)
	c.historyIdx = len(c.history)
	appendShellHistory(line)
}

func (c *commandBar) historyUp() {
	if c.historyIdx > 0 {
		c.historyIdx--
		c.input.SetValue(c.history[c.historyIdx])
		c.input.CursorEnd()
	}
}

func (c *commandBar) historyDown() {
	if c.historyIdx < len(c.history)-1 {
		c.historyIdx++
		c.input.SetValue(c.history[c.historyIdx])
		c.input.CursorEnd()
	} else {
		c.historyIdx = len(c.history)
		c.input.SetValue("")
	}
}

// ── suggestions ──────────────────────────────────────────────────────────────

func (c *commandBar) updateSuggestions() {
	text := c.input.Value()
	if text == "" {
		c.input.SetSuggestions(nil)
		return
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		c.input.SetSuggestions(nil)
		return
	}
	trailingSpace := strings.HasSuffix(text, " ")

	if len(parts) == 1 && !trailingSpace {
		c.input.SetSuggestions(pruneExactSuggestions(filterSuggestions(allCommandNames(), parts[0]), text))
		return
	}

	typedCmd := parts[0]
	cmd := strings.ToLower(typedCmd)

	// Suggest second-token completions when the user is typing:
	//   "<cmd> "  or  "<cmd> <prefix>"
	if len(parts) <= 2 && (len(parts) == 1 || !trailingSpace) {
		prefix := ""
		if len(parts) == 2 {
			prefix = parts[1]
		}
		var tokenSuggestions []string

		switch cmd {
		case "use", "inspect":
			tokenSuggestions = c.projectSuggestions(prefix)
		case "context":
			tokenSuggestions = filterSuggestions([]string{"clear", "project", "item"}, prefix)
		case "what-now", "log":
			tokenSuggestions = filterSuggestions([]string{"30", "45", "60", "90", "120"}, prefix)
		case "explain":
			tokenSuggestions = filterSuggestions([]string{"now", "why-not"}, prefix)
		case "review":
			tokenSuggestions = filterSuggestions([]string{"weekly"}, prefix)
		case "help":
			tokenSuggestions = filterSuggestions([]string{"chat"}, prefix)
		}

		if len(tokenSuggestions) == 0 {
			if subs, ok := subcommandNames()[cmd]; ok {
				tokenSuggestions = filterSuggestions(subs, prefix)
			}
		}
		if len(tokenSuggestions) > 0 {
			fullSuggestions := qualifySuggestions(typedCmd, tokenSuggestions)
			c.input.SetSuggestions(pruneExactSuggestions(fullSuggestions, text))
			return
		}
	}

	c.input.SetSuggestions(nil)
}

// qualifySuggestions expands token candidates to full command-line suggestions.
func qualifySuggestions(cmd string, suggestions []string) []string {
	full := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		full = append(full, cmd+" "+s)
	}
	return full
}

// pruneExactSuggestions removes exact (case-insensitive) matches of the
// current input so the cursor remains visible after completion.
func pruneExactSuggestions(suggestions []string, input string) []string {
	target := strings.ToLower(strings.TrimSpace(input))
	if target == "" {
		return suggestions
	}
	filtered := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		if strings.ToLower(strings.TrimSpace(s)) == target {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func (c *commandBar) projectSuggestions(prefix string) []string {
	projects := c.state.Cache.get(c.state.App)
	var suggestions []string
	for _, p := range projects {
		id := p.DisplayID()
		if prefix == "" || strings.HasPrefix(strings.ToLower(id), strings.ToLower(prefix)) {
			suggestions = append(suggestions, id)
		}
	}
	return suggestions
}

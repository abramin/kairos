package cli

import (
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
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
	trailingSpace := strings.HasSuffix(text, " ")

	if len(parts) <= 1 && !trailingSpace {
		c.input.SetSuggestions(filterSuggestions(allCommandNames(), parts[0]))
		return
	}

	cmd := strings.ToLower(parts[0])

	if len(parts) <= 2 && (!trailingSpace || len(parts) == 1) {
		prefix := ""
		if len(parts) == 2 {
			prefix = parts[1]
		}

		switch cmd {
		case "use", "inspect":
			c.input.SetSuggestions(c.projectSuggestions(prefix))
			return
		case "context":
			c.input.SetSuggestions(filterSuggestions([]string{"clear", "project", "item"}, prefix))
			return
		case "what-now", "log":
			c.input.SetSuggestions(filterSuggestions([]string{"30", "45", "60", "90", "120"}, prefix))
			return
		case "explain":
			c.input.SetSuggestions(filterSuggestions([]string{"now", "why-not"}, prefix))
			return
		case "review":
			c.input.SetSuggestions(filterSuggestions([]string{"weekly"}, prefix))
			return
		case "help":
			c.input.SetSuggestions(filterSuggestions([]string{"chat"}, prefix))
			return
		}

		if subs, ok := subcommandNames()[cmd]; ok {
			c.input.SetSuggestions(filterSuggestions(subs, prefix))
			return
		}
	}

	c.input.SetSuggestions(nil)
}

func (c *commandBar) projectSuggestions(prefix string) []string {
	projects := c.state.Cache.get(c.state.App)
	var suggestions []string
	for _, p := range projects {
		id := domain.DisplayID(p.ShortID, p.ID)
		if prefix == "" || strings.HasPrefix(strings.ToLower(id), strings.ToLower(prefix)) {
			suggestions = append(suggestions, id)
		}
	}
	return suggestions
}

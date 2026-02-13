package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// menuAction represents a single option in the action menu.
type menuAction struct {
	label string
	key   string // single-key shortcut
	fn    func() tea.Cmd
}

// actionMenuView presents a list of actions for a selected work item.
type actionMenuView struct {
	state     *SharedState
	itemID    string
	itemTitle string
	itemSeq   int
	cursor    int
	actions   []menuAction
}

func newActionMenuView(state *SharedState, itemID, title string, seq int) *actionMenuView {
	v := &actionMenuView{
		state:     state,
		itemID:    itemID,
		itemTitle: title,
		itemSeq:   seq,
	}
	v.actions = []menuAction{
		{label: "Start Timer", key: "s", fn: v.actionStart},
		{label: "Log Past Session", key: "l", fn: v.actionLog},
		{label: "Adjust Logged Time", key: "a", fn: v.actionAdjustLogged},
		{label: "Mark Done", key: "d", fn: v.actionMarkDone},
		{label: "Edit Details", key: "e", fn: v.actionEdit},
		{label: "Delete", key: "x", fn: v.actionDelete},
	}
	return v
}

func (v *actionMenuView) ID() ViewID   { return ViewActionMenu }
func (v *actionMenuView) Title() string { return "Actions" }

func (v *actionMenuView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *actionMenuView) Init() tea.Cmd { return nil }

func (v *actionMenuView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.actions)-1 {
				v.cursor++
			}
		case "enter":
			if v.cursor < len(v.actions) {
				return v, v.actions[v.cursor].fn()
			}
		default:
			for i, a := range v.actions {
				if msg.String() == a.key {
					v.cursor = i
					return v, a.fn()
				}
			}
		}
	}
	return v, nil
}

func (v *actionMenuView) View() string {
	var b strings.Builder

	seqStr := ""
	if v.itemSeq > 0 {
		seqStr = formatter.Dim(fmt.Sprintf("#%d ", v.itemSeq))
	}
	b.WriteString("\n")
	b.WriteString("  " + formatter.StyleHeader.Render("ACTIONS") + "\n")
	b.WriteString("  " + formatter.Dim("for ") + seqStr + formatter.Bold(v.itemTitle) + "\n\n")

	for i, a := range v.actions {
		cursor := "  "
		style := formatter.StyleFg
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
			style = formatter.StyleBold
		}
		keyHint := formatter.Dim("[" + a.key + "]")
		b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, style.Render(a.label), keyHint))
	}

	return b.String()
}

// ── action handlers ──────────────────────────────────────────────────────────

func (v *actionMenuView) actionStart() tea.Cmd {
	id, title, seq, state := v.itemID, v.itemTitle, v.itemSeq, v.state
	return func() tea.Msg {
		return wrapAsWizardComplete(func() (string, error) {
			return execStartItem(context.Background(), state.App, state, id, title, seq)
		})
	}
}

func (v *actionMenuView) actionLog() tea.Cmd {
	return pushView(newLogFormView(v.state, v.itemID, v.itemTitle))
}

func (v *actionMenuView) actionAdjustLogged() tea.Cmd {
	return pushView(newAdjustLoggedView(v.state, v.itemID, v.itemTitle))
}

func (v *actionMenuView) actionMarkDone() tea.Cmd {
	id, title, state := v.itemID, v.itemTitle, v.state
	return func() tea.Msg {
		return wrapAsWizardComplete(func() (string, error) {
			return execMarkDone(context.Background(), state.App, state, id, title)
		})
	}
}

func (v *actionMenuView) actionEdit() tea.Cmd {
	return pushView(newEditWorkItemView(v.state, v.itemID, v.itemTitle))
}

func (v *actionMenuView) actionDelete() tea.Cmd {
	return execDeleteItem(v.state, v.itemID, v.itemTitle)
}

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
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

	// Detail data (nil if fetch failed — graceful degradation).
	item     *domain.WorkItem
	nodeName string
}

func newActionMenuView(state *SharedState, itemID, title string, seq int) *actionMenuView {
	v := &actionMenuView{
		state:     state,
		itemID:    itemID,
		itemTitle: title,
		itemSeq:   seq,
	}

	// Fetch full work item for detail display.
	// Failure is non-fatal: the view degrades to showing just the title.
	ctx := context.Background()
	if item, err := state.App.WorkItems.GetByID(ctx, itemID); err == nil {
		v.item = item
		if node, err := state.App.Nodes.GetByID(ctx, item.NodeID); err == nil {
			v.nodeName = node.Title
		}
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
	case refreshViewMsg:
		// Reload work item data to reflect mutations.
		ctx := context.Background()
		if item, err := v.state.App.WorkItems.GetByID(ctx, v.itemID); err == nil {
			v.item = item
			v.itemTitle = item.Title
		}
		return v, nil
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
	b.WriteString("  " + formatter.Dim("for ") + seqStr + formatter.Bold(v.itemTitle) + "\n")

	// Work item details (empty string if data unavailable).
	if details := v.renderDetails(); details != "" {
		b.WriteString("\n")
		b.WriteString(details)
	}

	b.WriteString("\n")

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

// renderDetails returns a formatted work item detail section.
// Returns empty string if no item data is available.
func (v *actionMenuView) renderDetails() string {
	if v.item == nil {
		return ""
	}
	item := v.item
	var b strings.Builder

	// Status pill
	b.WriteString("  " + formatter.WorkItemStatusPill(item.Status) + "\n")

	// Type badge
	if item.Type != "" {
		label := strings.ToUpper(item.Type[:1]) + item.Type[1:]
		b.WriteString("  " + formatter.Dim("Type      ") + formatter.StylePurple.Render(label) + "\n")
	}

	// Parent node context
	if v.nodeName != "" {
		b.WriteString("  " + formatter.Dim("Section   ") + formatter.StyleFg.Render(v.nodeName) + "\n")
	}

	// Progress bar (logged / planned)
	if item.PlannedMin > 0 {
		pct := float64(item.LoggedMin) / float64(item.PlannedMin)
		if pct > 1.0 {
			pct = 1.0
		}
		b.WriteString("  " + formatter.Dim("Progress  ") + formatter.RenderProgress(pct, 14) + "\n")
		b.WriteString(fmt.Sprintf("            %s logged / %s planned\n",
			formatter.Bold(formatter.FormatMinutes(item.LoggedMin)),
			formatter.Dim(formatter.FormatMinutes(item.PlannedMin)),
		))
	} else if item.LoggedMin > 0 {
		b.WriteString(fmt.Sprintf("  %s %s logged (no estimate)\n",
			formatter.Dim("Time      "),
			formatter.Bold(formatter.FormatMinutes(item.LoggedMin)),
		))
	}

	// Units progress
	if item.UnitsTotal > 0 {
		unitLabel := "units"
		if item.UnitsKind != "" {
			unitLabel = item.UnitsKind
		}
		b.WriteString(fmt.Sprintf("  %s %d/%d %s\n",
			formatter.Dim("Units     "),
			item.UnitsDone, item.UnitsTotal, unitLabel,
		))
	}

	// Due date
	if item.DueDate != nil {
		b.WriteString("  " + formatter.Dim("Due       ") + formatter.RelativeDateStyled(*item.DueDate))
		daysLeft := int(time.Until(*item.DueDate).Hours() / 24)
		if daysLeft >= 0 {
			b.WriteString(formatter.Dim(fmt.Sprintf(" (%dd left)", daysLeft)))
		}
		b.WriteString("\n")
	}

	// Not-before constraint
	if item.NotBefore != nil && time.Now().Before(*item.NotBefore) {
		b.WriteString("  " + formatter.Dim("Starts    ") + formatter.RelativeDateStyled(*item.NotBefore) + "\n")
	}

	// Session constraints (only show non-default values)
	var sessionParts []string
	if item.MinSessionMin > 0 {
		sessionParts = append(sessionParts, fmt.Sprintf("min %s", formatter.FormatMinutes(item.MinSessionMin)))
	}
	if item.MaxSessionMin > 0 {
		sessionParts = append(sessionParts, fmt.Sprintf("max %s", formatter.FormatMinutes(item.MaxSessionMin)))
	}
	if item.DefaultSessionMin > 0 {
		sessionParts = append(sessionParts, fmt.Sprintf("default %s", formatter.FormatMinutes(item.DefaultSessionMin)))
	}
	if len(sessionParts) > 0 {
		b.WriteString("  " + formatter.Dim("Session   ") + formatter.Dim(strings.Join(sessionParts, ", ")) + "\n")
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

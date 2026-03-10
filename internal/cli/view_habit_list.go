package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// habitListLoadedMsg signals that habit status data has been loaded.
type habitListLoadedMsg struct {
	habits []service.HabitStatus
	err    error
}

// habitLoggedMsg signals that a habit session has been logged.
type habitLoggedMsg struct {
	log   *domain.HabitLog
	title string
	err   error
}

// habitUndoneMsg signals that a habit log has been undone.
type habitUndoneMsg struct {
	title string
	err   error
}

// undoableLog tracks the most recent log for undo support.
type undoableLog struct {
	logID      string
	habitID    string
	habitTitle string
}

// habitListView is an interactive habit checklist with mark-done and undo.
type habitListView struct {
	state    *SharedState
	habits   []service.HabitStatus
	cursor   int
	loading  bool
	err      error
	lastLog  *undoableLog // most recent log created in this session
	feedback string       // transient status message
}

func newHabitListView(state *SharedState) *habitListView {
	return &habitListView{state: state, loading: true}
}

func (v *habitListView) ID() ViewID    { return ViewHabitList }
func (v *habitListView) Title() string { return "Habits" }

func (v *habitListView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "done")),
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *habitListView) Init() tea.Cmd {
	return v.loadHabits()
}

func (v *habitListView) loadHabits() tea.Cmd {
	svc := v.state.App.Habits
	return func() tea.Msg {
		habits, err := svc.GetStatus(context.Background(), time.Now().UTC())
		return habitListLoadedMsg{habits: habits, err: err}
	}
}

func (v *habitListView) clampCursor() {
	if len(v.habits) == 0 {
		v.cursor = 0
		return
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.habits) {
		v.cursor = len(v.habits) - 1
	}
}

func (v *habitListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case habitListLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.habits = msg.habits
		v.clampCursor()
		return v, nil

	case habitLoggedMsg:
		if msg.err != nil {
			v.feedback = formatter.StyleRed.Render("Error: " + msg.err.Error())
			return v, nil
		}
		v.lastLog = &undoableLog{
			logID:      msg.log.ID,
			habitID:    msg.log.HabitID,
			habitTitle: msg.title,
		}
		v.feedback = formatter.StyleGreen.Render(fmt.Sprintf("Logged %s!", msg.title))
		v.loading = true
		return v, v.loadHabits()

	case habitUndoneMsg:
		if msg.err != nil {
			v.feedback = formatter.StyleRed.Render("Error: " + msg.err.Error())
			return v, nil
		}
		v.lastLog = nil
		v.feedback = formatter.StyleGreen.Render(fmt.Sprintf("Undone %s!", msg.title))
		v.loading = true
		return v, v.loadHabits()

	case refreshViewMsg:
		v.loading = true
		return v, v.loadHabits()

	case tea.KeyMsg:
		// Clear feedback on any keypress.
		v.feedback = ""

		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.habits)-1 {
				v.cursor++
			}
		case " ", "enter":
			if v.cursor < len(v.habits) {
				return v, v.logHabitCmd(v.habits[v.cursor])
			}
		case "u":
			if v.lastLog != nil {
				return v, v.undoLogCmd(v.lastLog)
			}
		case "r":
			v.loading = true
			return v, v.loadHabits()
		}
	}
	return v, nil
}

func (v *habitListView) logHabitCmd(hs service.HabitStatus) tea.Cmd {
	svc := v.state.App.Habits
	habitID := hs.Habit.ID
	title := hs.Habit.Title
	return func() tea.Msg {
		log, err := svc.LogSession(context.Background(), service.LogHabitRequest{
			HabitID: habitID,
		})
		return habitLoggedMsg{log: log, title: title, err: err}
	}
}

func (v *habitListView) undoLogCmd(undo *undoableLog) tea.Cmd {
	svc := v.state.App.Habits
	logID := undo.logID
	title := undo.habitTitle
	return func() tea.Msg {
		err := svc.UndoLog(context.Background(), logID)
		return habitUndoneMsg{title: title, err: err}
	}
}

func (v *habitListView) View() string {
	if v.loading {
		return formatter.Dim("Loading...")
	}
	if v.err != nil {
		return formatter.StyleRed.Render("Error: " + v.err.Error())
	}

	var b strings.Builder
	b.WriteString(formatter.Header("habits"))
	b.WriteString("\n\n")

	if len(v.habits) == 0 {
		b.WriteString(formatter.Dim("No habits configured. Use 'habits add' to create one."))
		return b.String()
	}

	for i, hs := range v.habits {
		cursor := "  "
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
		}

		// Status icon.
		icon := "○"
		if hs.DaysSinceLog == 0 {
			icon = formatter.StyleGreen.Render("✓")
		}

		// Title.
		title := hs.Habit.Title
		if i == v.cursor {
			title = formatter.Bold(title)
		}

		// Cadence label.
		cadence := "daily"
		if hs.Habit.CadenceDays == 7 {
			cadence = "weekly"
		} else if hs.Habit.CadenceDays > 1 {
			cadence = fmt.Sprintf("every %dd", hs.Habit.CadenceDays)
		}

		// Status badge.
		var status string
		switch {
		case hs.DaysSinceLog == 0:
			status = formatter.StyleGreen.Render("done today")
		case hs.DaysSinceLog == 9999:
			status = formatter.StyleYellow.Render("never done")
		case hs.DaysUntilDue < 0:
			status = formatter.StyleYellow.Render(fmt.Sprintf("overdue %dd", -hs.DaysUntilDue))
		case hs.DaysUntilDue == 0:
			status = formatter.StyleYellow.Render("due today")
		default:
			status = formatter.Dim(fmt.Sprintf("due in %dd", hs.DaysUntilDue))
		}

		b.WriteString(fmt.Sprintf("%s%s %s  %s  %s  %s\n",
			cursor,
			icon,
			title,
			formatter.Dim(cadence),
			formatter.Dim(formatter.FormatMinutes(hs.Habit.TargetMin)),
			status,
		))
	}

	// Feedback line.
	if v.feedback != "" {
		b.WriteString("\n")
		b.WriteString(v.feedback)
		b.WriteString("\n")
	}

	// Undo hint.
	b.WriteString("\n")
	helpParts := []string{"space done"}
	if v.lastLog != nil {
		helpParts = append(helpParts, "u undo")
	}
	helpParts = append(helpParts, "r refresh", "esc back")
	b.WriteString(formatter.Dim(strings.Join(helpParts, "  ")))

	return b.String()
}

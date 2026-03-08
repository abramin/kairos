package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// checklistLoadedMsg signals that task list data has been loaded.
type checklistLoadedMsg struct {
	tasks []*domain.Task
	err   error
}

// checklistView is a global (non-project-scoped) task checklist.
type checklistView struct {
	state   *SharedState
	tasks   []*domain.Task
	cursor  int
	loading bool
	err     error
}

func newChecklistView(state *SharedState) *checklistView {
	return &checklistView{state: state, loading: true}
}

func (v *checklistView) ID() ViewID    { return ViewCheckList }
func (v *checklistView) Title() string { return "Tasks" }

func (v *checklistView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "done")),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
		key.NewBinding(key.WithKeys("shift+up"), key.WithHelp("⇧↑/⇧↓", "reorder")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *checklistView) Init() tea.Cmd {
	return v.loadTasks()
}

func (v *checklistView) loadTasks() tea.Cmd {
	svc := v.state.App.Tasks
	return func() tea.Msg {
		tasks, err := svc.ListActive(context.Background())
		return checklistLoadedMsg{tasks: tasks, err: err}
	}
}

func (v *checklistView) clampCursor() {
	if len(v.tasks) == 0 {
		v.cursor = 0
		return
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.tasks) {
		v.cursor = len(v.tasks) - 1
	}
}

func (v *checklistView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case checklistLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.tasks = msg.tasks
		v.clampCursor()
		return v, nil

	case refreshViewMsg:
		v.loading = true
		return v, v.loadTasks()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyShiftUp:
			if v.cursor > 0 && len(v.tasks) > 0 {
				id := v.tasks[v.cursor].ID
				v.cursor--
				return v, v.reorderCmd(id, true)
			}
			return v, nil

		case tea.KeyShiftDown:
			if v.cursor < len(v.tasks)-1 {
				id := v.tasks[v.cursor].ID
				v.cursor++
				return v, v.reorderCmd(id, false)
			}
			return v, nil
		}

		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.tasks)-1 {
				v.cursor++
			}
		case "a":
			return v, pushView(newAddTaskView(v.state))
		case " ":
			if v.cursor < len(v.tasks) {
				return v, v.markDoneCmd(v.tasks[v.cursor])
			}
		case "e":
			if v.cursor < len(v.tasks) {
				return v, pushView(newEditTaskView(v.state, v.tasks[v.cursor]))
			}
		case "x":
			if v.cursor < len(v.tasks) {
				return v, v.deleteCmd(v.tasks[v.cursor])
			}
		case "r":
			v.loading = true
			return v, v.loadTasks()
		}
	}
	return v, nil
}

func (v *checklistView) reorderCmd(id string, up bool) tea.Cmd {
	svc := v.state.App.Tasks
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if up {
			err = svc.MoveUp(ctx, id)
		} else {
			err = svc.MoveDown(ctx, id)
		}
		if err != nil {
			return checklistLoadedMsg{err: err}
		}
		tasks, err := svc.ListActive(ctx)
		return checklistLoadedMsg{tasks: tasks, err: err}
	}
}

func (v *checklistView) markDoneCmd(t *domain.Task) tea.Cmd {
	svc := v.state.App.Tasks
	title := t.Title
	return func() tea.Msg {
		ctx := context.Background()
		if err := svc.MarkDone(ctx, t.ID); err != nil {
			return checklistLoadedMsg{err: err}
		}
		tasks, err := svc.ListActive(ctx)
		if err != nil {
			return checklistLoadedMsg{err: err}
		}
		_ = title // used below via closure capture in a separate output
		return checklistLoadedMsg{tasks: tasks}
	}
}

func (v *checklistView) deleteCmd(t *domain.Task) tea.Cmd {
	svc := v.state.App.Tasks
	return func() tea.Msg {
		ctx := context.Background()
		if err := svc.Delete(ctx, t.ID); err != nil {
			return checklistLoadedMsg{err: err}
		}
		tasks, err := svc.ListActive(ctx)
		return checklistLoadedMsg{tasks: tasks, err: err}
	}
}

func (v *checklistView) View() string {
	if v.loading {
		return formatter.Dim("Loading…")
	}
	if v.err != nil {
		return formatter.StyleRed.Render("Error: " + v.err.Error())
	}

	var b strings.Builder
	b.WriteString(formatter.Header("tasks"))
	b.WriteString("\n\n")

	if len(v.tasks) == 0 {
		b.WriteString(formatter.Dim("No tasks. Press 'a' to add one."))
		return b.String()
	}

	for i, t := range v.tasks {
		cursor := "  "
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
		}

		title := t.Title
		if i == v.cursor {
			title = formatter.Bold(title)
		}
		b.WriteString(fmt.Sprintf("%s[ ] %s\n", cursor, title))

		if t.Description != "" {
			b.WriteString(fmt.Sprintf("    %s\n", formatter.Dim(t.Description)))
		}
	}

	b.WriteString("\n")
	b.WriteString(formatter.Dim("a add  space done  e edit  x delete  ⇧↑/⇧↓ reorder  esc back"))
	return b.String()
}

// newAddTaskView creates a wizard form for adding a new global task.
func newAddTaskView(state *SharedState) View {
	var title, desc string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Task").
				Placeholder("What needs doing?").
				Value(&title).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description (optional)").
				Placeholder("More detail…").
				Value(&desc),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		_, err := state.App.Tasks.Add(context.Background(), service.AddTaskRequest{
			Title:       title,
			Description: desc,
		})
		if err != nil {
			return func() tea.Msg { return formErrorOutput(err) }
		}
		return nil // refreshed checklist is the confirmation
	}

	return newWizardView(state, "Add Task", form, done)
}

// newEditTaskView creates a wizard form for editing an existing task.
func newEditTaskView(state *SharedState, t *domain.Task) View {
	title := t.Title
	desc := t.Description
	id := t.ID

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Task").
				Value(&title).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description (optional)").
				Value(&desc),
		),
	).WithTheme(kairosHuhTheme()).WithShowHelp(false)

	done := func() tea.Cmd {
		if err := state.App.Tasks.Update(context.Background(), id, title, desc); err != nil {
			return func() tea.Msg { return formErrorOutput(err) }
		}
		return func() tea.Msg {
			return formSuccessOutput(fmt.Sprintf("%s Updated: %s",
				formatter.StyleGreen.Render("✔"),
				formatter.Bold(title)))
		}
	}

	return newWizardView(state, "Edit Task", form, done)
}

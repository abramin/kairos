package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// recommendationLoadedMsg signals that what-now data has been loaded.
type recommendationLoadedMsg struct {
	resp *contract.WhatNowResponse
	err  error
}

// recommendationView shows interactive what-now results.
type recommendationView struct {
	state   *SharedState
	minutes int
	resp    *contract.WhatNowResponse
	cursor  int
	loading bool
	err     error
}

func newRecommendationView(state *SharedState, minutes int) *recommendationView {
	return &recommendationView{
		state:   state,
		minutes: minutes,
		loading: true,
	}
}

func (v *recommendationView) ID() ViewID { return ViewRecommendation }
func (v *recommendationView) Title() string {
	return fmt.Sprintf("What Now (%dm)", v.minutes)
}

func (v *recommendationView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "actions")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *recommendationView) Init() tea.Cmd {
	return v.loadRecommendations()
}

func (v *recommendationView) loadRecommendations() tea.Cmd {
	app := v.state.App
	minutes := v.minutes
	return func() tea.Msg {
		ctx := context.Background()
		req := contract.NewWhatNowRequest(minutes)
		resp, err := app.WhatNow.Recommend(ctx, req)
		return recommendationLoadedMsg{resp: resp, err: err}
	}
}

func (v *recommendationView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case recommendationLoadedMsg:
		v.loading = false
		if msg.err != nil {
			v.err = msg.err
			return v, nil
		}
		v.resp = msg.resp
		return v, nil

	case refreshViewMsg:
		v.loading = true
		v.err = nil
		return v, v.loadRecommendations()

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < v.recCount()-1 {
				v.cursor++
			}
		case "enter":
			if v.resp != nil && v.cursor < len(v.resp.Recommendations) {
				rec := v.resp.Recommendations[v.cursor]
				return v, pushView(newActionMenuView(v.state, rec.WorkItemID, rec.Title, rec.WorkItemSeq))
			}
		case "r":
			v.loading = true
			v.err = nil
			return v, v.loadRecommendations()
		}
	}
	return v, nil
}

func (v *recommendationView) recCount() int {
	if v.resp == nil {
		return 0
	}
	return len(v.resp.Recommendations)
}

func (v *recommendationView) View() string {
	if v.loading {
		return "\n  " + formatter.Dim("Computing recommendations...")
	}
	if v.err != nil {
		return "\n  " + formatter.StyleRed.Render("Error: "+v.err.Error())
	}
	if v.resp == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Mode badge
	b.WriteString("  " + formatter.ModeBadge(v.resp.Mode) + "\n")

	// Allocation summary
	b.WriteString(fmt.Sprintf("  %s requested  %s allocated  %s free\n\n",
		formatter.Bold(fmt.Sprintf("%dm", v.resp.RequestedMin)),
		formatter.StyleGreen.Render(fmt.Sprintf("%dm", v.resp.AllocatedMin)),
		formatter.Dim(fmt.Sprintf("%dm", v.resp.UnallocatedMin)),
	))

	if len(v.resp.Recommendations) == 0 {
		b.WriteString("  " + formatter.Dim("No recommendations available.") + "\n")
		if len(v.resp.Blockers) > 0 {
			b.WriteString("\n  " + formatter.StyleYellow.Render("Blockers:") + "\n")
			for _, bl := range v.resp.Blockers {
				b.WriteString("  " + formatter.Dim("• "+bl.Message) + "\n")
			}
		}
		return b.String()
	}

	for i, rec := range v.resp.Recommendations {
		cursor := "  "
		if i == v.cursor {
			cursor = formatter.StyleGreen.Render("▸ ")
		}

		// Primary reason
		reason := ""
		if len(rec.Reasons) > 0 {
			reason = formatter.Dim(" — " + rec.Reasons[0].Message)
		}

		b.WriteString(fmt.Sprintf("%s%s  %s  %s%s\n",
			cursor,
			formatter.Bold(rec.Title),
			formatter.StyleGreen.Render(fmt.Sprintf("%dm", rec.AllocatedMin)),
			formatter.Dim(fmt.Sprintf("%.0f pts", rec.Score)),
			reason,
		))
	}

	return b.String()
}

package cli

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewID identifies each type of view in the TUI.
type ViewID int

const (
	ViewDashboard ViewID = iota
	ViewProjectList
	ViewTaskList
	ViewActionMenu
	ViewRecommendation
	ViewForm
	ViewDraft
	ViewHelpChat
)

// View is the interface that all TUI views must implement.
// It extends tea.Model with navigation and help metadata.
type View interface {
	tea.Model
	ID() ViewID
	ShortHelp() []key.Binding // key hints shown in the bottom bar
	Title() string            // breadcrumb segment for this view
}

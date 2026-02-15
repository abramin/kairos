package cli

import (
	"context"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SharedState holds context shared across all views via pointer.
type SharedState struct {
	App *App

	// Active project context
	ActiveProjectID   string
	ActiveShortID     string
	ActiveProjectName string

	// Active work item context
	ActiveItemID    string
	ActiveItemTitle string
	ActiveItemSeq   int

	// Session defaults
	LastDuration int

	// Terminal dimensions
	Width  int
	Height int

	// Project cache for suggestions
	Cache *shellProjectCache

	// Transient recommendation context
	LastRecommendedItemID    string
	LastRecommendedItemTitle string
	LastInspectedProjectID   string
}

// ClearProjectContext resets the active project and item state.
func (s *SharedState) ClearProjectContext() {
	s.ActiveProjectID = ""
	s.ActiveShortID = ""
	s.ActiveProjectName = ""
	s.ActiveItemID = ""
	s.ActiveItemTitle = ""
	s.ActiveItemSeq = 0
}

// ClearItemContext resets only the active item state.
func (s *SharedState) ClearItemContext() {
	s.ActiveItemID = ""
	s.ActiveItemTitle = ""
	s.ActiveItemSeq = 0
}

// SetActiveProject resolves a project ID and sets the active project context.
func (s *SharedState) SetActiveProject(ctx context.Context, projectID string) {
	p, err := s.App.Projects.GetByID(ctx, projectID)
	if err != nil {
		return
	}
	s.SetActiveProjectFrom(p)
}

// SetActiveProjectFrom sets the active project context from an already-loaded project.
func (s *SharedState) SetActiveProjectFrom(p *domain.Project) {
	s.ActiveProjectID = p.ID
	s.ActiveShortID = p.DisplayID()
	s.ActiveProjectName = p.Name
}

// SetActiveItem sets the active work item context.
func (s *SharedState) SetActiveItem(id, title string, seq int) {
	s.ActiveItemID = id
	s.ActiveItemTitle = title
	s.ActiveItemSeq = seq
}

// ContentHeight returns the available height for view content,
// accounting for header (2 lines: title + separator),
// status bar (2 lines: separator + hints), and command bar (1 line).
func (s *SharedState) ContentHeight() int {
	h := s.Height - 5
	if h < 1 {
		return 1
	}
	return h
}

package cli

import (
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/spf13/cobra"
)

// App holds references to all service interfaces used by CLI commands.
type App struct {
	// v1 services
	Projects  service.ProjectService
	Nodes     service.NodeService
	WorkItems service.WorkItemService
	Sessions  service.SessionService
	WhatNow   service.WhatNowService
	Status    service.StatusService
	Replan    service.ReplanService
	Templates service.TemplateService

	// v2 intelligence services (nil when LLM disabled)
	Intent        intelligence.IntentService
	Explain       intelligence.ExplainService
	TemplateDraft intelligence.TemplateDraftService
}

// NewRootCmd creates the top-level "kairos" command and registers all
// subcommands against the provided App.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "kairos",
		Short: "Project planner and session recommender",
	}

	root.AddCommand(
		newProjectCmd(app),
		newNodeCmd(app),
		newWorkCmd(app),
		newSessionCmd(app),
		newWhatNowCmd(app),
		newStatusCmd(app),
		newReplanCmd(app),
		newTemplateCmd(app),
		// v2 intelligence commands
		newAskCmd(app),
		newExplainCmd(app),
		newReviewCmd(app),
	)

	return root
}

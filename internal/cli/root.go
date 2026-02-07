package cli

import (
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/spf13/cobra"
)

// App holds references to all service interfaces used by CLI commands.
type App struct {
	Projects  service.ProjectService
	Nodes     service.NodeService
	WorkItems service.WorkItemService
	Sessions  service.SessionService
	WhatNow   service.WhatNowService
	Status    service.StatusService
	Replan    service.ReplanService
	Templates service.TemplateService
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
	)

	return root
}

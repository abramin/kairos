package cli

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/alexanderramin/kairos/internal/contract"
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
	Import    service.ImportService

	// v2 intelligence services (nil when LLM disabled)
	Intent        intelligence.IntentService
	Explain       intelligence.ExplainService
	TemplateDraft intelligence.TemplateDraftService
	ProjectDraft  intelligence.ProjectDraftService
	Help          intelligence.HelpService

	// Cached command spec (populated lazily by getCommandSpec).
	cmdSpec     *CommandSpec
	cmdSpecOnce sync.Once
}

// NewRootCmd creates the top-level "kairos" command and registers all
// subcommands against the provided App.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "kairos [minutes]",
		Short: "Project planner and session recommender",
		Long: `Project planner and session recommender.

Quick usage: kairos <minutes> is shorthand for kairos what-now --minutes <minutes>`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			minutes, err := strconv.Atoi(args[0])
			if err != nil || minutes <= 0 {
				return fmt.Errorf("invalid minutes %q — expected a positive integer", args[0])
			}
			req := contract.NewWhatNowRequest(minutes)
			resp, err := app.WhatNow.Recommend(context.Background(), req)
			if err != nil {
				return err
			}
			fmt.Print(formatWhatNowResponse(context.Background(), app, resp))
			return nil
		},
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
		newShellCmd(app),
		// v2 intelligence commands
		newAskCmd(app),
		newExplainCmd(app),
		newReviewCmd(app),
	)

	// Replace Cobra's auto-generated help command with our custom one
	// that adds `help chat` while preserving default help behavior.
	root.SetHelpCommand(newHelpCmd(app, root))

	return root
}

// getCommandSpec lazily builds and caches the CommandSpec from the Cobra tree.
func (a *App) getCommandSpec(root *cobra.Command) *CommandSpec {
	a.cmdSpecOnce.Do(func() {
		a.cmdSpec = BuildCommandSpec(root)
	})
	return a.cmdSpec
}

package cli

import (
	"fmt"
	"sync"

	"github.com/alexanderramin/kairos/internal/app"
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
	WhatNow   app.WhatNowUseCase
	Status    app.StatusUseCase
	Replan    app.ReplanUseCase
	Templates service.TemplateService
	Import    service.ImportService

	// Phase 1 app ports with CLI-level fallback to legacy service fields.
	LogSession    app.LogSessionUseCase
	InitProject   app.InitProjectUseCase
	ImportProject app.ImportProjectUseCase

	// v2 intelligence services (nil when LLM disabled)
	Intent        intelligence.IntentService
	Explain       intelligence.ExplainService
	TemplateDraft intelligence.TemplateDraftService
	ProjectDraft  intelligence.ProjectDraftService
	Help          intelligence.HelpService

	// IsInteractive reports whether stdin is a terminal.
	// Set by main; tests override to return false.
	IsInteractive func() bool

	// Cached command spec (populated lazily by getCommandSpec).
	cmdSpec     *CommandSpec
	cmdSpecOnce sync.Once
}

// NewRootCmd creates the top-level "kairos" command and registers all
// subcommands against the provided App.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "kairos",
		Short: "Project planner and session recommender",
		Long:  `Project planner and session recommender. Launches an interactive shell.`,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.IsInteractive != nil && app.IsInteractive() {
				return runShell(app)
			}
			return fmt.Errorf("kairos requires an interactive terminal")
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

package cli

import (
	"sync"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/service"
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

// getCommandSpec lazily builds and caches the static shell CommandSpec.
func (a *App) getCommandSpec() *CommandSpec {
	a.cmdSpecOnce.Do(func() {
		a.cmdSpec = ShellCommandSpec()
	})
	return a.cmdSpec
}

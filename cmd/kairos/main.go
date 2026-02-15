package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexanderramin/kairos/internal/cli"
	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/mattn/go-isatty"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Determine DB path: env var or default ~/.kairos/kairos.db
	dbPath := os.Getenv("KAIROS_DB")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".kairos", "kairos.db")
	}

	// Determine template directory
	templateDir := os.Getenv("KAIROS_TEMPLATES")
	if templateDir == "" {
		// Check for ./templates in current directory first (development)
		if stat, err := os.Stat("./templates"); err == nil && stat.IsDir() {
			templateDir = "./templates"
		} else {
			// Fall back to ~/.kairos/templates (production)
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("finding home directory: %w", err)
			}
			templateDir = filepath.Join(home, ".kairos", "templates")
		}
	}

	// Open database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	// Wire repositories
	projectRepo := repository.NewSQLiteProjectRepo(database)
	nodeRepo := repository.NewSQLitePlanNodeRepo(database)
	workItemRepo := repository.NewSQLiteWorkItemRepo(database)
	depRepo := repository.NewSQLiteDependencyRepo(database)
	sessionRepo := repository.NewSQLiteSessionRepo(database)
	profileRepo := repository.NewSQLiteUserProfileRepo(database)

	// Wire unit of work for transactional operations
	uow := db.NewSQLiteUnitOfWork(database)

	// Wire services
	sessionSvc := service.NewSessionService(sessionRepo, workItemRepo, uow)
	templateSvc := service.NewTemplateService(templateDir, projectRepo, nodeRepo, workItemRepo, depRepo, uow)
	importSvc := service.NewImportService(projectRepo, nodeRepo, workItemRepo, depRepo, uow)

	app := &cli.App{
		Projects:  service.NewProjectService(projectRepo),
		Nodes:     service.NewNodeService(nodeRepo),
		WorkItems: service.NewWorkItemService(workItemRepo, nodeRepo),
		Sessions:  sessionSvc,
		WhatNow:   service.NewWhatNowService(workItemRepo, sessionRepo, depRepo, profileRepo),
		Status:    service.NewStatusService(projectRepo, workItemRepo, sessionRepo, profileRepo),
		Replan:    service.NewReplanService(projectRepo, workItemRepo, sessionRepo, profileRepo),
		Templates: templateSvc,
		Import:    importSvc,

		LogSession:    sessionSvc,
		InitProject:   templateSvc,
		ImportProject: importSvc,
	}

	// Detect interactive terminal for shell-only entrypoint.
	app.IsInteractive = func() bool {
		return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	}

	// Wire v2 intelligence services (only when LLM is enabled)
	llmCfg := llm.LoadConfig()
	if llmCfg.Enabled {
		var observer llm.Observer = llm.NoopObserver{}
		if llmCfg.LogCalls {
			observer = llm.NewLogObserver(os.Stderr)
		}
		llmClient := llm.NewOllamaClient(llmCfg, observer)
		policy := intelligence.DefaultConfirmationPolicy(llmCfg.ConfidenceThreshold)

		app.Intent = intelligence.NewIntentService(llmClient, observer, policy)
		app.Explain = intelligence.NewExplainService(llmClient, observer)
		app.TemplateDraft = intelligence.NewTemplateDraftService(llmClient, observer)
		app.ProjectDraft = intelligence.NewProjectDraftService(llmClient, observer)
		app.Help = intelligence.NewHelpService(llmClient, observer)
	}

	// Execute root command
	rootCmd := cli.NewRootCmd(app)
	return rootCmd.Execute()
}

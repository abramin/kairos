package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexanderramin/kairos/internal/cli"
	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/service"
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
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		templateDir = filepath.Join(home, ".kairos", "templates")
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

	// Wire services
	app := &cli.App{
		Projects:  service.NewProjectService(projectRepo),
		Nodes:     service.NewNodeService(nodeRepo),
		WorkItems: service.NewWorkItemService(workItemRepo),
		Sessions:  service.NewSessionService(sessionRepo, workItemRepo),
		WhatNow:   service.NewWhatNowService(workItemRepo, sessionRepo, projectRepo, depRepo, profileRepo),
		Status:    service.NewStatusService(projectRepo, workItemRepo, sessionRepo, profileRepo),
		Replan:    service.NewReplanService(projectRepo, workItemRepo, sessionRepo, profileRepo),
		Templates: service.NewTemplateService(templateDir, projectRepo, nodeRepo, workItemRepo, depRepo),
	}

	// Execute root command
	rootCmd := cli.NewRootCmd(app)
	return rootCmd.Execute()
}

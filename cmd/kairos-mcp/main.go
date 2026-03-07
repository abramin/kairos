// kairos-mcp is a Model Context Protocol server for Kairos.
//
// It exposes Kairos work items and projects as MCP tools over stdio/JSON-RPC 2.0.
// Claude Code, configured with both this server and a Google Calendar MCP server,
// can read due-date items from Kairos and push them to Google Calendar on request.
//
// Usage:
//
//	kairos-mcp                    # reads KAIROS_DB env or ~/.kairos/kairos.db
//	KAIROS_DB=/path/to/db kairos-mcp
//
// Configure in Claude Code by adding to .mcp.json (see project root for example).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/mcpserver"
	"github.com/alexanderramin/kairos/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "kairos-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := os.Getenv("KAIROS_DB")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".kairos", "kairos.db")
	}

	database, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	deps := mcpserver.Deps{
		Projects:  repository.NewSQLiteProjectRepo(database),
		WorkItems: repository.NewSQLiteWorkItemRepo(database),
	}

	srv := mcpserver.NewServer(deps)
	return srv.Serve()
}

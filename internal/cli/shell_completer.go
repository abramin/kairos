package cli

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

// shellProjectCache caches the project list for autocomplete,
// refreshing at most every 5 seconds.
type shellProjectCache struct {
	mu        sync.Mutex
	projects  []*domain.Project
	fetchedAt time.Time
	ttl       time.Duration
}

func newShellProjectCache() *shellProjectCache {
	return &shellProjectCache{ttl: 5 * time.Second}
}

func (c *shellProjectCache) get(app *App) []*domain.Project {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.fetchedAt) < c.ttl && c.projects != nil {
		return c.projects
	}
	projects, err := app.Projects.List(context.Background(), false)
	if err != nil {
		return c.projects // return stale data on error
	}
	c.projects = projects
	c.fetchedAt = time.Now()
	return c.projects
}

// allCommandNames returns the full list of shell command names for autocomplete.
func allCommandNames() []string {
	return []string{
		"projects", "use", "inspect",
		"status", "what-now", "replan",
		"log", "start", "finish", "add", "context",
		"project", "node", "work", "session",
		"draft", "template",
		"ask", "explain", "review",
		"clear", "help", "exit", "quit",
	}
}

// subcommandNames returns subcommand lists by parent command.
func subcommandNames() map[string][]string {
	return map[string][]string{
		"project":  {"add", "list", "inspect", "update", "archive", "unarchive", "remove", "init", "import", "draft"},
		"node":     {"add", "inspect", "update", "remove"},
		"work":     {"add", "inspect", "update", "done", "archive", "remove"},
		"session":  {"log", "list", "remove"},
		"template": {"list", "show", "draft"},
		"explain":  {"now", "why-not"},
		"review":   {"weekly"},
	}
}

// filterSuggestions returns items from pool that start with prefix (case-insensitive).
func filterSuggestions(pool []string, prefix string) []string {
	if prefix == "" {
		return pool
	}
	lp := strings.ToLower(prefix)
	var result []string
	for _, s := range pool {
		if strings.HasPrefix(strings.ToLower(s), lp) {
			result = append(result, s)
		}
	}
	return result
}

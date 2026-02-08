package cli

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	prompt "github.com/c-bata/go-prompt"
)

// commandSuggestions are the top-level shell commands.
var commandSuggestions = []prompt.Suggest{
	{Text: "projects", Description: "List all active projects"},
	{Text: "use", Description: "Set active project context"},
	{Text: "inspect", Description: "Show project details and plan tree"},
	{Text: "status", Description: "Show project status overview"},
	{Text: "what-now", Description: "Get session recommendations"},
	{Text: "clear", Description: "Clear the screen"},
	{Text: "help", Description: "Show available commands"},
	{Text: "exit", Description: "Quit the shell"},
}

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

func (s *shellSession) completer(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	if text == "" {
		return nil
	}

	parts := strings.Fields(text)

	// Still typing the first word — suggest commands.
	if len(parts) <= 1 && !strings.HasSuffix(text, " ") {
		return prompt.FilterHasPrefix(commandSuggestions, d.GetWordBeforeCursor(), true)
	}

	// Second-word completion for commands that take arguments.
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "use", "inspect":
		return s.projectSuggestions(d.GetWordBeforeCursor())
	case "what-now":
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "30", Description: "30 minutes"},
			{Text: "45", Description: "45 minutes"},
			{Text: "60", Description: "1 hour"},
			{Text: "90", Description: "1.5 hours"},
			{Text: "120", Description: "2 hours"},
		}, d.GetWordBeforeCursor(), true)
	}

	return nil
}

func (s *shellSession) projectSuggestions(prefix string) []prompt.Suggest {
	projects := s.cache.get(s.app)
	suggestions := make([]prompt.Suggest, 0, len(projects))
	for _, p := range projects {
		id := p.ShortID
		if id == "" && len(p.ID) >= 8 {
			id = p.ID[:8]
		}
		suggestions = append(suggestions, prompt.Suggest{
			Text:        id,
			Description: p.Name,
		})
	}
	return prompt.FilterHasPrefix(suggestions, prefix, true)
}

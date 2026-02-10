package cli

import (
	"context"
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

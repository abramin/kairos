package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterSuggestions_MatchesPrefix(t *testing.T) {
	pool := []string{"projects", "project", "status", "start"}
	got := filterSuggestions(pool, "pro")
	assert.Equal(t, []string{"projects", "project"}, got)
}

func TestFilterSuggestions_EmptyPrefixReturnsAll(t *testing.T) {
	pool := []string{"projects", "status"}
	got := filterSuggestions(pool, "")
	assert.Equal(t, pool, got)
}

func TestFilterSuggestions_CaseInsensitive(t *testing.T) {
	pool := []string{"Projects", "status"}
	got := filterSuggestions(pool, "proj")
	assert.Equal(t, []string{"Projects"}, got)
}

func TestFilterSuggestions_NoMatch(t *testing.T) {
	pool := []string{"projects", "status"}
	got := filterSuggestions(pool, "xyz")
	assert.Nil(t, got)
}

func TestAllCommandNames_ContainsNewCommands(t *testing.T) {
	names := allCommandNames()
	has := func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
	assert.True(t, has("log"), "should have log")
	assert.True(t, has("start"), "should have start")
	assert.True(t, has("finish"), "should have finish")
	assert.True(t, has("context"), "should have context")
	assert.True(t, has("projects"), "should have projects")
	assert.True(t, has("what-now"), "should have what-now")
	assert.True(t, has("draft"), "should have draft")
}

func TestSubcommandNames_HasExpectedGroups(t *testing.T) {
	subs := subcommandNames()
	assert.Contains(t, subs, "project")
	assert.Contains(t, subs, "node")
	assert.Contains(t, subs, "work")
	assert.Contains(t, subs, "session")
	assert.Contains(t, subs, "explain")
}

func TestShellProjectCache_ReturnsProjects(t *testing.T) {
	app := testApp(t)
	_, err := executeCmd(t, app, "project", "add",
		"--id", "CMP01",
		"--name", "Autocomplete Project",
		"--domain", "education",
		"--start", "2026-01-15",
	)
	require.NoError(t, err)

	cache := newShellProjectCache()
	projects := cache.get(app)
	require.Len(t, projects, 1)
	assert.Equal(t, "CMP01", projects[0].ShortID)
}

func TestShellProjectCache_FallsBackToUUIDPrefix(t *testing.T) {
	app := testApp(t)
	_, err := executeCmd(t, app, "project", "add",
		"--id", "CMP02",
		"--name", "No ShortID Project",
		"--domain", "education",
		"--start", "2026-01-15",
	)
	require.NoError(t, err)

	projects, err := app.Projects.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	projects[0].ShortID = ""
	require.NoError(t, app.Projects.Update(context.Background(), projects[0]))

	m := newShellModel(app)
	suggestions := m.projectSuggestionsPlain("")
	require.NotEmpty(t, suggestions)
	assert.Equal(t, projects[0].ID[:8], suggestions[0])
}

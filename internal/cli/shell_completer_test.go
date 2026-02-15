package cli

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
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
	ctx := context.Background()

	proj := testutil.NewTestProject("Autocomplete Project", testutil.WithShortID("CMP01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	cache := newShellProjectCache()
	projects := cache.get(app)
	require.Len(t, projects, 1)
	assert.Equal(t, "CMP01", projects[0].ShortID)
}

func TestShellProjectCache_FallsBackToUUIDPrefix(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("No ShortID Project", testutil.WithShortID("CMP02"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	projects, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	projects[0].ShortID = ""
	require.NoError(t, app.Projects.Update(ctx, projects[0]))

	// Test via commandBar projectSuggestions.
	cb := testCommandBar(t, app)
	suggestions := cb.projectSuggestions("")
	require.NotEmpty(t, suggestions)
	assert.Equal(t, projects[0].ID[:8], suggestions[0])
}

func TestCommandBar_UpdateSuggestions_PrunesExactMatch(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cb.input.SetValue("add")
	cb.updateSuggestions()

	assert.Empty(t, cb.input.MatchedSuggestions(), "exact match should not remain as a suggestion")
}

func TestCommandBar_UpdateSuggestions_UsesFullLineForSubcommands(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	cb.input.SetValue("project a")
	cb.updateSuggestions()

	assert.Contains(t, cb.input.MatchedSuggestions(), "project add")
}

func TestCommandBar_TabCompletion_StaysInteractiveAfterExactMatch(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)
	cb.Focus()

	cb.input.SetValue("a")
	cb.updateSuggestions()
	require.Contains(t, cb.input.MatchedSuggestions(), "add")

	_ = cb.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "add", cb.input.Value())
	assert.Empty(t, cb.input.MatchedSuggestions(), "cursor should remain visible after completing exact match")

	_ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	_ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, "add x", cb.input.Value())
}

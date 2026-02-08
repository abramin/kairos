package cli

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	prompt "github.com/c-bata/go-prompt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func docWithCursorAtEnd(text string) prompt.Document {
	d := prompt.Document{Text: text}
	v := reflect.ValueOf(&d).Elem().FieldByName("cursorPosition")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetInt(int64(len([]rune(text))))
	return d
}

func hasSuggestion(suggestions []prompt.Suggest, want string) bool {
	for _, s := range suggestions {
		if s.Text == want {
			return true
		}
	}
	return false
}

func TestShellLivePrefix(t *testing.T) {
	sess := &shellSession{}

	prefix, ok := sess.livePrefix()
	require.True(t, ok)
	assert.Equal(t, "kairos ❯ ", prefix)

	sess.activeProjectID = "project-1"
	sess.activeShortID = "ABC01"
	prefix, ok = sess.livePrefix()
	require.True(t, ok)
	assert.Equal(t, "kairos (ABC01) ❯ ", prefix)

	sess.helpChatMode = true
	prefix, ok = sess.livePrefix()
	require.True(t, ok)
	assert.Equal(t, "help> ", prefix)
}

func TestShellCompleter_CommandSuggestions(t *testing.T) {
	sess := &shellSession{}
	suggestions := sess.completer(docWithCursorAtEnd("wh"))
	assert.True(t, hasSuggestion(suggestions, "what-now"))
}

func TestShellCompleter_HelpChatSuggestion(t *testing.T) {
	sess := &shellSession{}
	suggestions := sess.completer(docWithCursorAtEnd("help ch"))
	assert.True(t, hasSuggestion(suggestions, "chat"))
}

func TestShellCompleter_HelpChatModeSlashSuggestions(t *testing.T) {
	sess := &shellSession{helpChatMode: true}
	suggestions := sess.completer(docWithCursorAtEnd("/c"))
	assert.True(t, hasSuggestion(suggestions, "/commands"))
}

func TestShellCompleter_ProjectSuggestionsForUse(t *testing.T) {
	app := testApp(t)
	_, err := executeCmd(t, app, "project", "add",
		"--id", "CMP01",
		"--name", "Autocomplete Project",
		"--domain", "education",
		"--start", "2026-01-15",
	)
	require.NoError(t, err)

	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}
	suggestions := sess.completer(docWithCursorAtEnd("use CM"))
	assert.True(t, hasSuggestion(suggestions, "CMP01"))
}

func TestShellProjectSuggestions_FallsBackToUUIDPrefix(t *testing.T) {
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

	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}
	suggestions := sess.projectSuggestions("")
	require.NotEmpty(t, suggestions)
	assert.Equal(t, projects[0].ID[:8], suggestions[0].Text)
}

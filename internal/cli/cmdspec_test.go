package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellCommandSpec_PopulatesCommands(t *testing.T) {
	spec := ShellCommandSpec()

	require.NotNil(t, spec)
	assert.True(t, len(spec.Commands) > 0, "spec should have commands")

	// Verify top-level commands exist.
	assert.True(t, spec.ValidateCommandPath("projects"))
	assert.True(t, spec.ValidateCommandPath("status"))
	assert.True(t, spec.ValidateCommandPath("what-now"))
	assert.True(t, spec.ValidateCommandPath("help"))
	assert.True(t, spec.ValidateCommandPath("replan"))
	assert.True(t, spec.ValidateCommandPath("import"))

	// Verify subcommands exist.
	assert.True(t, spec.ValidateCommandPath("project list"))
	assert.True(t, spec.ValidateCommandPath("project add"))
	assert.True(t, spec.ValidateCommandPath("session log"))
	assert.True(t, spec.ValidateCommandPath("help chat"))
}

func TestShellCommandSpec_IncludesFlags(t *testing.T) {
	spec := ShellCommandSpec()

	cmd := spec.FindCommand("project add")
	require.NotNil(t, cmd)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		flagNames[f.Name] = true
	}

	assert.True(t, flagNames["id"], "should have --id flag")
	assert.True(t, flagNames["name"], "should have --name flag")
	assert.True(t, flagNames["domain"], "should have --domain flag")
}

func TestCommandSpec_ValidateCommandPath(t *testing.T) {
	spec := ShellCommandSpec()

	assert.True(t, spec.ValidateCommandPath("project add"))
	assert.False(t, spec.ValidateCommandPath("nonexistent"))
	assert.False(t, spec.ValidateCommandPath(""))
}

func TestCommandSpec_ValidateFlag(t *testing.T) {
	spec := ShellCommandSpec()

	assert.True(t, spec.ValidateFlag("project add", "name"))
	assert.True(t, spec.ValidateFlag("project add", "domain"))
	assert.False(t, spec.ValidateFlag("project add", "nonexistent"))
	assert.False(t, spec.ValidateFlag("nonexistent", "name"))
}

func TestCommandSpec_FindCommand(t *testing.T) {
	spec := ShellCommandSpec()

	cmd := spec.FindCommand("session log")
	require.NotNil(t, cmd)
	assert.Equal(t, "Log a work session", cmd.Short)

	assert.Nil(t, spec.FindCommand("doesnotexist"))
}

func TestCommandSpec_FuzzyMatch(t *testing.T) {
	spec := ShellCommandSpec()

	matches := spec.FuzzyMatch("log session", 3)
	require.True(t, len(matches) > 0, "should find matches for 'log session'")

	found := false
	for _, m := range matches {
		if m.FullPath == "session log" {
			found = true
			break
		}
	}
	assert.True(t, found, "should match 'session log'")
}

func TestCommandSpec_FuzzyMatch_NoResults(t *testing.T) {
	spec := ShellCommandSpec()

	matches := spec.FuzzyMatch("xyznonexistent", 3)
	assert.Empty(t, matches)
}

func TestCommandSpec_FuzzyMatch_EmptyQuery(t *testing.T) {
	spec := ShellCommandSpec()

	matches := spec.FuzzyMatch("", 3)
	assert.Empty(t, matches)
}

func TestSerializeCommandSpec(t *testing.T) {
	spec := ShellCommandSpec()

	json := SerializeCommandSpec(spec)
	assert.Contains(t, json, "project add")
	assert.Contains(t, json, "full_path")
}

func TestBuildValidationMaps(t *testing.T) {
	spec := ShellCommandSpec()

	cmds, flags := BuildValidationMaps(spec)

	assert.True(t, cmds["project add"])
	assert.False(t, cmds["nonexistent"])

	require.NotNil(t, flags["project add"])
	assert.True(t, flags["project add"]["name"])
	assert.False(t, flags["project add"]["nonexistent"])
}

func TestShellCommandSpec_ContainsExpectedCommands(t *testing.T) {
	spec := ShellCommandSpec()

	expected := []string{
		"projects",
		"status",
		"what-now",
		"replan",
		"import",
		"log",
		"start",
		"finish",
		"add",
		"project list",
		"project add",
		"project inspect",
		"node add",
		"work add",
		"session log",
		"template list",
		"help chat",
		"explain now",
		"review weekly",
	}

	for _, cmd := range expected {
		assert.True(t, spec.ValidateCommandPath(cmd), "missing command path: %s", cmd)
	}
}

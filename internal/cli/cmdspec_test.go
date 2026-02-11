package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kairos",
		Short: "Test CLI",
	}

	project := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a project",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	addCmd.Flags().String("name", "", "Project name")
	addCmd.Flags().String("domain", "", "Project domain")
	addCmd.Flags().Bool("force", false, "Force creation")
	_ = addCmd.MarkFlagRequired("name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	listCmd.Flags().Bool("all", false, "Include archived")

	project.AddCommand(addCmd, listCmd)

	session := &cobra.Command{
		Use:   "session",
		Short: "Manage sessions",
	}
	logCmd := &cobra.Command{
		Use:   "log",
		Short: "Log a work session",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	logCmd.Flags().String("work-item", "", "Work item ID")
	logCmd.Flags().Int("minutes", 0, "Minutes spent")
	session.AddCommand(logCmd)

	status := &cobra.Command{
		Use:   "status",
		Short: "Show project status overview",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}

	whatNow := &cobra.Command{
		Use:   "what-now",
		Short: "Get session recommendations",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	whatNow.Flags().Int("minutes", 60, "Available minutes")

	help := &cobra.Command{
		Use:   "help",
		Short: "Help about any command",
	}
	helpChat := &cobra.Command{
		Use:   "chat",
		Short: "Interactive LLM-powered help",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	help.AddCommand(helpChat)

	root.AddCommand(project, session, status, whatNow, help)
	return root
}

func TestBuildCommandSpec_PopulatesCommands(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	require.NotNil(t, spec)
	assert.True(t, len(spec.Commands) > 0, "spec should have commands")

	// Verify top-level commands exist.
	assert.True(t, spec.ValidateCommandPath("kairos project"))
	assert.True(t, spec.ValidateCommandPath("kairos session"))
	assert.True(t, spec.ValidateCommandPath("kairos status"))
	assert.True(t, spec.ValidateCommandPath("kairos what-now"))
	assert.True(t, spec.ValidateCommandPath("kairos help"))

	// Verify subcommands exist.
	assert.True(t, spec.ValidateCommandPath("kairos project add"))
	assert.True(t, spec.ValidateCommandPath("kairos project list"))
	assert.True(t, spec.ValidateCommandPath("kairos session log"))
	assert.True(t, spec.ValidateCommandPath("kairos help chat"))
}

func TestBuildCommandSpec_IncludesFlags(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	cmd := spec.FindCommand("kairos project add")
	require.NotNil(t, cmd)

	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		flagNames[f.Name] = true
	}

	assert.True(t, flagNames["name"], "should have --name flag")
	assert.True(t, flagNames["domain"], "should have --domain flag")
	assert.True(t, flagNames["force"], "should have --force flag")
}

func TestBuildCommandSpec_SubcommandPaths(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	project := spec.FindCommand("kairos project")
	require.NotNil(t, project)
	assert.Contains(t, project.Subcommands, "kairos project add")
	assert.Contains(t, project.Subcommands, "kairos project list")
}

func TestCommandSpec_ValidateCommandPath(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	assert.True(t, spec.ValidateCommandPath("kairos project add"))
	assert.False(t, spec.ValidateCommandPath("kairos nonexistent"))
	assert.False(t, spec.ValidateCommandPath(""))
}

func TestCommandSpec_ValidateFlag(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	assert.True(t, spec.ValidateFlag("kairos project add", "name"))
	assert.True(t, spec.ValidateFlag("kairos project add", "domain"))
	assert.False(t, spec.ValidateFlag("kairos project add", "nonexistent"))
	assert.False(t, spec.ValidateFlag("kairos nonexistent", "name"))
}

func TestCommandSpec_FindCommand(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	cmd := spec.FindCommand("kairos session log")
	require.NotNil(t, cmd)
	assert.Equal(t, "Log a work session", cmd.Short)

	assert.Nil(t, spec.FindCommand("kairos doesnotexist"))
}

func TestCommandSpec_FuzzyMatch(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	matches := spec.FuzzyMatch("log session", 3)
	require.True(t, len(matches) > 0, "should find matches for 'log session'")

	// "kairos session log" should be among the top results.
	found := false
	for _, m := range matches {
		if m.FullPath == "kairos session log" {
			found = true
			break
		}
	}
	assert.True(t, found, "should match 'kairos session log'")
}

func TestCommandSpec_FuzzyMatch_NoResults(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	matches := spec.FuzzyMatch("xyznonexistent", 3)
	assert.Empty(t, matches)
}

func TestCommandSpec_FuzzyMatch_EmptyQuery(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	matches := spec.FuzzyMatch("", 3)
	assert.Empty(t, matches)
}

func TestSerializeCommandSpec(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	json := SerializeCommandSpec(spec)
	assert.Contains(t, json, "kairos project add")
	assert.Contains(t, json, "full_path")
}

func TestBuildValidationMaps(t *testing.T) {
	root := newTestRootCmd()
	spec := BuildCommandSpec(root)

	cmds, flags := BuildValidationMaps(spec)

	assert.True(t, cmds["kairos project add"])
	assert.False(t, cmds["kairos nonexistent"])

	require.NotNil(t, flags["kairos project add"])
	assert.True(t, flags["kairos project add"]["name"])
	assert.False(t, flags["kairos project add"]["nonexistent"])
}

func TestBuildCommandSpec_RealRootContainsExpectedCommands(t *testing.T) {
	app := testApp(t)
	root := NewRootCmd(app)
	spec := BuildCommandSpec(root)

	expected := []string{
		"kairos project",
		"kairos node",
		"kairos work",
		"kairos session",
		"kairos what-now",
		"kairos status",
		"kairos replan",
		"kairos template",
		"kairos ask",
		"kairos explain",
		"kairos review",
	}

	for _, cmd := range expected {
		assert.True(t, spec.ValidateCommandPath(cmd), "missing command path: %s", cmd)
	}
}

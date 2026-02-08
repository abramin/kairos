package intelligence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHelpCommands() []HelpCommandInfo {
	return []HelpCommandInfo{
		{FullPath: "kairos status", Short: "Show project status"},
		{FullPath: "kairos what-now", Short: "Get recommendations"},
		{FullPath: "kairos session log", Short: "Log a work session"},
		{FullPath: "kairos help", Short: "Show help"},
	}
}

func TestDeterministicHelp_CommandMatch(t *testing.T) {
	answer := DeterministicHelp("how do I log a session?", testHelpCommands())

	assert.Equal(t, "deterministic", answer.Source)
	assert.Equal(t, 1.0, answer.Confidence)
	require.NotEmpty(t, answer.Examples)
	found := false
	for _, ex := range answer.Examples {
		if ex.Command == "kairos session log" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected session log example in deterministic fallback")
}

func TestDeterministicHelp_GlossaryMatch(t *testing.T) {
	answer := DeterministicHelp("what is session policy?", testHelpCommands())

	assert.Contains(t, answer.Answer, "session_policy:")
	assert.Equal(t, "deterministic", answer.Source)
}

func TestDeterministicHelp_DefaultForEmptyQuestion(t *testing.T) {
	answer := DeterministicHelp("   ", testHelpCommands())

	assert.Equal(t, "deterministic", answer.Source)
	assert.Equal(t, 1.0, answer.Confidence)
	assert.Contains(t, answer.Answer, "common commands")
	require.NotEmpty(t, answer.NextCommands)
}

package intelligence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateHelpGrounding_ValidCommands(t *testing.T) {
	validCmds := map[string]bool{
		"kairos status":      true,
		"kairos session log": true,
		"kairos what-now":    true,
	}
	validFlags := map[string]map[string]bool{
		"kairos session log": {"work-item": true, "minutes": true},
		"kairos what-now":    {"minutes": true},
	}

	answer := &HelpAnswer{
		Answer: "Use session log to record time.",
		Examples: []ShellExample{
			{Command: "kairos session log --work-item abc --minutes 30", Description: "Log 30 min"},
			{Command: "kairos status", Description: "Check status"},
		},
		NextCommands: []string{"kairos what-now", "kairos status"},
		Confidence:   0.9,
		Source:       "llm",
	}

	result, anyRemoved := ValidateHelpGrounding(answer, validCmds, validFlags)
	assert.False(t, anyRemoved)
	assert.Len(t, result.Examples, 2)
	assert.Len(t, result.NextCommands, 2)
}

func TestValidateHelpGrounding_InvalidCommandStripped(t *testing.T) {
	validCmds := map[string]bool{
		"kairos status": true,
	}
	validFlags := map[string]map[string]bool{}

	answer := &HelpAnswer{
		Examples: []ShellExample{
			{Command: "kairos status", Description: "Valid"},
			{Command: "kairos fabricated command", Description: "Invalid"},
		},
		NextCommands: []string{"kairos status", "kairos does-not-exist"},
		Confidence:   0.8,
		Source:       "llm",
	}

	result, anyRemoved := ValidateHelpGrounding(answer, validCmds, validFlags)
	assert.True(t, anyRemoved)
	assert.Len(t, result.Examples, 1)
	assert.Equal(t, "kairos status", result.Examples[0].Command)
	assert.Len(t, result.NextCommands, 1)
	assert.Equal(t, "kairos status", result.NextCommands[0])
}

func TestValidateHelpGrounding_InvalidFlagStripped(t *testing.T) {
	validCmds := map[string]bool{
		"kairos session log": true,
	}
	validFlags := map[string]map[string]bool{
		"kairos session log": {"work-item": true, "minutes": true},
	}

	answer := &HelpAnswer{
		Examples: []ShellExample{
			{Command: "kairos session log --work-item abc --minutes 30", Description: "Valid"},
			{Command: "kairos session log --fake-flag value", Description: "Invalid flag"},
		},
		Confidence: 0.8,
		Source:     "llm",
	}

	result, anyRemoved := ValidateHelpGrounding(answer, validCmds, validFlags)
	assert.True(t, anyRemoved)
	assert.Len(t, result.Examples, 1)
	assert.Contains(t, result.Examples[0].Command, "--work-item")
}

func TestValidateHelpGrounding_AllInvalid(t *testing.T) {
	validCmds := map[string]bool{}
	validFlags := map[string]map[string]bool{}

	answer := &HelpAnswer{
		Examples: []ShellExample{
			{Command: "kairos fake1", Description: "Bad"},
			{Command: "kairos fake2", Description: "Bad"},
		},
		NextCommands: []string{"kairos fake3"},
		Confidence:   0.5,
		Source:       "llm",
	}

	result, anyRemoved := ValidateHelpGrounding(answer, validCmds, validFlags)
	assert.True(t, anyRemoved)
	assert.Empty(t, result.Examples)
	assert.Empty(t, result.NextCommands)
	assert.Equal(t, "deterministic", result.Source)
}

func TestParseCommandString(t *testing.T) {
	tests := []struct {
		input     string
		wantPath  string
		wantFlags []string
	}{
		{
			input:     "kairos session log --work-item abc --minutes 30",
			wantPath:  "kairos session log",
			wantFlags: []string{"work-item", "minutes"},
		},
		{
			input:    "kairos status",
			wantPath: "kairos status",
		},
		{
			input:     "kairos what-now --minutes=60",
			wantPath:  "kairos what-now",
			wantFlags: []string{"minutes"},
		},
		{
			input:    "",
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			path, flags := parseCommandString(tt.input)
			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantFlags, flags)
		})
	}
}

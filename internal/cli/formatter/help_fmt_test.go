package formatter

import (
	"strings"
	"testing"

	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/stretchr/testify/assert"
)

func TestFormatHelpAnswer_LLM(t *testing.T) {
	answer := &intelligence.HelpAnswer{
		Answer: "Use kairos status.",
		Examples: []intelligence.ShellExample{
			{Command: "kairos status", Description: "Show project risks"},
		},
		NextCommands: []string{"kairos what-now --minutes 45"},
		Confidence:   0.91,
		Source:       "llm",
	}

	out := FormatHelpAnswer(answer)
	assert.Contains(t, out, "Use kairos status.")
	assert.Contains(t, out, "$ status")
	assert.Contains(t, out, "Show project risks")
	assert.Contains(t, out, "what-now --minutes 45")
	assert.Contains(t, out, "[LLM | Confidence: 91%]")
}

func TestFormatHelpAnswer_DeterministicSourceLabel(t *testing.T) {
	answer := &intelligence.HelpAnswer{
		Answer:     "Use local command help.",
		Confidence: 1.0,
		Source:     "deterministic",
	}

	out := FormatHelpAnswer(answer)
	assert.Contains(t, out, "Use local command help.")
	assert.Contains(t, out, "[Local | Confidence: 100%]")
}

func TestFormatHelpChatWelcomeAndCommandList(t *testing.T) {
	welcome := FormatHelpChatWelcome()
	assert.Contains(t, welcome, "help agent")
	assert.Contains(t, welcome, "/quit")
	assert.Contains(t, welcome, "/commands")

	list := FormatCommandList([]intelligence.HelpCommandInfo{
		{FullPath: "kairos status", Short: "Show status overview"},
		{FullPath: "kairos what-now", Short: "Get recommendations"},
	})
	assert.Contains(t, list, "status")
	assert.Contains(t, list, "Show status overview")
	assert.Contains(t, list, "what-now")
}

func TestWrapText(t *testing.T) {
	got := wrapText("one two three four five", 10)
	assert.Equal(t, "one two\nthree four\nfive", got)
}

func TestIndentWrapped(t *testing.T) {
	got := indentWrapped("alpha beta gamma", 2, 8)
	assert.Equal(t, "  alpha\n  beta\n  gamma", got)
}

func TestFormatHelpAnswer_WrapsLongAnswer(t *testing.T) {
	longAnswer := strings.Repeat("longword ", 20)
	answer := &intelligence.HelpAnswer{
		Answer:     strings.TrimSpace(longAnswer),
		Confidence: 0.8,
		Source:     "llm",
	}

	out := FormatHelpAnswer(answer)
	assert.NotContains(t, out, strings.TrimSpace(longAnswer))
	assert.Contains(t, out, "longword")
}

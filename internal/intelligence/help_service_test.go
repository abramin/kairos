package intelligence

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHelpCommandSpec = `{
  "commands": [
    {"full_path":"kairos status","short":"Show project status"},
    {"full_path":"kairos what-now","short":"Get recommendations","flags":[{"name":"minutes"}]},
    {"full_path":"kairos session log","short":"Log a work session","flags":[{"name":"work-item"},{"name":"minutes"}]},
    {"full_path":"kairos help","short":"Help about any command"}
  ]
}`

func TestHelpServiceAsk_FallbackWhenLLMUnavailable(t *testing.T) {
	svc := NewHelpService(&mockLLMClient{err: llm.ErrOllamaUnavailable}, llm.NoopObserver{})

	answer, err := svc.Ask(context.Background(), "how do I check status?", testHelpCommandSpec)

	require.NoError(t, err)
	assert.Equal(t, "deterministic", answer.Source)
	assert.NotEmpty(t, answer.Answer)
}

func TestHelpServiceAsk_ValidLLMResponse(t *testing.T) {
	client := &mockLLMClient{
		response: `{
      "answer":"Use status to view project health.",
      "examples":[{"command":"kairos status","description":"Show status"}],
      "next_commands":["kairos help"],
      "confidence":0.9
    }`,
	}
	svc := NewHelpService(client, llm.NoopObserver{})

	answer, err := svc.Ask(context.Background(), "how do I check status?", testHelpCommandSpec)

	require.NoError(t, err)
	assert.Equal(t, "llm", answer.Source)
	require.Len(t, answer.Examples, 1)
	assert.Equal(t, "kairos status", answer.Examples[0].Command)
}

func TestHelpServiceAsk_InvalidGroundingFallsBack(t *testing.T) {
	client := &mockLLMClient{
		response: `{
      "answer":"Use this command.",
      "examples":[{"command":"kairos fake --oops 1","description":"Not real"}],
      "next_commands":["kairos nope"],
      "confidence":0.7
    }`,
	}
	svc := NewHelpService(client, llm.NoopObserver{})

	answer, err := svc.Ask(context.Background(), "status", testHelpCommandSpec)

	require.NoError(t, err)
	assert.Equal(t, "deterministic", answer.Source)
	require.NotEmpty(t, answer.Examples)
}

func TestHelpServiceStartChatAndNextTurn(t *testing.T) {
	client := &mockLLMClient{
		response: `{
      "answer":"Try kairos what-now --minutes 45.",
      "examples":[{"command":"kairos what-now --minutes 45","description":"recommendations"}],
      "next_commands":["kairos status"],
      "confidence":0.88
    }`,
	}
	svc := NewHelpService(client, llm.NoopObserver{})

	conv, first, err := svc.StartChat(context.Background(), "what should I do now?", testHelpCommandSpec)
	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, "llm", first.Source)

	next, err := svc.NextTurn(context.Background(), conv, "and then?")
	require.NoError(t, err)
	assert.NotNil(t, next)
	assert.GreaterOrEqual(t, len(conv.Turns), 4)
}

package cli

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHelpService struct {
	answer    *intelligence.HelpAnswer
	askCalled bool
}

func (f *fakeHelpService) Ask(_ context.Context, _, _ string) (*intelligence.HelpAnswer, error) {
	f.askCalled = true
	return f.answer, nil
}

func (f *fakeHelpService) StartChat(_ context.Context, question, commandSpec string) (*intelligence.HelpConversation, *intelligence.HelpAnswer, error) {
	return &intelligence.HelpConversation{
		CommandSpec: commandSpec,
		Turns:       []intelligence.ConversationTurn{{Role: "User", Content: question}},
	}, f.answer, nil
}

func (f *fakeHelpService) NextTurn(_ context.Context, _ *intelligence.HelpConversation, _ string) (*intelligence.HelpAnswer, error) {
	return f.answer, nil
}

func TestHelpCmd_StaticHelpStillWorks(t *testing.T) {
	app := testApp(t)

	output, err := executeCmd(t, app, "help")
	require.NoError(t, err)
	assert.Contains(t, output, "Usage:")

	output, err = executeCmd(t, app, "help", "project")
	require.NoError(t, err)
	assert.Contains(t, output, "project")
}

func TestHelpChatCmd_OneShot_DeterministicWhenLLMDisabled(t *testing.T) {
	app := testApp(t)

	_, err := executeCmd(t, app, "help", "chat", "how do I check status?")
	require.NoError(t, err)
}

func TestHelpChatCmd_OneShot_UsesHelpService(t *testing.T) {
	app := testApp(t)
	fake := &fakeHelpService{
		answer: &intelligence.HelpAnswer{
			Answer:       "Use kairos status.",
			Examples:     []intelligence.ShellExample{{Command: "kairos status", Description: "Show status"}},
			NextCommands: []string{"kairos help"},
			Confidence:   0.9,
			Source:       "llm",
		},
	}
	app.Help = fake

	_, err := executeCmd(t, app, "help", "chat", "how do I check status?")
	require.NoError(t, err)
	assert.True(t, fake.askCalled)
}

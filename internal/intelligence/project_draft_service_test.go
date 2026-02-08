package intelligence

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type draftMockClient struct {
	response string
	err      error
	lastReq  llm.GenerateRequest
}

func (m *draftMockClient) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &llm.GenerateResponse{Text: m.response, Model: "llama3.2"}, nil
}

func (m *draftMockClient) Available(_ context.Context) bool { return m.err == nil }

func draftJSON(resp draftTurnResponse) string {
	data, _ := json.Marshal(resp)
	return string(data)
}

func minimalDraft() *importer.ImportSchema {
	return &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   "PHYS01",
			Name:      "Physics 101",
			Domain:    "education",
			StartDate: "2025-02-01",
		},
		Nodes: []importer.NodeImport{
			{Ref: "n1", Title: "Chapter 1", Kind: "module", Order: 0},
		},
		WorkItems: []importer.WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Read Chapter 1", Type: "reading"},
		},
	}
}

func TestProjectDraftService_Start_ReturnsInitialConversation(t *testing.T) {
	draft := minimalDraft()
	client := &draftMockClient{
		response: draftJSON(draftTurnResponse{
			Message: "I'll help you set up Physics 101. When is the target date?",
			Draft:   draft,
			Status:  "gathering",
		}),
	}

	svc := NewProjectDraftService(client, llm.NoopObserver{})
	conv, err := svc.Start(context.Background(), "I want to study physics")

	require.NoError(t, err)
	assert.Equal(t, DraftStatusGathering, conv.Status)
	assert.Equal(t, "I'll help you set up Physics 101. When is the target date?", conv.LLMMessage)
	assert.NotNil(t, conv.Draft)
	assert.Equal(t, "PHYS01", conv.Draft.Project.ShortID)
	assert.Len(t, conv.Turns, 2) // user turn + assistant turn
	assert.Equal(t, "User", conv.Turns[0].Role)
	assert.Equal(t, "Assistant", conv.Turns[1].Role)
}

func TestProjectDraftService_NextTurn_UpdatesDraft(t *testing.T) {
	targetDate := "2025-06-15"
	initialDraft := minimalDraft()
	updatedDraft := minimalDraft()
	updatedDraft.Project.TargetDate = &targetDate

	client := &draftMockClient{}
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	// Simulate a conversation with one prior turn.
	conv := &DraftConversation{
		Turns: []ConversationTurn{
			{Role: "User", Content: "I want to study physics"},
			{Role: "Assistant", Content: draftJSON(draftTurnResponse{
				Message: "When is the target date?",
				Draft:   initialDraft,
				Status:  "gathering",
			})},
		},
		Draft:  initialDraft,
		Status: DraftStatusGathering,
	}

	client.response = draftJSON(draftTurnResponse{
		Message: "Great, the exam is June 15. How many chapters?",
		Draft:   updatedDraft,
		Status:  "gathering",
	})

	conv, err := svc.NextTurn(context.Background(), conv, "The exam is June 15th")

	require.NoError(t, err)
	assert.Equal(t, DraftStatusGathering, conv.Status)
	assert.NotNil(t, conv.Draft.Project.TargetDate)
	assert.Equal(t, "2025-06-15", *conv.Draft.Project.TargetDate)
	assert.Len(t, conv.Turns, 4) // 2 prior + 2 new
}

func TestProjectDraftService_NextTurn_TranscriptIncludesPriorTurns(t *testing.T) {
	client := &draftMockClient{
		response: draftJSON(draftTurnResponse{
			Message: "Got it.",
			Draft:   minimalDraft(),
			Status:  "gathering",
		}),
	}
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	conv := &DraftConversation{
		Turns: []ConversationTurn{
			{Role: "User", Content: "first message"},
			{Role: "Assistant", Content: "first response"},
		},
		Draft:  minimalDraft(),
		Status: DraftStatusGathering,
	}

	_, err := svc.NextTurn(context.Background(), conv, "second message")
	require.NoError(t, err)

	// Verify the prompt sent to LLM includes the prior turns.
	prompt := client.lastReq.UserPrompt
	assert.Contains(t, prompt, "User: first message")
	assert.Contains(t, prompt, "Assistant: first response")
	assert.Contains(t, prompt, "User: second message")
}

func TestProjectDraftService_ReadyStatus(t *testing.T) {
	client := &draftMockClient{
		response: draftJSON(draftTurnResponse{
			Message: "Here's the complete plan. Review and accept when ready.",
			Draft:   minimalDraft(),
			Status:  "ready",
		}),
	}

	svc := NewProjectDraftService(client, llm.NoopObserver{})
	conv, err := svc.Start(context.Background(), "physics study plan")

	require.NoError(t, err)
	assert.Equal(t, DraftStatusReady, conv.Status)
}

func TestProjectDraftService_PreservesDraftOnNilReturn(t *testing.T) {
	existing := minimalDraft()

	client := &draftMockClient{
		response: draftJSON(draftTurnResponse{
			Message: "Could you clarify?",
			Draft:   nil,
			Status:  "gathering",
		}),
	}

	svc := NewProjectDraftService(client, llm.NoopObserver{})
	conv := &DraftConversation{
		Draft:  existing,
		Status: DraftStatusGathering,
	}

	updated, err := svc.NextTurn(context.Background(), conv, "something unclear")
	require.NoError(t, err)
	assert.Equal(t, existing, updated.Draft, "should preserve previous draft when LLM returns nil")
}

func TestProjectDraftService_LLMError(t *testing.T) {
	client := &draftMockClient{err: llm.ErrOllamaUnavailable}
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	_, err := svc.Start(context.Background(), "physics")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm project draft failed")
}

func TestProjectDraftService_InvalidJSON(t *testing.T) {
	client := &draftMockClient{response: "I don't understand what you mean."}
	svc := NewProjectDraftService(client, llm.NoopObserver{})

	_, err := svc.Start(context.Background(), "physics")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract draft response")
}

func TestProjectDraftService_UsesProjectDraftTask(t *testing.T) {
	client := &draftMockClient{
		response: draftJSON(draftTurnResponse{
			Message: "Starting.",
			Draft:   minimalDraft(),
			Status:  "gathering",
		}),
	}

	svc := NewProjectDraftService(client, llm.NoopObserver{})
	_, err := svc.Start(context.Background(), "physics")
	require.NoError(t, err)
	assert.Equal(t, llm.TaskProjectDraft, client.lastReq.Task)
}

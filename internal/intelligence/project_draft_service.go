package intelligence

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/llm"
)

// DraftStatus represents the state of a project drafting conversation.
type DraftStatus string

const (
	DraftStatusGathering DraftStatus = "gathering"
	DraftStatusReady     DraftStatus = "ready"
)

// ConversationTurn records a single exchange in the drafting conversation.
type ConversationTurn struct {
	Role    string // "User" or "Assistant"
	Content string
}

// DraftConversation holds the full state of a multi-turn project drafting session.
type DraftConversation struct {
	Turns      []ConversationTurn
	Draft      *importer.ImportSchema
	Status     DraftStatus
	LLMMessage string // latest message from the LLM
}

// draftTurnResponse is the JSON structure the LLM outputs at each turn.
type draftTurnResponse struct {
	Message string                `json:"message"`
	Draft   *importer.ImportSchema `json:"draft"`
	Status  string                `json:"status"`
}

// ProjectDraftService manages an interactive, multi-turn conversation
// to build an ImportSchema from natural language.
type ProjectDraftService interface {
	// Start initiates a new conversation with an initial NL description.
	Start(ctx context.Context, description string) (*DraftConversation, error)

	// NextTurn sends a user message in an ongoing conversation and returns
	// the updated conversation with the LLM's response.
	NextTurn(ctx context.Context, conv *DraftConversation, userMessage string) (*DraftConversation, error)
}

type projectDraftService struct {
	client   llm.LLMClient
	observer llm.Observer
}

// NewProjectDraftService creates a ProjectDraftService backed by an LLM client.
func NewProjectDraftService(client llm.LLMClient, observer llm.Observer) ProjectDraftService {
	return &projectDraftService{client: client, observer: observer}
}

func (s *projectDraftService) Start(ctx context.Context, description string) (*DraftConversation, error) {
	conv := &DraftConversation{
		Status: DraftStatusGathering,
	}
	return s.nextTurn(ctx, conv, description)
}

func (s *projectDraftService) NextTurn(ctx context.Context, conv *DraftConversation, userMessage string) (*DraftConversation, error) {
	return s.nextTurn(ctx, conv, userMessage)
}

func (s *projectDraftService) nextTurn(ctx context.Context, conv *DraftConversation, userMessage string) (*DraftConversation, error) {
	prompt := s.buildPrompt(conv, userMessage)

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskProjectDraft,
		SystemPrompt: projectDraftSystemPrompt,
		UserPrompt:   prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("llm project draft failed: %w", err)
	}

	turnResp, err := llm.ExtractJSON[draftTurnResponse](resp.Text, validateDraftTurnResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to extract draft response: %w", err)
	}

	// Preserve previous draft if LLM returned nil (prevent data loss).
	draft := turnResp.Draft
	if draft == nil {
		draft = conv.Draft
	}

	status := DraftStatusGathering
	if turnResp.Status == "ready" {
		status = DraftStatusReady
	}

	// Build updated conversation with the new turns appended.
	updated := &DraftConversation{
		Turns:      make([]ConversationTurn, len(conv.Turns), len(conv.Turns)+2),
		Draft:      draft,
		Status:     status,
		LLMMessage: turnResp.Message,
	}
	copy(updated.Turns, conv.Turns)
	updated.Turns = append(updated.Turns,
		ConversationTurn{Role: "User", Content: userMessage},
		ConversationTurn{Role: "Assistant", Content: resp.Text},
	)

	return updated, nil
}

func (s *projectDraftService) buildPrompt(conv *DraftConversation, currentMessage string) string {
	var b strings.Builder

	for _, turn := range conv.Turns {
		b.WriteString(turn.Role)
		b.WriteString(": ")
		b.WriteString(turn.Content)
		b.WriteString("\n\n")
	}

	b.WriteString("User: ")
	b.WriteString(currentMessage)

	return b.String()
}

func validateDraftTurnResponse(resp draftTurnResponse) error {
	if resp.Message == "" {
		return fmt.Errorf("message field is required")
	}
	if resp.Status != "gathering" && resp.Status != "ready" {
		return fmt.Errorf("status must be \"gathering\" or \"ready\", got %q", resp.Status)
	}
	return nil
}

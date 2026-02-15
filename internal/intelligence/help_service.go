package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/llm"
)

// HelpAnswer is the structured response from the help agent.
type HelpAnswer struct {
	Answer       string         `json:"answer"`
	Examples     []ShellExample `json:"examples"`
	NextCommands []string       `json:"next_commands"`
	Confidence   float64        `json:"confidence"`
	Source       string         `json:"source"` // "llm" or "deterministic"
}

// ShellExample is a command-line example with a description.
type ShellExample struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// HelpCommandInfo is a simplified command descriptor passed from the CLI layer
// to the intelligence layer, avoiding an import cycle with the cli package.
type HelpCommandInfo struct {
	FullPath string
	Short    string
}

// HelpConversation holds multi-turn help chat state.
type HelpConversation struct {
	Turns       []ConversationTurn // reuse ConversationTurn from project_draft
	CommandSpec string             // serialized spec, stored once
}

// HelpService answers user questions about using Kairos.
type HelpService interface {
	// Ask handles a one-shot help question.
	Ask(ctx context.Context, question, commandSpec string) (*HelpAnswer, error)

	// StartChat begins an interactive help conversation.
	StartChat(ctx context.Context, question, commandSpec string) (*HelpConversation, *HelpAnswer, error)

	// NextTurn continues an interactive help conversation.
	NextTurn(ctx context.Context, conv *HelpConversation, question string) (*HelpAnswer, error)
}

type helpService struct {
	client   llm.LLMClient
	observer llm.Observer
}

// NewHelpService creates a HelpService backed by an LLM client.
func NewHelpService(client llm.LLMClient, observer llm.Observer) HelpService {
	return &helpService{client: client, observer: observer}
}

// helpLLMResponse is the JSON structure expected from the LLM.
type helpLLMResponse struct {
	Answer       string         `json:"answer"`
	Examples     []ShellExample `json:"examples"`
	NextCommands []string       `json:"next_commands"`
	Confidence   float64        `json:"confidence"`
}

func (s *helpService) Ask(ctx context.Context, question, commandSpec string) (*HelpAnswer, error) {
	return s.resolveWithFallback(ctx, nil, question, commandSpec), nil
}

func (s *helpService) StartChat(ctx context.Context, question, commandSpec string) (*HelpConversation, *HelpAnswer, error) {
	conv := &HelpConversation{
		CommandSpec: commandSpec,
	}

	answer := s.resolveWithFallback(ctx, conv, question, commandSpec)

	// Record conversation turns.
	conv.Turns = append(conv.Turns,
		ConversationTurn{Role: "User", Content: question},
		ConversationTurn{Role: "Assistant", Content: answer.Answer},
	)

	return conv, answer, nil
}

func (s *helpService) NextTurn(ctx context.Context, conv *HelpConversation, question string) (*HelpAnswer, error) {
	if conv == nil {
		return nil, fmt.Errorf("conversation is nil")
	}
	answer := s.resolveWithFallback(ctx, conv, question, conv.CommandSpec)

	// Append turns.
	conv.Turns = append(conv.Turns,
		ConversationTurn{Role: "User", Content: question},
		ConversationTurn{Role: "Assistant", Content: answer.Answer},
	)

	return answer, nil
}

func (s *helpService) resolveWithFallback(ctx context.Context, conv *HelpConversation, question, commandSpec string) *HelpAnswer {
	commandInfos, validCmds, validFlags := parseHelpCommandSpec(commandSpec)

	userPrompt := buildHelpUserPrompt(conv, question, commandSpec)
	answer, err := s.generate(ctx, userPrompt)
	if err != nil {
		return DeterministicHelp(question, commandInfos)
	}

	answer, groundingStripped := ValidateHelpGrounding(answer, validCmds, validFlags)
	if groundingStripped && len(answer.Examples) == 0 && len(answer.NextCommands) == 0 {
		// LLM produced only hallucinated commands; fall back to deterministic.
		fallback := DeterministicHelp(question, commandInfos)
		if strings.TrimSpace(answer.Answer) == "" {
			answer.Answer = fallback.Answer
		}
		answer.Examples = fallback.Examples
		answer.NextCommands = fallback.NextCommands
		answer.Confidence = fallback.Confidence
		answer.Source = "deterministic"
	}

	return answer
}

func (s *helpService) generate(ctx context.Context, userPrompt string) (*HelpAnswer, error) {
	systemPrompt := buildHelpSystemPrompt()

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskHelp,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("llm help generation failed: %w", err)
	}

	parsed, err := llm.ExtractJSON[helpLLMResponse](resp.Text, validateHelpResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to extract help response: %w", err)
	}

	return &HelpAnswer{
		Answer:       parsed.Answer,
		Examples:     parsed.Examples,
		NextCommands: parsed.NextCommands,
		Confidence:   parsed.Confidence,
		Source:       "llm",
	}, nil
}

func buildHelpUserPrompt(conv *HelpConversation, question, commandSpec string) string {
	var b strings.Builder

	// Include conversation history for multi-turn.
	if conv != nil && len(conv.Turns) > 0 {
		b.WriteString("Previous conversation:\n")
		for _, turn := range conv.Turns {
			b.WriteString(turn.Role)
			b.WriteString(": ")
			b.WriteString(turn.Content)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("## Command Specification\n")
	b.WriteString(commandSpec)
	b.WriteString("\n\n## User Question\n")
	b.WriteString(question)

	return b.String()
}

func validateHelpResponse(resp helpLLMResponse) error {
	if resp.Answer == "" {
		return fmt.Errorf("answer field is required")
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1, got %f", resp.Confidence)
	}
	return nil
}

type helpCommandSpec struct {
	Commands []helpCommandSpecCommand `json:"commands"`
}

type helpCommandSpecCommand struct {
	FullPath string                `json:"full_path"`
	Short    string                `json:"short"`
	Flags    []helpCommandSpecFlag `json:"flags,omitempty"`
}

type helpCommandSpecFlag struct {
	Name string `json:"name"`
}

func parseHelpCommandSpec(commandSpec string) ([]HelpCommandInfo, map[string]bool, map[string]map[string]bool) {
	var parsed helpCommandSpec
	if err := json.Unmarshal([]byte(commandSpec), &parsed); err != nil {
		return nil, map[string]bool{}, map[string]map[string]bool{}
	}

	infos := make([]HelpCommandInfo, 0, len(parsed.Commands))
	validCmds := make(map[string]bool, len(parsed.Commands))
	validFlags := make(map[string]map[string]bool, len(parsed.Commands))

	for _, cmd := range parsed.Commands {
		if cmd.FullPath == "" {
			continue
		}

		infos = append(infos, HelpCommandInfo{
			FullPath: cmd.FullPath,
			Short:    cmd.Short,
		})
		validCmds[cmd.FullPath] = true

		if len(cmd.Flags) == 0 {
			continue
		}
		fm := make(map[string]bool, len(cmd.Flags))
		for _, flag := range cmd.Flags {
			if flag.Name != "" {
				fm[flag.Name] = true
			}
		}
		validFlags[cmd.FullPath] = fm
	}

	return infos, validCmds, validFlags
}

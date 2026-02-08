package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/intelligence"
)

// draftPhase tracks progress through the interactive draft flow.
type draftPhase int

const (
	draftPhaseDescription draftPhase = iota
	draftPhaseStartDate
	draftPhaseDeadline
	draftPhaseStructure
	draftPhaseConversation
	draftPhaseReview
)

// execDraft enters draft mode within the shell. If a description is provided
// as arguments, it skips the gathering phase and starts the LLM conversation
// directly.
func (s *shellSession) execDraft(args []string) {
	if s.app.ProjectDraft == nil {
		fmt.Println(formatter.StyleRed.Render(
			"LLM features are disabled. Use explicit commands:\n" +
				"  project add --name ... --domain ... --start ...\n" +
				"  project import file.json\n\n" +
				"Enable with: KAIROS_LLM_ENABLED=true"))
		return
	}

	s.draftMode = true

	if len(args) > 0 {
		// Description provided as argument — skip gathering.
		description := strings.Join(args, " ")
		description += "\nStart date: " + time.Now().Format("2006-01-02")
		s.startDraftConversation(description)
		return
	}

	// Enter gathering mode.
	s.draftPhase = draftPhaseDescription
	fmt.Print(formatter.FormatDraftWelcome())
	fmt.Print("  Describe your project:\n  > ")
}

// execDraftTurn handles a single line of input while in draft mode.
func (s *shellSession) execDraftTurn(input string) {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "/quit" || lower == "/cancel" || lower == "/q" {
		fmt.Println("Draft cancelled.")
		s.exitDraftMode()
		return
	}

	switch s.draftPhase {
	case draftPhaseDescription:
		if input == "" {
			fmt.Println(formatter.StyleRed.Render("  Project description is required."))
			fmt.Print("  > ")
			return
		}
		s.draftDescription = input
		s.draftPhase = draftPhaseStartDate
		fmt.Print("\n  When do you want to start? (YYYY-MM-DD, or Enter for today)\n  > ")

	case draftPhaseStartDate:
		s.draftStartDate = time.Now().Format("2006-01-02")
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("  Invalid date format, using today.")
			} else {
				s.draftStartDate = input
			}
		}
		s.draftPhase = draftPhaseDeadline
		fmt.Print("\n  When is the deadline? (YYYY-MM-DD, or Enter to skip)\n  > ")

	case draftPhaseDeadline:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("  Invalid date format, skipping deadline.")
			} else {
				s.draftDeadline = input
			}
		}
		s.draftPhase = draftPhaseStructure
		fmt.Print("\n  How is the work organized? (e.g., \"5 chapters with reading + exercises each\")\n  > ")

	case draftPhaseStructure:
		s.draftStructure = input

		var b strings.Builder
		b.WriteString(s.draftDescription)
		b.WriteString("\nStart date: ")
		b.WriteString(s.draftStartDate)
		if s.draftDeadline != "" {
			b.WriteString("\nDeadline: ")
			b.WriteString(s.draftDeadline)
		}
		if s.draftStructure != "" {
			b.WriteString("\nStructure: ")
			b.WriteString(s.draftStructure)
		}
		fmt.Printf("\n  %s\n\n", formatter.Dim("Building your project draft..."))
		s.startDraftConversation(b.String())

	case draftPhaseConversation:
		s.handleDraftConversationInput(input)

	case draftPhaseReview:
		s.handleDraftReviewInput(input)
	}
}

func (s *shellSession) startDraftConversation(description string) {
	ctx := context.Background()
	conv, err := s.app.ProjectDraft.Start(ctx, description)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Failed to start project draft: %v", err)))
		s.exitDraftMode()
		return
	}
	s.draftConv = conv
	fmt.Print(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		s.draftPhase = draftPhaseReview
		fmt.Print(formatter.FormatDraftReview(conv))
		fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")
	} else {
		s.draftPhase = draftPhaseConversation
	}
}

func (s *shellSession) handleDraftConversationInput(input string) {
	if input == "" {
		return
	}

	lower := strings.ToLower(input)
	switch lower {
	case "/show", "/draft":
		fmt.Print(formatter.FormatDraftPreview(s.draftConv))
		return
	case "/accept":
		if s.draftConv.Draft != nil {
			s.acceptShellDraft()
		} else {
			fmt.Println("No draft to accept yet.")
		}
		return
	}

	ctx := context.Background()
	conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draftConv, input)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	s.draftConv = conv
	fmt.Print(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		s.draftPhase = draftPhaseReview
		fmt.Print(formatter.FormatDraftReview(conv))
		fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")
	}
}

func (s *shellSession) handleDraftReviewInput(input string) {
	switch strings.ToLower(input) {
	case "a", "accept":
		s.acceptShellDraft()
	case "c", "cancel":
		fmt.Println("Draft cancelled.")
		s.exitDraftMode()
	case "e", "edit":
		s.draftConv.Status = intelligence.DraftStatusGathering
		s.draftPhase = draftPhaseConversation
		fmt.Print("What would you like to change?\n")
	default:
		// Treat as an edit instruction.
		s.draftConv.Status = intelligence.DraftStatusGathering
		s.draftPhase = draftPhaseConversation
		ctx := context.Background()
		conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draftConv, input)
		if err != nil {
			fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
			return
		}
		s.draftConv = conv
		fmt.Print(formatter.FormatDraftTurn(conv))
		if conv.Status == intelligence.DraftStatusReady {
			s.draftPhase = draftPhaseReview
			fmt.Print(formatter.FormatDraftReview(conv))
			fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")
		}
	}
}

func (s *shellSession) acceptShellDraft() {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(s.draftConv.Draft)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors. Continue editing to fix them.")
		s.draftConv.Status = intelligence.DraftStatusGathering
		s.draftPhase = draftPhaseConversation
		return
	}

	result, err := s.app.Import.ImportProjectFromSchema(ctx, s.draftConv.Draft)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err)))
		return
	}

	fmt.Print(formatter.FormatDraftAccepted(result))
	s.exitDraftMode()
}

func (s *shellSession) exitDraftMode() {
	s.draftMode = false
	s.draftPhase = draftPhaseDescription
	s.draftConv = nil
	s.draftDescription = ""
	s.draftStartDate = ""
	s.draftDeadline = ""
	s.draftStructure = ""
}

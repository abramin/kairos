package cli

import (
	"context"
	"fmt"
	"strconv"
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
	// Wizard phases.
	draftPhaseGroupCount
	draftPhaseGroupLabel
	draftPhaseGroupNodeCount
	draftPhaseGroupKind
	draftPhaseGroupDays
	draftPhaseWorkItemTitle
	draftPhaseWorkItemType
	draftPhaseWorkItemMinutes
	draftPhaseSpecialTitle
	draftPhaseSpecialKind
	draftPhaseSpecialDueDate
	draftPhaseSpecialWITitle
	draftPhaseSpecialWIType
	draftPhaseSpecialWIMinutes
	draftPhaseWizardReview
	// LLM conversation phases.
	draftPhaseConversation
	draftPhaseReview
)

// execDraft enters draft mode within the shell.
func (s *shellSession) execDraft(args []string) {
	if len(args) > 0 && s.app.ProjectDraft == nil {
		fmt.Println(formatter.StyleRed.Render(
			"LLM features are disabled. Run 'draft' without arguments for the guided wizard.\n" +
				"Or use explicit commands:\n" +
				"  project add --name ... --domain ... --start ...\n" +
				"  project import file.json\n\n" +
				"Enable with: KAIROS_LLM_ENABLED=true"))
		return
	}

	s.draftMode = true

	if len(args) > 0 {
		// Description provided: use LLM conversational flow.
		description := strings.Join(args, " ")
		description += "\nStart date: " + time.Now().Format("2006-01-02")
		s.startDraftConversation(description, nil)
		return
	}

	// Enter wizard gathering mode.
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
		// Enter wizard: ask for group count.
		s.draftPhase = draftPhaseGroupCount
		s.draftGroups = nil
		s.draftWorkItems = nil
		s.draftSpecialNodes = nil
		s.draftGroupTotal = 1
		s.draftCurrentGroupIdx = 0
		fmt.Print("\n  How many groups of work? (e.g., phases, levels — Enter for 1)\n  > ")

	case draftPhaseGroupCount:
		n := 1
		if input != "" {
			parsed, err := strconv.Atoi(input)
			if err != nil || parsed < 1 {
				fmt.Println("  Invalid number, using 1.")
			} else {
				n = parsed
			}
		}
		s.draftGroupTotal = n
		s.draftCurrentGroupIdx = 0
		s.draftGroups = nil
		s.draftCurrentGroup = wizardGroup{Kind: "module"}
		s.draftPhase = draftPhaseGroupLabel
		if n > 1 {
			fmt.Printf("\n  --- Group 1 ---\n")
			fmt.Print("  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"): ")
		} else {
			fmt.Print("\n  Node label (e.g., \"Chapter\", \"Week\", \"Module\"): ")
		}

	case draftPhaseGroupLabel:
		label := input
		if label == "" {
			label = "Module"
		}
		s.draftCurrentGroup.Label = label
		s.draftPhase = draftPhaseGroupNodeCount
		fmt.Print("  How many? ")

	case draftPhaseGroupNodeCount:
		count, err := strconv.Atoi(input)
		if err != nil || count < 1 {
			fmt.Println("  Invalid number, using 1.")
			count = 1
		}
		s.draftCurrentGroup.Count = count
		s.draftPhase = draftPhaseGroupKind
		fmt.Print("  Node kind [module/week/section/stage/assessment/generic] (Enter for module): ")

	case draftPhaseGroupKind:
		if input != "" {
			kind := strings.ToLower(input)
			if validNodeKinds[kind] {
				s.draftCurrentGroup.Kind = kind
			} else {
				fmt.Println("  Invalid kind, using module.")
			}
		}
		s.draftPhase = draftPhaseGroupDays
		fmt.Print("  Days per node (Enter to spread evenly): ")

	case draftPhaseGroupDays:
		if input != "" {
			days, err := strconv.Atoi(input)
			if err != nil || days < 1 {
				fmt.Println("  Invalid number, skipping.")
			} else {
				s.draftCurrentGroup.DaysPer = days
			}
		}
		// Save group and advance.
		s.draftGroups = append(s.draftGroups, s.draftCurrentGroup)
		s.draftCurrentGroupIdx++

		if s.draftCurrentGroupIdx < s.draftGroupTotal {
			s.draftCurrentGroup = wizardGroup{Kind: "module"}
			s.draftPhase = draftPhaseGroupLabel
			fmt.Printf("\n  --- Group %d ---\n", s.draftCurrentGroupIdx+1)
			fmt.Print("  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"): ")
		} else {
			// Move to work items.
			s.draftPhase = draftPhaseWorkItemTitle
			fmt.Print("\n  --- Work Items (applied to every node) ---\n")
			fmt.Print("  Title (Enter when done): ")
		}

	case draftPhaseWorkItemTitle:
		if input == "" {
			if len(s.draftWorkItems) == 0 {
				fmt.Println("  At least one work item is recommended.")
			}
			// Move to special nodes.
			s.draftPhase = draftPhaseSpecialTitle
			fmt.Print("\n  --- Special Nodes (exams, milestones — Enter to skip) ---\n")
			fmt.Print("  Title (Enter to skip): ")
			return
		}
		s.draftCurrentWI = wizardWorkItem{Title: input, Type: "task"}
		s.draftPhase = draftPhaseWorkItemType
		fmt.Print("    Type [reading/practice/review/assignment/task/quiz/study]: ")

	case draftPhaseWorkItemType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				s.draftCurrentWI.Type = t
			} else {
				fmt.Println("    Invalid type, using task.")
			}
		}
		s.draftPhase = draftPhaseWorkItemMinutes
		fmt.Print("    Estimated minutes: ")

	case draftPhaseWorkItemMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				fmt.Println("    Invalid number, using 30.")
				mins = 30
			}
			s.draftCurrentWI.PlannedMin = mins
		}
		s.draftWorkItems = append(s.draftWorkItems, s.draftCurrentWI)
		s.draftPhase = draftPhaseWorkItemTitle
		fmt.Print("  Title (Enter when done): ")

	case draftPhaseSpecialTitle:
		if input == "" {
			// Done with special nodes, build and review.
			s.buildAndShowWizardDraft()
			return
		}
		s.draftCurrentSpecial = wizardSpecialNode{Title: input, Kind: "assessment"}
		s.draftPhase = draftPhaseSpecialKind
		fmt.Print("    Kind [assessment/generic] (Enter for assessment): ")

	case draftPhaseSpecialKind:
		if input != "" {
			kind := strings.ToLower(input)
			if kind == "assessment" || kind == "generic" {
				s.draftCurrentSpecial.Kind = kind
			} else {
				fmt.Println("    Invalid kind, using assessment.")
			}
		}
		s.draftPhase = draftPhaseSpecialDueDate
		fmt.Print("    Due date (YYYY-MM-DD, Enter for deadline): ")

	case draftPhaseSpecialDueDate:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("    Invalid date, skipping.")
			} else {
				s.draftCurrentSpecial.DueDate = input
			}
		}
		s.draftPhase = draftPhaseSpecialWITitle
		fmt.Print("    Work item title (Enter when done): ")

	case draftPhaseSpecialWITitle:
		if input == "" {
			// Done with this special node's work items.
			s.draftSpecialNodes = append(s.draftSpecialNodes, s.draftCurrentSpecial)
			s.draftPhase = draftPhaseSpecialTitle
			fmt.Print("  Title (Enter to skip): ")
			return
		}
		s.draftCurrentSpecialWI = wizardWorkItem{Title: input, Type: "task"}
		s.draftPhase = draftPhaseSpecialWIType
		fmt.Print("      Type [reading/practice/review/assignment/task/quiz/study]: ")

	case draftPhaseSpecialWIType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				s.draftCurrentSpecialWI.Type = t
			} else {
				fmt.Println("      Invalid type, using task.")
			}
		}
		s.draftPhase = draftPhaseSpecialWIMinutes
		fmt.Print("      Estimated minutes: ")

	case draftPhaseSpecialWIMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				fmt.Println("      Invalid number, using 30.")
				mins = 30
			}
			s.draftCurrentSpecialWI.PlannedMin = mins
		}
		s.draftCurrentSpecial.WorkItems = append(s.draftCurrentSpecial.WorkItems, s.draftCurrentSpecialWI)
		s.draftPhase = draftPhaseSpecialWITitle
		fmt.Print("    Work item title (Enter when done): ")

	case draftPhaseWizardReview:
		s.handleWizardReviewInput(input)

	case draftPhaseConversation:
		s.handleDraftConversationInput(input)

	case draftPhaseReview:
		s.handleDraftReviewInput(input)
	}
}

func (s *shellSession) buildAndShowWizardDraft() {
	wizard := &wizardResult{
		Description:  s.draftDescription,
		StartDate:    s.draftStartDate,
		Deadline:     s.draftDeadline,
		Groups:       s.draftGroups,
		WorkItems:    s.draftWorkItems,
		SpecialNodes: s.draftSpecialNodes,
	}
	s.draftWizard = wizard
	s.draftSchema = buildSchemaFromWizard(wizard)

	// Show preview.
	conv := &intelligence.DraftConversation{
		Draft:  s.draftSchema,
		Status: intelligence.DraftStatusReady,
	}
	fmt.Print(formatter.FormatDraftPreview(conv))

	s.draftPhase = draftPhaseWizardReview
	if s.app.ProjectDraft != nil {
		fmt.Print("\n[a]ccept  [r]efine with AI  [c]ancel: ")
	} else {
		fmt.Print("\n[a]ccept  [c]ancel: ")
	}
}

func (s *shellSession) handleWizardReviewInput(input string) {
	switch strings.ToLower(input) {
	case "a", "accept":
		s.acceptWizardSchema()
	case "c", "cancel":
		fmt.Println("Draft cancelled.")
		s.exitDraftMode()
	case "r", "refine":
		if s.app.ProjectDraft == nil {
			fmt.Println("LLM features are disabled. Accept the draft or cancel.")
			if s.app.ProjectDraft != nil {
				fmt.Print("\n[a]ccept  [r]efine with AI  [c]ancel: ")
			} else {
				fmt.Print("\n[a]ccept  [c]ancel: ")
			}
			return
		}
		desc := buildLLMDescription(s.draftWizard)
		s.startDraftConversation(desc, s.draftSchema)
	default:
		fmt.Println("Invalid option.")
		if s.app.ProjectDraft != nil {
			fmt.Print("\n[a]ccept  [r]efine with AI  [c]ancel: ")
		} else {
			fmt.Print("\n[a]ccept  [c]ancel: ")
		}
	}
}

func (s *shellSession) acceptWizardSchema() {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(s.draftSchema)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors.")
		s.exitDraftMode()
		return
	}

	result, err := s.app.Import.ImportProjectFromSchema(ctx, s.draftSchema)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err)))
		return
	}

	fmt.Print(formatter.FormatDraftAccepted(result))
	s.exitDraftMode()
}

func (s *shellSession) startDraftConversation(description string, preDraft *importer.ImportSchema) {
	ctx := context.Background()

	var conv *intelligence.DraftConversation
	var err error

	if preDraft != nil {
		stopSpinner := formatter.StartSpinner("Preparing for refinement...")
		conv, err = s.app.ProjectDraft.StartWithDraft(ctx, description, preDraft)
		stopSpinner()
	} else {
		stopSpinner := formatter.StartSpinner("Building your project draft...")
		conv, err = s.app.ProjectDraft.Start(ctx, description)
		stopSpinner()
	}
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
	stopSpinner := formatter.StartSpinner("Thinking...")
	conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draftConv, input)
	stopSpinner()
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
		stopSpinner := formatter.StartSpinner("Thinking...")
		conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draftConv, input)
		stopSpinner()
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
	s.draftGroups = nil
	s.draftWorkItems = nil
	s.draftSpecialNodes = nil
	s.draftGroupTotal = 0
	s.draftCurrentGroupIdx = 0
	s.draftCurrentGroup = wizardGroup{}
	s.draftCurrentWI = wizardWorkItem{}
	s.draftCurrentSpecial = wizardSpecialNode{}
	s.draftCurrentSpecialWI = wizardWorkItem{}
	s.draftWizard = nil
	s.draftSchema = nil
}

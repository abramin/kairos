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

// draftWizardState holds all mutable state for a draft-mode session.
type draftWizardState struct {
	phase            draftPhase
	description      string
	startDate        string
	deadline         string
	conv             *intelligence.DraftConversation
	groups           []wizardGroup
	workItems        []wizardWorkItem
	specialNodes     []wizardSpecialNode
	groupTotal       int
	currentGroupIdx  int
	currentGroup     wizardGroup
	currentWI        wizardWorkItem
	currentSpecial   wizardSpecialNode
	currentSpecialWI wizardWorkItem
	wizard           *wizardResult
	schema           *importer.ImportSchema
}

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
	s.draft = &draftWizardState{}

	if len(args) > 0 {
		// Description provided: use LLM conversational flow.
		description := strings.Join(args, " ")
		description += "\nStart date: " + time.Now().Format("2006-01-02")
		s.startDraftConversation(description, nil)
		return
	}

	// Enter wizard gathering mode.
	s.draft.phase = draftPhaseDescription
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

	switch s.draft.phase {
	case draftPhaseDescription:
		if input == "" {
			fmt.Println(formatter.StyleRed.Render("  Project description is required."))
			fmt.Print("  > ")
			return
		}
		s.draft.description = input
		s.draft.phase = draftPhaseStartDate
		fmt.Print("\n  When do you want to start? (YYYY-MM-DD, or Enter for today)\n  > ")

	case draftPhaseStartDate:
		s.draft.startDate = time.Now().Format("2006-01-02")
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("  Invalid date format, using today.")
			} else {
				s.draft.startDate = input
			}
		}
		s.draft.phase = draftPhaseDeadline
		fmt.Print("\n  When is the deadline? (YYYY-MM-DD, or Enter to skip)\n  > ")

	case draftPhaseDeadline:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("  Invalid date format, skipping deadline.")
			} else {
				s.draft.deadline = input
			}
		}
		// Enter wizard: ask for group count.
		s.draft.phase = draftPhaseGroupCount
		s.draft.groups = nil
		s.draft.workItems = nil
		s.draft.specialNodes = nil
		s.draft.groupTotal = 1
		s.draft.currentGroupIdx = 0
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
		s.draft.groupTotal = n
		s.draft.currentGroupIdx = 0
		s.draft.groups = nil
		s.draft.currentGroup = wizardGroup{Kind: "module"}
		s.draft.phase = draftPhaseGroupLabel
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
		s.draft.currentGroup.Label = label
		s.draft.phase = draftPhaseGroupNodeCount
		fmt.Print("  How many? ")

	case draftPhaseGroupNodeCount:
		count, err := strconv.Atoi(input)
		if err != nil || count < 1 {
			fmt.Println("  Invalid number, using 1.")
			count = 1
		}
		s.draft.currentGroup.Count = count
		s.draft.phase = draftPhaseGroupKind
		fmt.Print("  Node kind [module/week/section/stage/assessment/generic] (Enter for module): ")

	case draftPhaseGroupKind:
		if input != "" {
			kind := strings.ToLower(input)
			if validNodeKinds[kind] {
				s.draft.currentGroup.Kind = kind
			} else {
				fmt.Println("  Invalid kind, using module.")
			}
		}
		s.draft.phase = draftPhaseGroupDays
		fmt.Print("  Days per node (Enter to spread evenly): ")

	case draftPhaseGroupDays:
		if input != "" {
			days, err := strconv.Atoi(input)
			if err != nil || days < 1 {
				fmt.Println("  Invalid number, skipping.")
			} else {
				s.draft.currentGroup.DaysPer = days
			}
		}
		// Save group and advance.
		s.draft.groups = append(s.draft.groups, s.draft.currentGroup)
		s.draft.currentGroupIdx++

		if s.draft.currentGroupIdx < s.draft.groupTotal {
			s.draft.currentGroup = wizardGroup{Kind: "module"}
			s.draft.phase = draftPhaseGroupLabel
			fmt.Printf("\n  --- Group %d ---\n", s.draft.currentGroupIdx+1)
			fmt.Print("  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"): ")
		} else {
			// Move to work items.
			s.draft.phase = draftPhaseWorkItemTitle
			fmt.Print("\n  --- Work Items (applied to every node) ---\n")
			fmt.Print("  Title (Enter when done): ")
		}

	case draftPhaseWorkItemTitle:
		if input == "" {
			if len(s.draft.workItems) == 0 {
				fmt.Println("  At least one work item is recommended.")
			}
			// Move to special nodes.
			s.draft.phase = draftPhaseSpecialTitle
			fmt.Print("\n  --- Special Nodes (exams, milestones — Enter to skip) ---\n")
			fmt.Print("  Title (Enter to skip): ")
			return
		}
		s.draft.currentWI = wizardWorkItem{Title: input, Type: "task"}
		s.draft.phase = draftPhaseWorkItemType
		fmt.Print("    Type [reading/practice/review/assignment/task/quiz/study]: ")

	case draftPhaseWorkItemType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				s.draft.currentWI.Type = t
			} else {
				fmt.Println("    Invalid type, using task.")
			}
		}
		s.draft.phase = draftPhaseWorkItemMinutes
		fmt.Print("    Estimated minutes: ")

	case draftPhaseWorkItemMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				fmt.Println("    Invalid number, using 30.")
				mins = 30
			}
			s.draft.currentWI.PlannedMin = mins
		}
		s.draft.workItems = append(s.draft.workItems, s.draft.currentWI)
		s.draft.phase = draftPhaseWorkItemTitle
		fmt.Print("  Title (Enter when done): ")

	case draftPhaseSpecialTitle:
		if input == "" {
			// Done with special nodes, build and review.
			s.buildAndShowWizardDraft()
			return
		}
		s.draft.currentSpecial = wizardSpecialNode{Title: input, Kind: "assessment"}
		s.draft.phase = draftPhaseSpecialKind
		fmt.Print("    Kind [assessment/generic] (Enter for assessment): ")

	case draftPhaseSpecialKind:
		if input != "" {
			kind := strings.ToLower(input)
			if kind == "assessment" || kind == "generic" {
				s.draft.currentSpecial.Kind = kind
			} else {
				fmt.Println("    Invalid kind, using assessment.")
			}
		}
		s.draft.phase = draftPhaseSpecialDueDate
		fmt.Print("    Due date (YYYY-MM-DD, Enter for deadline): ")

	case draftPhaseSpecialDueDate:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Println("    Invalid date, skipping.")
			} else {
				s.draft.currentSpecial.DueDate = input
			}
		}
		s.draft.phase = draftPhaseSpecialWITitle
		fmt.Print("    Work item title (Enter when done): ")

	case draftPhaseSpecialWITitle:
		if input == "" {
			// Done with this special node's work items.
			s.draft.specialNodes = append(s.draft.specialNodes, s.draft.currentSpecial)
			s.draft.phase = draftPhaseSpecialTitle
			fmt.Print("  Title (Enter to skip): ")
			return
		}
		s.draft.currentSpecialWI = wizardWorkItem{Title: input, Type: "task"}
		s.draft.phase = draftPhaseSpecialWIType
		fmt.Print("      Type [reading/practice/review/assignment/task/quiz/study]: ")

	case draftPhaseSpecialWIType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				s.draft.currentSpecialWI.Type = t
			} else {
				fmt.Println("      Invalid type, using task.")
			}
		}
		s.draft.phase = draftPhaseSpecialWIMinutes
		fmt.Print("      Estimated minutes: ")

	case draftPhaseSpecialWIMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				fmt.Println("      Invalid number, using 30.")
				mins = 30
			}
			s.draft.currentSpecialWI.PlannedMin = mins
		}
		s.draft.currentSpecial.WorkItems = append(s.draft.currentSpecial.WorkItems, s.draft.currentSpecialWI)
		s.draft.phase = draftPhaseSpecialWITitle
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
		Description:  s.draft.description,
		StartDate:    s.draft.startDate,
		Deadline:     s.draft.deadline,
		Groups:       s.draft.groups,
		WorkItems:    s.draft.workItems,
		SpecialNodes: s.draft.specialNodes,
	}
	s.draft.wizard = wizard
	s.draft.schema = buildSchemaFromWizard(wizard)

	// Show preview.
	conv := &intelligence.DraftConversation{
		Draft:  s.draft.schema,
		Status: intelligence.DraftStatusReady,
	}
	fmt.Print(formatter.FormatDraftPreview(conv))

	s.draft.phase = draftPhaseWizardReview
	s.printWizardReviewPrompt()
}

func (s *shellSession) printWizardReviewPrompt() {
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
			s.printWizardReviewPrompt()
			return
		}
		desc := buildLLMDescription(s.draft.wizard)
		s.startDraftConversation(desc, s.draft.schema)
	default:
		fmt.Println("Invalid option.")
		s.printWizardReviewPrompt()
	}
}

func (s *shellSession) acceptWizardSchema() {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(s.draft.schema)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors.")
		s.exitDraftMode()
		return
	}

	result, err := s.app.Import.ImportProjectFromSchema(ctx, s.draft.schema)
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
	s.draft.conv = conv
	fmt.Print(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		s.draft.phase = draftPhaseReview
		fmt.Print(formatter.FormatDraftReview(conv))
		fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")
	} else {
		s.draft.phase = draftPhaseConversation
	}
}

func (s *shellSession) handleDraftConversationInput(input string) {
	if input == "" {
		return
	}

	lower := strings.ToLower(input)
	switch lower {
	case "/show", "/draft":
		fmt.Print(formatter.FormatDraftPreview(s.draft.conv))
		return
	case "/accept":
		if s.draft.conv.Draft != nil {
			s.acceptShellDraft()
		} else {
			fmt.Println("No draft to accept yet.")
		}
		return
	}

	ctx := context.Background()
	stopSpinner := formatter.StartSpinner("Thinking...")
	conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draft.conv, input)
	stopSpinner()
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	s.draft.conv = conv
	fmt.Print(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		s.draft.phase = draftPhaseReview
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
		s.draft.conv.Status = intelligence.DraftStatusGathering
		s.draft.phase = draftPhaseConversation
		fmt.Print("What would you like to change?\n")
	default:
		// Treat as an edit instruction.
		s.draft.conv.Status = intelligence.DraftStatusGathering
		s.draft.phase = draftPhaseConversation
		ctx := context.Background()
		stopSpinner := formatter.StartSpinner("Thinking...")
		conv, err := s.app.ProjectDraft.NextTurn(ctx, s.draft.conv, input)
		stopSpinner()
		if err != nil {
			fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
			return
		}
		s.draft.conv = conv
		fmt.Print(formatter.FormatDraftTurn(conv))
		if conv.Status == intelligence.DraftStatusReady {
			s.draft.phase = draftPhaseReview
			fmt.Print(formatter.FormatDraftReview(conv))
			fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")
		}
	}
}

func (s *shellSession) acceptShellDraft() {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(s.draft.conv.Draft)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors. Continue editing to fix them.")
		s.draft.conv.Status = intelligence.DraftStatusGathering
		s.draft.phase = draftPhaseConversation
		return
	}

	result, err := s.app.Import.ImportProjectFromSchema(ctx, s.draft.conv.Draft)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err)))
		return
	}

	fmt.Print(formatter.FormatDraftAccepted(result))
	s.exitDraftMode()
}

func (s *shellSession) exitDraftMode() {
	s.draftMode = false
	s.draft = nil
}

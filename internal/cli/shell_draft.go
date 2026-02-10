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
	tea "github.com/charmbracelet/bubbletea"
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
func (m *shellModel) execDraft(args []string) string {
	if len(args) > 0 && m.app.ProjectDraft == nil {
		return formatter.StyleRed.Render(
			"LLM features are disabled. Run 'draft' without arguments for the guided wizard.\n" +
				"Or use explicit commands:\n" +
				"  project add --name ... --domain ... --start ...\n" +
				"  project import file.json\n\n" +
				"Enable with: KAIROS_LLM_ENABLED=true")
	}

	m.mode = modeDraft
	m.draft = &draftWizardState{}

	if len(args) > 0 {
		// Description provided: use LLM conversational flow.
		description := strings.Join(args, " ")
		description += "\nStart date: " + time.Now().Format("2006-01-02")
		return m.startDraftConversation(description, nil)
	}

	// Enter wizard gathering mode.
	m.draft.phase = draftPhaseDescription
	return formatter.FormatDraftWelcome() + "\n  Describe your project:"
}

// handleDraftInput handles a single line of input while in draft mode.
// Dispatches to phase-specific handlers.
func (m *shellModel) handleDraftInput(input string) (string, tea.Cmd) {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "/quit" || lower == "/cancel" || lower == "/q" {
		m.exitDraftMode()
		return "Draft cancelled.", nil
	}

	switch {
	case m.draft.phase <= draftPhaseDeadline:
		return m.handleDraftMetadata(input), nil
	case m.draft.phase <= draftPhaseGroupDays:
		return m.handleDraftGroup(input), nil
	case m.draft.phase <= draftPhaseWorkItemMinutes:
		return m.handleDraftWorkItem(input), nil
	case m.draft.phase <= draftPhaseSpecialWIMinutes:
		return m.handleDraftSpecialNode(input), nil
	case m.draft.phase == draftPhaseWizardReview:
		return m.handleWizardReviewInput(input), nil
	case m.draft.phase == draftPhaseConversation:
		return m.handleDraftConversationInput(input), nil
	case m.draft.phase == draftPhaseReview:
		return m.handleDraftReviewInput(input), nil
	}
	return "", nil
}

// handleDraftMetadata handles the description, start date, and deadline phases.
func (m *shellModel) handleDraftMetadata(input string) string {
	switch m.draft.phase {
	case draftPhaseDescription:
		if input == "" {
			return formatter.StyleRed.Render("  Project description is required.")
		}
		m.draft.description = input
		m.draft.phase = draftPhaseStartDate
		return "\n  When do you want to start? (YYYY-MM-DD, or Enter for today)"

	case draftPhaseStartDate:
		m.draft.startDate = time.Now().Format("2006-01-02")
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				m.draft.startDate = input
			}
		}
		m.draft.phase = draftPhaseDeadline
		return "\n  When is the deadline? (YYYY-MM-DD, or Enter to skip)"

	case draftPhaseDeadline:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				m.draft.deadline = input
			}
		}
		m.draft.phase = draftPhaseGroupCount
		m.draft.groups = nil
		m.draft.workItems = nil
		m.draft.specialNodes = nil
		m.draft.groupTotal = 1
		m.draft.currentGroupIdx = 0
		return "\n  How many groups of work? (e.g., phases, levels — Enter for 1)"
	}
	return ""
}

// handleDraftGroup handles the group count, label, node count, kind, and days phases.
func (m *shellModel) handleDraftGroup(input string) string {
	switch m.draft.phase {
	case draftPhaseGroupCount:
		n := 1
		if input != "" {
			if parsed, err := strconv.Atoi(input); err == nil && parsed >= 1 {
				n = parsed
			}
		}
		m.draft.groupTotal = n
		m.draft.currentGroupIdx = 0
		m.draft.groups = nil
		m.draft.currentGroup = wizardGroup{Kind: "module"}
		m.draft.phase = draftPhaseGroupLabel
		if n > 1 {
			return fmt.Sprintf("\n  --- Group 1 ---\n  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"):")
		}
		return "\n  Node label (e.g., \"Chapter\", \"Week\", \"Module\"):"

	case draftPhaseGroupLabel:
		label := input
		if label == "" {
			label = "Module"
		}
		m.draft.currentGroup.Label = label
		m.draft.phase = draftPhaseGroupNodeCount
		return "  How many?"

	case draftPhaseGroupNodeCount:
		count, err := strconv.Atoi(input)
		if err != nil || count < 1 {
			count = 1
		}
		m.draft.currentGroup.Count = count
		m.draft.phase = draftPhaseGroupKind
		return "  Node kind [module/week/section/stage/assessment/generic] (Enter for module):"

	case draftPhaseGroupKind:
		if input != "" {
			kind := strings.ToLower(input)
			if validNodeKinds[kind] {
				m.draft.currentGroup.Kind = kind
			}
		}
		m.draft.phase = draftPhaseGroupDays
		return "  Days per node (Enter to spread evenly):"

	case draftPhaseGroupDays:
		if input != "" {
			if days, err := strconv.Atoi(input); err == nil && days >= 1 {
				m.draft.currentGroup.DaysPer = days
			}
		}
		m.draft.groups = append(m.draft.groups, m.draft.currentGroup)
		m.draft.currentGroupIdx++

		if m.draft.currentGroupIdx < m.draft.groupTotal {
			m.draft.currentGroup = wizardGroup{Kind: "module"}
			m.draft.phase = draftPhaseGroupLabel
			return fmt.Sprintf("\n  --- Group %d ---\n  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"):", m.draft.currentGroupIdx+1)
		}
		m.draft.phase = draftPhaseWorkItemTitle
		return "\n  --- Work Items (applied to every node) ---\n  Title (Enter when done):"
	}
	return ""
}

// handleDraftWorkItem handles the work item title, type, and minutes phases.
func (m *shellModel) handleDraftWorkItem(input string) string {
	switch m.draft.phase {
	case draftPhaseWorkItemTitle:
		if input == "" {
			m.draft.phase = draftPhaseSpecialTitle
			return "\n  --- Special Nodes (exams, milestones — Enter to skip) ---\n  Title (Enter to skip):"
		}
		m.draft.currentWI = wizardWorkItem{Title: input, Type: "task"}
		m.draft.phase = draftPhaseWorkItemType
		return "    Type [reading/practice/review/assignment/task/quiz/study]:"

	case draftPhaseWorkItemType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				m.draft.currentWI.Type = t
			}
		}
		m.draft.phase = draftPhaseWorkItemMinutes
		return "    Estimated minutes:"

	case draftPhaseWorkItemMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				mins = 30
			}
			m.draft.currentWI.PlannedMin = mins
		}
		m.draft.workItems = append(m.draft.workItems, m.draft.currentWI)
		m.draft.phase = draftPhaseWorkItemTitle
		return "  Title (Enter when done):"
	}
	return ""
}

// handleDraftSpecialNode handles the special node title, kind, due date, and work item phases.
func (m *shellModel) handleDraftSpecialNode(input string) string {
	switch m.draft.phase {
	case draftPhaseSpecialTitle:
		if input == "" {
			return m.buildAndShowWizardDraft()
		}
		m.draft.currentSpecial = wizardSpecialNode{Title: input, Kind: "assessment"}
		m.draft.phase = draftPhaseSpecialKind
		return "    Kind [assessment/generic] (Enter for assessment):"

	case draftPhaseSpecialKind:
		if input != "" {
			kind := strings.ToLower(input)
			if kind == "assessment" || kind == "generic" {
				m.draft.currentSpecial.Kind = kind
			}
		}
		m.draft.phase = draftPhaseSpecialDueDate
		return "    Due date (YYYY-MM-DD, Enter for deadline):"

	case draftPhaseSpecialDueDate:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				m.draft.currentSpecial.DueDate = input
			}
		}
		m.draft.phase = draftPhaseSpecialWITitle
		return "    Work item title (Enter when done):"

	case draftPhaseSpecialWITitle:
		if input == "" {
			m.draft.specialNodes = append(m.draft.specialNodes, m.draft.currentSpecial)
			m.draft.phase = draftPhaseSpecialTitle
			return "  Title (Enter to skip):"
		}
		m.draft.currentSpecialWI = wizardWorkItem{Title: input, Type: "task"}
		m.draft.phase = draftPhaseSpecialWIType
		return "      Type [reading/practice/review/assignment/task/quiz/study]:"

	case draftPhaseSpecialWIType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				m.draft.currentSpecialWI.Type = t
			}
		}
		m.draft.phase = draftPhaseSpecialWIMinutes
		return "      Estimated minutes:"

	case draftPhaseSpecialWIMinutes:
		if input != "" {
			mins, err := strconv.Atoi(input)
			if err != nil || mins < 1 {
				mins = 30
			}
			m.draft.currentSpecialWI.PlannedMin = mins
		}
		m.draft.currentSpecial.WorkItems = append(m.draft.currentSpecial.WorkItems, m.draft.currentSpecialWI)
		m.draft.phase = draftPhaseSpecialWITitle
		return "    Work item title (Enter when done):"
	}
	return ""
}

func (m *shellModel) buildAndShowWizardDraft() string {
	wizard := &wizardResult{
		Description:  m.draft.description,
		StartDate:    m.draft.startDate,
		Deadline:     m.draft.deadline,
		Groups:       m.draft.groups,
		WorkItems:    m.draft.workItems,
		SpecialNodes: m.draft.specialNodes,
	}
	m.draft.wizard = wizard
	m.draft.schema = buildSchemaFromWizard(wizard)

	conv := &intelligence.DraftConversation{
		Draft:  m.draft.schema,
		Status: intelligence.DraftStatusReady,
	}
	m.draft.phase = draftPhaseWizardReview
	return formatter.FormatDraftPreview(conv) + m.wizardReviewPrompt()
}

func (m *shellModel) wizardReviewPrompt() string {
	if m.app.ProjectDraft != nil {
		return "\n[a]ccept  [r]efine with AI  [c]ancel:"
	}
	return "\n[a]ccept  [c]ancel:"
}

func (m *shellModel) handleWizardReviewInput(input string) string {
	switch strings.ToLower(input) {
	case "a", "accept":
		return m.acceptWizardSchema()
	case "c", "cancel":
		m.exitDraftMode()
		return "Draft cancelled."
	case "r", "refine":
		if m.app.ProjectDraft == nil {
			return "LLM features are disabled. Accept the draft or cancel." + m.wizardReviewPrompt()
		}
		desc := buildLLMDescription(m.draft.wizard)
		return m.startDraftConversation(desc, m.draft.schema)
	default:
		return "Invalid option." + m.wizardReviewPrompt()
	}
}

func (m *shellModel) acceptWizardSchema() string {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(m.draft.schema)
	if len(errs) > 0 {
		m.exitDraftMode()
		return formatter.FormatDraftValidationErrors(errs) + "Draft has validation errors."
	}

	result, err := m.app.Import.ImportProjectFromSchema(ctx, m.draft.schema)
	if err != nil {
		return formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err))
	}

	m.exitDraftMode()
	return formatter.FormatDraftAccepted(result)
}

func (m *shellModel) startDraftConversation(description string, preDraft *importer.ImportSchema) string {
	ctx := context.Background()

	var conv *intelligence.DraftConversation
	var err error

	if preDraft != nil {
		conv, err = m.app.ProjectDraft.StartWithDraft(ctx, description, preDraft)
	} else {
		conv, err = m.app.ProjectDraft.Start(ctx, description)
	}
	if err != nil {
		m.exitDraftMode()
		return formatter.StyleRed.Render(fmt.Sprintf("Failed to start project draft: %v", err))
	}
	m.draft.conv = conv

	var b strings.Builder
	b.WriteString(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		m.draft.phase = draftPhaseReview
		b.WriteString(formatter.FormatDraftReview(conv))
		b.WriteString("\n[a]ccept  [e]dit  [c]ancel:")
	} else {
		m.draft.phase = draftPhaseConversation
	}

	return b.String()
}

func (m *shellModel) handleDraftConversationInput(input string) string {
	if input == "" {
		return ""
	}

	lower := strings.ToLower(input)
	switch lower {
	case "/show", "/draft":
		return formatter.FormatDraftPreview(m.draft.conv)
	case "/accept":
		if m.draft.conv.Draft != nil {
			return m.acceptShellDraft()
		}
		return "No draft to accept yet."
	}

	ctx := context.Background()
	conv, err := m.app.ProjectDraft.NextTurn(ctx, m.draft.conv, input)
	if err != nil {
		return shellError(err)
	}
	m.draft.conv = conv

	var b strings.Builder
	b.WriteString(formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		m.draft.phase = draftPhaseReview
		b.WriteString(formatter.FormatDraftReview(conv))
		b.WriteString("\n[a]ccept  [e]dit  [c]ancel:")
	}

	return b.String()
}

func (m *shellModel) handleDraftReviewInput(input string) string {
	switch strings.ToLower(input) {
	case "a", "accept":
		return m.acceptShellDraft()
	case "c", "cancel":
		m.exitDraftMode()
		return "Draft cancelled."
	case "e", "edit":
		m.draft.conv.Status = intelligence.DraftStatusGathering
		m.draft.phase = draftPhaseConversation
		return "What would you like to change?"
	default:
		m.draft.conv.Status = intelligence.DraftStatusGathering
		m.draft.phase = draftPhaseConversation
		ctx := context.Background()
		conv, err := m.app.ProjectDraft.NextTurn(ctx, m.draft.conv, input)
		if err != nil {
			return shellError(err)
		}
		m.draft.conv = conv

		var b strings.Builder
		b.WriteString(formatter.FormatDraftTurn(conv))
		if conv.Status == intelligence.DraftStatusReady {
			m.draft.phase = draftPhaseReview
			b.WriteString(formatter.FormatDraftReview(conv))
			b.WriteString("\n[a]ccept  [e]dit  [c]ancel:")
		}
		return b.String()
	}
}

func (m *shellModel) acceptShellDraft() string {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(m.draft.conv.Draft)
	if len(errs) > 0 {
		m.draft.conv.Status = intelligence.DraftStatusGathering
		m.draft.phase = draftPhaseConversation
		return formatter.FormatDraftValidationErrors(errs) + "Draft has validation errors. Continue editing to fix them."
	}

	result, err := m.app.Import.ImportProjectFromSchema(ctx, m.draft.conv.Draft)
	if err != nil {
		return formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err))
	}

	m.exitDraftMode()
	return formatter.FormatDraftAccepted(result)
}

func (m *shellModel) exitDraftMode() {
	m.mode = modePrompt
	m.draft = nil
}

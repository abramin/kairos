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
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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

// draftView is a dedicated view for the project draft/creation flow.
// It supports two paths:
//  1. Wizard flow (no LLM): phase-by-phase interactive collection
//  2. LLM conversational flow: multi-turn AI drafting
type draftView struct {
	state *SharedState
	input textinput.Model
	draft *draftWizardState

	// transcript holds the conversation history displayed to the user.
	transcript []string
	// currentPrompt is the prompt for the current phase.
	currentPrompt string
}

func newDraftView(state *SharedState, description string) *draftView {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 500

	v := &draftView{
		state: state,
		input: ti,
		draft: &draftWizardState{},
	}

	if description != "" && state.App.ProjectDraft != nil {
		// LLM conversational flow: start with the description.
		description += "\nStart date: " + time.Now().Format("2006-01-02")
		v.startLLMConversation(description, nil)
	} else if description != "" && state.App.ProjectDraft == nil {
		// LLM disabled but description provided.
		v.transcript = append(v.transcript, formatter.StyleRed.Render(
			"LLM features are disabled. Using guided wizard instead."))
		v.startWizardFlow()
	} else {
		// Wizard flow.
		v.startWizardFlow()
	}

	return v
}

func (v *draftView) startWizardFlow() {
	v.transcript = append(v.transcript, formatter.FormatDraftWelcome())
	v.draft.phase = draftPhaseDescription
	v.currentPrompt = "  Describe your project:"
}

func (v *draftView) startLLMConversation(description string, preDraft *importer.ImportSchema) {
	ctx := context.Background()

	var conv *intelligence.DraftConversation
	var err error

	if preDraft != nil {
		conv, err = v.state.App.ProjectDraft.StartWithDraft(ctx, description, preDraft)
	} else {
		conv, err = v.state.App.ProjectDraft.Start(ctx, description)
	}
	if err != nil {
		v.transcript = append(v.transcript,
			formatter.StyleRed.Render(fmt.Sprintf("Failed to start project draft: %v", err)))
		v.startWizardFlow()
		return
	}
	v.draft.conv = conv
	v.transcript = append(v.transcript, formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		v.draft.phase = draftPhaseReview
		v.transcript = append(v.transcript, formatter.FormatDraftReview(conv))
		v.currentPrompt = "[a]ccept  [e]dit  [c]ancel:"
	} else {
		v.draft.phase = draftPhaseConversation
		v.currentPrompt = ""
	}
}

// ── tea.Model interface ──────────────────────────────────────────────────────

func (v *draftView) Init() tea.Cmd {
	return textinput.Blink
}

func (v *draftView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			v.transcript = append(v.transcript, formatter.Dim("Draft cancelled."))
			return v, func() tea.Msg {
				return wizardCompleteMsg{nextCmd: outputCmd(formatter.Dim("Draft cancelled."))}
			}
		}

		if msg.Type == tea.KeyEnter {
			input := v.input.Value()
			v.input.Reset()
			return v.handleInput(input)
		}

		var cmd tea.Cmd
		v.input, cmd = v.input.Update(msg)
		return v, cmd
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

func (v *draftView) View() string {
	var b strings.Builder

	// Show transcript.
	for _, line := range v.transcript {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show current prompt.
	if v.currentPrompt != "" {
		b.WriteString(v.currentPrompt)
		b.WriteString("\n")
	}

	// Show input.
	prompt := formatter.StylePurple.Render("draft") + formatter.Dim("> ")
	b.WriteString(prompt)
	b.WriteString(v.input.View())

	return b.String()
}

// ── View interface ───────────────────────────────────────────────────────────

func (v *draftView) ID() ViewID   { return ViewDraft }
func (v *draftView) Title() string { return "Draft" }
func (v *draftView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// ── input handling ───────────────────────────────────────────────────────────

func (v *draftView) handleInput(input string) (tea.Model, tea.Cmd) {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "/quit" || lower == "/cancel" || lower == "/q" {
		return v, func() tea.Msg {
			return wizardCompleteMsg{nextCmd: outputCmd(formatter.Dim("Draft cancelled."))}
		}
	}

	switch {
	case v.draft.phase <= draftPhaseDeadline:
		v.handleMetadata(input)
	case v.draft.phase <= draftPhaseGroupDays:
		v.handleGroup(input)
	case v.draft.phase <= draftPhaseWorkItemMinutes:
		v.handleWorkItem(input)
	case v.draft.phase <= draftPhaseSpecialWIMinutes:
		v.handleSpecialNode(input)
	case v.draft.phase == draftPhaseWizardReview:
		return v.handleWizardReview(input)
	case v.draft.phase == draftPhaseConversation:
		v.handleConversation(input)
	case v.draft.phase == draftPhaseReview:
		return v.handleLLMReview(input)
	}

	return v, nil
}

// ── wizard phase handlers ────────────────────────────────────────────────────

func (v *draftView) handleMetadata(input string) {
	switch v.draft.phase {
	case draftPhaseDescription:
		if input == "" {
			v.currentPrompt = formatter.StyleRed.Render("  Project description is required.")
			return
		}
		v.draft.description = input
		v.transcript = append(v.transcript, formatter.Dim("  Description: ")+input)
		v.draft.phase = draftPhaseStartDate
		v.currentPrompt = "  When do you want to start? (YYYY-MM-DD, or Enter for today)"

	case draftPhaseStartDate:
		v.draft.startDate = time.Now().Format("2006-01-02")
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				v.draft.startDate = input
			}
		}
		v.transcript = append(v.transcript, formatter.Dim("  Start: ")+v.draft.startDate)
		v.draft.phase = draftPhaseDeadline
		v.currentPrompt = "  When is the deadline? (YYYY-MM-DD, or Enter to skip)"

	case draftPhaseDeadline:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				v.draft.deadline = input
			}
		}
		if v.draft.deadline != "" {
			v.transcript = append(v.transcript, formatter.Dim("  Deadline: ")+v.draft.deadline)
		} else {
			v.transcript = append(v.transcript, formatter.Dim("  Deadline: none"))
		}
		v.draft.phase = draftPhaseGroupCount
		v.draft.groups = nil
		v.draft.workItems = nil
		v.draft.specialNodes = nil
		v.draft.groupTotal = 1
		v.draft.currentGroupIdx = 0
		v.currentPrompt = "  How many groups of work? (e.g., phases, levels — Enter for 1)"
	}
}

func (v *draftView) handleGroup(input string) {
	switch v.draft.phase {
	case draftPhaseGroupCount:
		n := 1
		if input != "" {
			if parsed, err := strconv.Atoi(input); err == nil && parsed >= 1 {
				n = parsed
			}
		}
		v.draft.groupTotal = n
		v.draft.currentGroupIdx = 0
		v.draft.groups = nil
		v.draft.currentGroup = wizardGroup{Kind: "module"}
		v.draft.phase = draftPhaseGroupLabel
		if n > 1 {
			v.currentPrompt = fmt.Sprintf("\n  --- Group 1 ---\n  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"):")
		} else {
			v.currentPrompt = "  Node label (e.g., \"Chapter\", \"Week\", \"Module\"):"
		}

	case draftPhaseGroupLabel:
		label := input
		if label == "" {
			label = "Module"
		}
		v.draft.currentGroup.Label = label
		v.draft.phase = draftPhaseGroupNodeCount
		v.currentPrompt = "  How many?"

	case draftPhaseGroupNodeCount:
		count, err := strconv.Atoi(input)
		if err != nil || count < 1 {
			count = 1
		}
		v.draft.currentGroup.Count = count
		v.draft.phase = draftPhaseGroupKind
		v.currentPrompt = "  Node kind [module/week/section/stage/assessment/generic] (Enter for module):"

	case draftPhaseGroupKind:
		if input != "" {
			kind := strings.ToLower(input)
			if validNodeKinds[kind] {
				v.draft.currentGroup.Kind = kind
			}
		}
		v.draft.phase = draftPhaseGroupDays
		v.currentPrompt = "  Days per node (Enter to spread evenly):"

	case draftPhaseGroupDays:
		if input != "" {
			if days, err := strconv.Atoi(input); err == nil && days >= 1 {
				v.draft.currentGroup.DaysPer = days
			}
		}
		v.draft.groups = append(v.draft.groups, v.draft.currentGroup)
		v.transcript = append(v.transcript,
			formatter.Dim(fmt.Sprintf("  Group: %s x%d (%s)",
				v.draft.currentGroup.Label, v.draft.currentGroup.Count, v.draft.currentGroup.Kind)))
		v.draft.currentGroupIdx++

		if v.draft.currentGroupIdx < v.draft.groupTotal {
			v.draft.currentGroup = wizardGroup{Kind: "module"}
			v.draft.phase = draftPhaseGroupLabel
			v.currentPrompt = fmt.Sprintf("\n  --- Group %d ---\n  Label (e.g., \"Chapter\", \"Week\"):", v.draft.currentGroupIdx+1)
		} else {
			v.draft.phase = draftPhaseWorkItemTitle
			v.currentPrompt = "\n  --- Work Items (applied to every node) ---\n  Title (Enter when done):"
		}
	}
}

func (v *draftView) handleWorkItem(input string) {
	switch v.draft.phase {
	case draftPhaseWorkItemTitle:
		if input == "" {
			v.draft.phase = draftPhaseSpecialTitle
			v.currentPrompt = "\n  --- Special Nodes (exams, milestones — Enter to skip) ---\n  Title (Enter to skip):"
			return
		}
		v.draft.currentWI = wizardWorkItem{Title: input, Type: "task"}
		v.draft.phase = draftPhaseWorkItemType
		v.currentPrompt = "    Type [reading/practice/review/assignment/task/quiz/study]:"

	case draftPhaseWorkItemType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				v.draft.currentWI.Type = t
			}
		}
		v.draft.phase = draftPhaseWorkItemMinutes
		v.currentPrompt = "    Estimated minutes:"

	case draftPhaseWorkItemMinutes:
		mins := 30
		if input != "" {
			parsed, err := strconv.Atoi(input)
			if err == nil && parsed >= 1 {
				mins = parsed
			}
		}
		v.draft.currentWI.PlannedMin = mins
		v.draft.workItems = append(v.draft.workItems, v.draft.currentWI)
		v.transcript = append(v.transcript,
			formatter.Dim(fmt.Sprintf("  + %s (%s, %dm)",
				v.draft.currentWI.Title, v.draft.currentWI.Type, v.draft.currentWI.PlannedMin)))
		v.draft.phase = draftPhaseWorkItemTitle
		v.currentPrompt = "  Title (Enter when done):"
	}
}

func (v *draftView) handleSpecialNode(input string) {
	switch v.draft.phase {
	case draftPhaseSpecialTitle:
		if input == "" {
			v.buildAndShowWizardDraft()
			return
		}
		v.draft.currentSpecial = wizardSpecialNode{Title: input, Kind: "assessment"}
		v.draft.phase = draftPhaseSpecialKind
		v.currentPrompt = "    Kind [assessment/generic] (Enter for assessment):"

	case draftPhaseSpecialKind:
		if input != "" {
			kind := strings.ToLower(input)
			if kind == "assessment" || kind == "generic" {
				v.draft.currentSpecial.Kind = kind
			}
		}
		v.draft.phase = draftPhaseSpecialDueDate
		v.currentPrompt = "    Due date (YYYY-MM-DD, Enter for deadline):"

	case draftPhaseSpecialDueDate:
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err == nil {
				v.draft.currentSpecial.DueDate = input
			}
		}
		v.draft.phase = draftPhaseSpecialWITitle
		v.currentPrompt = "    Work item title (Enter when done):"

	case draftPhaseSpecialWITitle:
		if input == "" {
			v.draft.specialNodes = append(v.draft.specialNodes, v.draft.currentSpecial)
			v.transcript = append(v.transcript,
				formatter.Dim(fmt.Sprintf("  + Special: %s (%s)", v.draft.currentSpecial.Title, v.draft.currentSpecial.Kind)))
			v.draft.phase = draftPhaseSpecialTitle
			v.currentPrompt = "  Title (Enter to skip):"
			return
		}
		v.draft.currentSpecialWI = wizardWorkItem{Title: input, Type: "task"}
		v.draft.phase = draftPhaseSpecialWIType
		v.currentPrompt = "      Type [reading/practice/review/assignment/task/quiz/study]:"

	case draftPhaseSpecialWIType:
		if input != "" {
			t := strings.ToLower(input)
			if validWorkItemTypes[t] {
				v.draft.currentSpecialWI.Type = t
			}
		}
		v.draft.phase = draftPhaseSpecialWIMinutes
		v.currentPrompt = "      Estimated minutes:"

	case draftPhaseSpecialWIMinutes:
		mins := 30
		if input != "" {
			parsed, err := strconv.Atoi(input)
			if err == nil && parsed >= 1 {
				mins = parsed
			}
		}
		v.draft.currentSpecialWI.PlannedMin = mins
		v.draft.currentSpecial.WorkItems = append(v.draft.currentSpecial.WorkItems, v.draft.currentSpecialWI)
		v.draft.phase = draftPhaseSpecialWITitle
		v.currentPrompt = "    Work item title (Enter when done):"
	}
}

func (v *draftView) buildAndShowWizardDraft() {
	wizard := &wizardResult{
		Description:  v.draft.description,
		StartDate:    v.draft.startDate,
		Deadline:     v.draft.deadline,
		Groups:       v.draft.groups,
		WorkItems:    v.draft.workItems,
		SpecialNodes: v.draft.specialNodes,
	}
	v.draft.wizard = wizard
	v.draft.schema = buildSchemaFromWizard(wizard)

	conv := &intelligence.DraftConversation{
		Draft:  v.draft.schema,
		Status: intelligence.DraftStatusReady,
	}
	v.draft.phase = draftPhaseWizardReview
	v.transcript = append(v.transcript, formatter.FormatDraftPreview(conv))

	if v.state.App.ProjectDraft != nil {
		v.currentPrompt = "[a]ccept  [r]efine with AI  [c]ancel:"
	} else {
		v.currentPrompt = "[a]ccept  [c]ancel:"
	}
}

func (v *draftView) handleWizardReview(input string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(input) {
	case "a", "accept":
		return v.acceptWizardSchema()
	case "c", "cancel":
		return v, func() tea.Msg {
			return wizardCompleteMsg{nextCmd: outputCmd(formatter.Dim("Draft cancelled."))}
		}
	case "r", "refine":
		if v.state.App.ProjectDraft == nil {
			v.currentPrompt = "LLM features are disabled. Accept the draft or cancel."
			return v, nil
		}
		desc := buildLLMDescription(v.draft.wizard)
		v.startLLMConversation(desc, v.draft.schema)
		return v, nil
	default:
		v.currentPrompt = "Invalid option. [a]ccept  [c]ancel:"
		return v, nil
	}
}

func (v *draftView) acceptWizardSchema() (tea.Model, tea.Cmd) {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(v.draft.schema)
	if len(errs) > 0 {
		v.transcript = append(v.transcript,
			formatter.FormatDraftValidationErrors(errs)+"Draft has validation errors.")
		return v, nil
	}

	result, err := v.state.App.Import.ImportProjectFromSchema(ctx, v.draft.schema)
	if err != nil {
		v.transcript = append(v.transcript,
			formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err)))
		return v, nil
	}

	msg := formatter.FormatDraftAccepted(result)
	return v, func() tea.Msg {
		return wizardCompleteMsg{nextCmd: outputCmd(msg)}
	}
}

// ── LLM conversation handlers ───────────────────────────────────────────────

func (v *draftView) handleConversation(input string) {
	if input == "" {
		return
	}

	lower := strings.ToLower(input)
	switch lower {
	case "/show", "/draft":
		v.transcript = append(v.transcript, formatter.FormatDraftPreview(v.draft.conv))
		return
	case "/accept":
		if v.draft.conv.Draft != nil {
			// Will be handled by handleLLMReview.
			v.draft.phase = draftPhaseReview
			v.currentPrompt = "[a]ccept  [e]dit  [c]ancel:"
			return
		}
		v.transcript = append(v.transcript, "No draft to accept yet.")
		return
	}

	v.transcript = append(v.transcript, formatter.Dim("You: ")+input)

	ctx := context.Background()
	conv, err := v.state.App.ProjectDraft.NextTurn(ctx, v.draft.conv, input)
	if err != nil {
		v.transcript = append(v.transcript, shellError(err))
		return
	}
	v.draft.conv = conv
	v.transcript = append(v.transcript, formatter.FormatDraftTurn(conv))

	if conv.Status == intelligence.DraftStatusReady {
		v.draft.phase = draftPhaseReview
		v.transcript = append(v.transcript, formatter.FormatDraftReview(conv))
		v.currentPrompt = "[a]ccept  [e]dit  [c]ancel:"
	}
}

func (v *draftView) handleLLMReview(input string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(input) {
	case "a", "accept":
		return v.acceptLLMDraft()
	case "c", "cancel":
		return v, func() tea.Msg {
			return wizardCompleteMsg{nextCmd: outputCmd(formatter.Dim("Draft cancelled."))}
		}
	case "e", "edit":
		v.draft.conv.Status = intelligence.DraftStatusGathering
		v.draft.phase = draftPhaseConversation
		v.currentPrompt = ""
		v.transcript = append(v.transcript, "What would you like to change?")
		return v, nil
	default:
		// Treat as a refinement message.
		v.draft.conv.Status = intelligence.DraftStatusGathering
		v.draft.phase = draftPhaseConversation
		v.handleConversation(input)
		return v, nil
	}
}

func (v *draftView) acceptLLMDraft() (tea.Model, tea.Cmd) {
	ctx := context.Background()
	errs := importer.ValidateImportSchema(v.draft.conv.Draft)
	if len(errs) > 0 {
		v.draft.conv.Status = intelligence.DraftStatusGathering
		v.draft.phase = draftPhaseConversation
		v.transcript = append(v.transcript,
			formatter.FormatDraftValidationErrors(errs)+"Draft has validation errors. Continue editing to fix them.")
		v.currentPrompt = ""
		return v, nil
	}

	result, err := v.state.App.Import.ImportProjectFromSchema(ctx, v.draft.conv.Draft)
	if err != nil {
		v.transcript = append(v.transcript,
			formatter.StyleRed.Render(fmt.Sprintf("Import failed: %v", err)))
		return v, nil
	}

	msg := formatter.FormatDraftAccepted(result)
	return v, func() tea.Msg {
		return wizardCompleteMsg{nextCmd: outputCmd(msg)}
	}
}

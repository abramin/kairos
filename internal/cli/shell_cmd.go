package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// pendingConfirmation holds state for a destructive command awaiting y/n input.
type pendingConfirmation struct {
	description string   // e.g., "Remove project Physics (PHI01)?"
	args        []string // original command args to re-execute on confirm
}

// destructiveCommands maps command groups to subcommands that require confirmation.
var destructiveCommands = map[string]map[string]bool{
	"project": {"remove": true, "archive": true},
	"node":    {"remove": true},
	"work":    {"remove": true, "archive": true},
	"session": {"remove": true},
}

func newShellCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Start kairos (interactive mode)",
		Long: `Start kairos in interactive mode with project context,
autocomplete, and styled output. This is the default when
running kairos with no arguments on a terminal.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(app)
		},
	}
}

func runShell(app *App) error {
	m := newShellModel(app)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

// ── exec* methods (return strings for bubbletea) ─────────────────────────────

func (m *shellModel) execProjects() string {
	ctx := context.Background()
	projects, err := m.app.Projects.List(ctx, false)
	if err != nil {
		return shellError(err)
	}
	if len(projects) == 0 {
		return formatter.Dim("No projects found.")
	}
	return formatter.FormatProjectList(projects)
}

func (m *shellModel) execUse(args []string) string {
	if len(args) == 0 {
		m.clearContext()
		return formatter.Dim("Cleared active project.")
	}

	ctx := context.Background()
	projectID, err := resolveProjectID(ctx, m.app, args[0])
	if err != nil {
		return shellError(err)
	}

	project, err := m.app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return shellError(err)
	}

	m.setActiveProjectFrom(project)
	// Clear item context when switching projects.
	m.activeItemID = ""
	m.activeItemTitle = ""
	m.activeItemSeq = 0

	return fmt.Sprintf("Active project: %s %s",
		formatter.Bold(project.Name),
		formatter.Dim(m.activeShortID),
	)
}

func (m *shellModel) execInspect(args []string) string {
	ctx := context.Background()

	var targetID string
	if len(args) > 0 {
		resolved, err := resolveProjectID(ctx, m.app, args[0])
		if err != nil {
			return shellError(err)
		}
		targetID = resolved
	} else if m.activeProjectID != "" {
		targetID = m.activeProjectID
	} else {
		return formatter.StyleYellow.Render(
			"No active project. Use 'use <id>' to select one, or 'inspect <id>'.")
	}

	output, err := m.buildInspectOutput(ctx, targetID)
	if err != nil {
		return shellError(err)
	}
	m.lastInspectedProjectID = targetID

	// Implicit context: set active project if not set.
	if m.activeProjectID == "" {
		if p, e := m.app.Projects.GetByID(ctx, targetID); e == nil {
			m.setActiveProjectFrom(p)
		}
	}
	return output
}

func (m *shellModel) buildInspectOutput(ctx context.Context, projectID string) (string, error) {
	p, err := m.app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}

	rootNodes, err := m.app.Nodes.ListRoots(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("listing root nodes: %w", err)
	}

	childMap := make(map[string][]*domain.PlanNode)
	workItems := make(map[string][]*domain.WorkItem)

	var fetchErr error
	var fetchChildren func(nodes []*domain.PlanNode)
	fetchChildren = func(nodes []*domain.PlanNode) {
		for _, n := range nodes {
			if fetchErr != nil {
				return
			}
			children, err := m.app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				fetchErr = fmt.Errorf("listing children of node %s: %w", n.ID, err)
				return
			}
			if len(children) > 0 {
				childMap[n.ID] = children
				fetchChildren(children)
			}
			items, err := m.app.WorkItems.ListByNode(ctx, n.ID)
			if err != nil {
				fetchErr = fmt.Errorf("listing work items for node %s: %w", n.ID, err)
				return
			}
			if len(items) > 0 {
				workItems[n.ID] = items
			}
		}
	}
	fetchChildren(rootNodes)
	if fetchErr != nil {
		return "", fetchErr
	}

	data := formatter.ProjectInspectData{
		Project:   p,
		RootNodes: rootNodes,
		ChildMap:  childMap,
		WorkItems: workItems,
	}

	return formatter.FormatProjectInspect(data), nil
}

func (m *shellModel) execStatus() string {
	ctx := context.Background()
	req := contract.NewStatusRequest()
	if m.activeProjectID != "" {
		req.ProjectScope = []string{m.activeProjectID}
	}
	resp, err := m.app.Status.GetStatus(ctx, req)
	if err != nil {
		return shellError(err)
	}
	return formatter.FormatStatus(resp)
}

func (m *shellModel) execWhatNow(args []string) string {
	minutes := 60
	if len(args) > 0 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 {
			return formatter.StyleRed.Render(
				fmt.Sprintf("Invalid minutes %q — expected a positive integer.", args[0]))
		}
		minutes = v
	}

	ctx := context.Background()
	req := contract.NewWhatNowRequest(minutes)
	if m.activeProjectID != "" {
		req.ProjectScope = []string{m.activeProjectID}
	}

	resp, err := m.app.WhatNow.Recommend(ctx, req)
	if err != nil {
		return shellError(err)
	}
	if len(resp.Recommendations) > 0 {
		rec := resp.Recommendations[0]
		m.lastRecommendedItemID = rec.WorkItemID
		m.lastRecommendedItemTitle = rec.Title
		// Implicit context: set active item from top recommendation.
		m.activeItemID = rec.WorkItemID
		m.activeItemTitle = rec.Title
	}
	return formatWhatNowResponse(ctx, m.app, resp)
}

// ── New shortcut commands ────────────────────────────────────────────────────

// execLog handles the "log" shell shortcut for logging a work session.
// It uses the progressive disclosure pattern: fill from context, then wizard for missing.
func (m *shellModel) execLog(args []string) (string, tea.Cmd) {
	ctx := context.Background()

	// Parse inline args: "log 60" or "log #5 45"
	var itemArg, minutesArg string
	for _, a := range args {
		if v, err := strconv.Atoi(a); err == nil && v > 0 {
			if minutesArg == "" {
				minutesArg = a
			}
		} else if strings.HasPrefix(a, "#") {
			itemArg = a[1:]
		} else {
			itemArg = a
		}
	}

	// Resolve project.
	projectID := m.activeProjectID
	if projectID == "" {
		// Need wizard for project selection.
		var result string
		form := wizardSelectProject(ctx, m.app, &result)
		if form == nil {
			return formatter.StyleYellow.Render("No projects found. Create one first with 'draft'."), nil
		}
		cmd := m.startWizard(form, func(m *shellModel) tea.Cmd {
			m.setActiveProject(ctx, result)
			return m.logAfterProject(itemArg, minutesArg)
		})
		return "", cmd
	}

	return "", m.logAfterProject(itemArg, minutesArg)
}

func (m *shellModel) logAfterProject(itemArg, minutesArg string) tea.Cmd {
	ctx := context.Background()

	// Resolve item.
	itemID := ""
	if itemArg != "" {
		resolved, err := resolveWorkItemID(ctx, m.app, itemArg, m.activeProjectID)
		if err == nil {
			itemID = resolved
		}
	}
	if itemID == "" && m.activeItemID != "" {
		itemID = m.activeItemID
	}
	if itemID == "" && m.lastRecommendedItemID != "" {
		itemID = m.lastRecommendedItemID
	}

	if itemID == "" {
		// Need wizard for item selection.
		var result string
		form := wizardSelectWorkItem(ctx, m.app, m.activeProjectID, nil, &result)
		if form == nil {
			return tea.Println(formatter.StyleYellow.Render("No work items found in this project."))
		}
		return m.startWizard(form, func(m *shellModel) tea.Cmd {
			return m.logAfterItem(result, minutesArg)
		})
	}

	return m.logAfterItem(itemID, minutesArg)
}

func (m *shellModel) logAfterItem(itemID, minutesArg string) tea.Cmd {
	// Resolve duration.
	if minutesArg != "" {
		return m.logExecute(itemID, minutesArg)
	}

	defaultMin := 60
	if m.lastDuration > 0 {
		defaultMin = m.lastDuration
	}

	var result string
	form := wizardInputDuration(defaultMin, &result)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		if result == "" {
			result = strconv.Itoa(defaultMin)
		}
		return m.logExecute(itemID, result)
	})
}

func (m *shellModel) logExecute(itemID, minutesStr string) tea.Cmd {
	ctx := context.Background()
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		return tea.Println(formatter.StyleRed.Render("Invalid duration."))
	}

	s := &domain.WorkSessionLog{
		ID:         uuid.New().String(),
		WorkItemID: itemID,
		StartedAt:  time.Now(),
		Minutes:    minutes,
		CreatedAt:  time.Now(),
	}
	if err := m.app.Sessions.LogSession(ctx, s); err != nil {
		return tea.Println(shellError(err))
	}

	// Update context.
	m.activeItemID = itemID
	m.lastDuration = minutes

	// Get item title for display.
	title := formatter.TruncID(itemID)
	if wi, err := m.app.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
		m.activeItemTitle = wi.Title
		m.activeItemSeq = wi.Seq
	}

	return tea.Println(fmt.Sprintf("%s Logged %s to %s.",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(formatter.FormatMinutes(minutes)),
		formatter.Bold(title),
	))
}

// execStart handles the "start" shell shortcut to mark a work item as in-progress.
func (m *shellModel) execStart(args []string) (string, tea.Cmd) {
	ctx := context.Background()

	// Parse inline item arg.
	var itemArg string
	if len(args) > 0 {
		itemArg = args[0]
		if strings.HasPrefix(itemArg, "#") {
			itemArg = itemArg[1:]
		}
	}

	projectID := m.activeProjectID
	if projectID == "" {
		var result string
		form := wizardSelectProject(ctx, m.app, &result)
		if form == nil {
			return formatter.StyleYellow.Render("No projects found."), nil
		}
		cmd := m.startWizard(form, func(m *shellModel) tea.Cmd {
			m.setActiveProject(ctx, result)
			return m.startAfterProject(itemArg)
		})
		return "", cmd
	}

	return "", m.startAfterProject(itemArg)
}

func (m *shellModel) startAfterProject(itemArg string) tea.Cmd {
	ctx := context.Background()

	if itemArg != "" {
		resolved, err := resolveWorkItemID(ctx, m.app, itemArg, m.activeProjectID)
		if err == nil {
			return m.startExecute(resolved)
		}
	}

	// Wizard: select from todo items.
	var result string
	form := wizardSelectWorkItem(ctx, m.app, m.activeProjectID,
		[]domain.WorkItemStatus{domain.WorkItemTodo}, &result)
	if form == nil {
		return tea.Println(formatter.StyleYellow.Render("No todo items found."))
	}
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.startExecute(result)
	})
}

func (m *shellModel) startExecute(itemID string) tea.Cmd {
	ctx := context.Background()
	if err := m.app.WorkItems.MarkInProgress(ctx, itemID); err != nil {
		return tea.Println(shellError(err))
	}

	m.activeItemID = itemID
	title := formatter.TruncID(itemID)
	if wi, err := m.app.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
		m.activeItemTitle = wi.Title
		m.activeItemSeq = wi.Seq
	}

	return tea.Println(fmt.Sprintf("%s Started: %s",
		formatter.StyleGreen.Render("▶"),
		formatter.Bold(title),
	))
}

// execFinish handles the "finish" shell shortcut to mark work item as done.
func (m *shellModel) execFinish(args []string) (string, tea.Cmd) {
	ctx := context.Background()

	var itemArg string
	if len(args) > 0 {
		itemArg = args[0]
		if strings.HasPrefix(itemArg, "#") {
			itemArg = itemArg[1:]
		}
	}

	// Try to resolve from arg or active context.
	if itemArg != "" {
		projectID := m.activeProjectID
		resolved, err := resolveWorkItemID(ctx, m.app, itemArg, projectID)
		if err == nil {
			return "", m.finishExecute(resolved)
		}
	}

	if m.activeItemID != "" {
		return "", m.finishExecute(m.activeItemID)
	}

	// Need wizard: select from in-progress items.
	projectID := m.activeProjectID
	if projectID == "" {
		var result string
		form := wizardSelectProject(ctx, m.app, &result)
		if form == nil {
			return formatter.StyleYellow.Render("No projects found."), nil
		}
		cmd := m.startWizard(form, func(m *shellModel) tea.Cmd {
			m.setActiveProject(ctx, result)
			return m.finishAfterProject()
		})
		return "", cmd
	}

	return "", m.finishAfterProject()
}

func (m *shellModel) finishAfterProject() tea.Cmd {
	ctx := context.Background()
	var result string
	form := wizardSelectWorkItem(ctx, m.app, m.activeProjectID,
		[]domain.WorkItemStatus{domain.WorkItemInProgress}, &result)
	if form == nil {
		return tea.Println(formatter.StyleYellow.Render("No in-progress items found."))
	}
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.finishExecute(result)
	})
}

func (m *shellModel) finishExecute(itemID string) tea.Cmd {
	ctx := context.Background()
	if err := m.app.WorkItems.MarkDone(ctx, itemID); err != nil {
		return tea.Println(shellError(err))
	}

	title := formatter.TruncID(itemID)
	if wi, err := m.app.WorkItems.GetByID(ctx, itemID); err == nil {
		title = wi.Title
	}

	// Clear active item if it's the one we just finished.
	if m.activeItemID == itemID {
		m.activeItemID = ""
		m.activeItemTitle = ""
		m.activeItemSeq = 0
	}

	return tea.Println(fmt.Sprintf("%s Finished: %s",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(title),
	))
}

// execContext shows or modifies the active shell context.
func (m *shellModel) execContext(args []string) string {
	if len(args) == 0 {
		return m.formatContext()
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "clear":
		m.clearContext()
		m.lastDuration = 0
		return formatter.Dim("Context cleared.")
	case "project":
		if len(args) < 2 {
			return formatter.StyleYellow.Render("Usage: context project <id>")
		}
		return m.execUse(args[1:])
	case "item":
		if len(args) < 2 {
			return formatter.StyleYellow.Render("Usage: context item <id>")
		}
		return m.setActiveItem(args[1])
	default:
		return formatter.StyleYellow.Render("Usage: context [clear | project <id> | item <id>]")
	}
}

func (m *shellModel) formatContext() string {
	var b strings.Builder
	b.WriteString(formatter.Header("Active Context"))
	b.WriteString("\n")

	if m.activeProjectID != "" {
		b.WriteString(fmt.Sprintf("  Project: %s %s\n",
			formatter.Bold(m.activeProjectName),
			formatter.Dim(m.activeShortID)))
	} else {
		b.WriteString(fmt.Sprintf("  Project: %s\n", formatter.Dim("none")))
	}

	if m.activeItemID != "" {
		label := m.activeItemTitle
		if m.activeItemSeq > 0 {
			label = fmt.Sprintf("#%d — %s", m.activeItemSeq, label)
		}
		b.WriteString(fmt.Sprintf("  Item:    %s\n", formatter.Bold(label)))
	} else {
		b.WriteString(fmt.Sprintf("  Item:    %s\n", formatter.Dim("none")))
	}

	if m.lastDuration > 0 {
		b.WriteString(fmt.Sprintf("  Duration: %s (last logged)\n",
			formatter.Bold(formatter.FormatMinutes(m.lastDuration))))
	} else {
		b.WriteString(fmt.Sprintf("  Duration: %s\n", formatter.Dim("60m (default)")))
	}

	return b.String()
}

func (m *shellModel) setActiveItem(input string) string {
	ctx := context.Background()
	if m.activeProjectID == "" {
		return formatter.StyleYellow.Render("Set a project first: 'use <id>'")
	}
	resolved, err := resolveWorkItemID(ctx, m.app, input, m.activeProjectID)
	if err != nil {
		return shellError(err)
	}
	wi, err := m.app.WorkItems.GetByID(ctx, resolved)
	if err != nil {
		return shellError(err)
	}
	m.activeItemID = wi.ID
	m.activeItemTitle = wi.Title
	m.activeItemSeq = wi.Seq
	return fmt.Sprintf("Active item: %s #%d — %s",
		formatter.StyleGreen.Render("▶"),
		wi.Seq, formatter.Bold(wi.Title))
}

// ── wizard-ified bare commands ───────────────────────────────────────────────

// shouldStartWizard returns true when a creation/log command has no flags.
func (m *shellModel) shouldStartWizard(parts []string) bool {
	if len(parts) != 2 {
		return false
	}
	group := strings.ToLower(parts[0])
	sub := strings.ToLower(parts[1])
	switch group {
	case "work":
		return sub == "add"
	case "session":
		return sub == "log"
	case "node":
		return sub == "add"
	}
	return false
}

// execWizardForCommand launches wizard forms for bare node/work/session commands.
func (m *shellModel) execWizardForCommand(parts []string) (string, tea.Cmd) {
	cmd := strings.ToLower(parts[0]) + " " + strings.ToLower(parts[1])

	switch cmd {
	case "session log":
		return m.execLog(nil)
	case "work add":
		return m.wizardWorkAdd()
	case "node add":
		return m.wizardNodeAdd()
	}
	return m.execCobraCapture(parts), nil
}

func (m *shellModel) wizardWorkAdd() (string, tea.Cmd) {
	ctx := context.Background()

	projectID := m.activeProjectID
	if projectID == "" {
		var result string
		form := wizardSelectProject(ctx, m.app, &result)
		if form == nil {
			return formatter.StyleYellow.Render("No projects found."), nil
		}
		cmd := m.startWizard(form, func(m *shellModel) tea.Cmd {
			m.setActiveProject(ctx, result)
			return m.workAddSelectNode()
		})
		return "", cmd
	}
	return "", m.workAddSelectNode()
}

func (m *shellModel) workAddSelectNode() tea.Cmd {
	ctx := context.Background()
	var nodeID string
	form := wizardSelectNode(ctx, m.app, m.activeProjectID, &nodeID)
	if form == nil {
		return tea.Println(formatter.StyleYellow.Render("No nodes found in this project."))
	}
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.workAddGetTitle(nodeID)
	})
}

func (m *shellModel) workAddGetTitle(nodeID string) tea.Cmd {
	var title string
	form := wizardInputText("Title", "Work item title", true, &title)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.workAddGetType(nodeID, title)
	})
}

func (m *shellModel) workAddGetType(nodeID, title string) tea.Cmd {
	var wiType string
	form := wizardSelectWorkItemType(&wiType)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.workAddGetMinutes(nodeID, title, wiType)
	})
}

func (m *shellModel) workAddGetMinutes(nodeID, title, wiType string) tea.Cmd {
	var minutes string
	form := wizardInputDuration(60, &minutes)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		args := []string{"work", "add",
			"--node", nodeID,
			"--title", title,
			"--type", wiType,
		}
		if v, err := strconv.Atoi(minutes); err == nil && v > 0 {
			args = append(args, "--planned-min", minutes)
		}
		output := m.execCobraCapture(args)
		return tea.Println(output)
	})
}

func (m *shellModel) wizardNodeAdd() (string, tea.Cmd) {
	ctx := context.Background()

	projectID := m.activeProjectID
	if projectID == "" {
		var result string
		form := wizardSelectProject(ctx, m.app, &result)
		if form == nil {
			return formatter.StyleYellow.Render("No projects found."), nil
		}
		cmd := m.startWizard(form, func(m *shellModel) tea.Cmd {
			m.setActiveProject(ctx, result)
			return m.nodeAddGetTitle()
		})
		return "", cmd
	}
	return "", m.nodeAddGetTitle()
}

func (m *shellModel) nodeAddGetTitle() tea.Cmd {
	var title string
	form := wizardInputText("Title", "Node title", true, &title)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		return m.nodeAddGetKind(title)
	})
}

func (m *shellModel) nodeAddGetKind(title string) tea.Cmd {
	var kind string
	form := wizardSelectNodeKind(&kind)
	return m.startWizard(form, func(m *shellModel) tea.Cmd {
		args := []string{"node", "add",
			"--project", m.activeProjectID,
			"--title", title,
			"--kind", kind,
		}
		output := m.execCobraCapture(args)
		return tea.Println(output)
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

// shellError formats an error for display in the shell REPL.
func shellError(err error) string {
	return formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err))
}

// clearContext resets all active-project and active-item state.
func (m *shellModel) clearContext() {
	m.activeProjectID = ""
	m.activeShortID = ""
	m.activeProjectName = ""
	m.activeItemID = ""
	m.activeItemTitle = ""
	m.activeItemSeq = 0
}

// setActiveProjectFrom sets the active project context from a domain.Project.
func (m *shellModel) setActiveProjectFrom(p *domain.Project) {
	m.activeProjectID = p.ID
	m.activeShortID = p.ShortID
	if m.activeShortID == "" && len(p.ID) >= 6 {
		m.activeShortID = p.ID[:6]
	}
	m.activeProjectName = p.Name
}

// setActiveProject resolves and sets the active project context from a UUID.
func (m *shellModel) setActiveProject(ctx context.Context, projectID string) {
	p, err := m.app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return
	}
	m.setActiveProjectFrom(p)
}

// ── splitShellArgs and prepareShellCobraArgs (unchanged) ─────────────────────

func splitShellArgs(input string) ([]string, error) {
	var parts []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false
	tokenStarted := false

	flush := func() {
		parts = append(parts, cur.String())
		cur.Reset()
		tokenStarted = false
	}

	for _, r := range input {
		if escaped {
			cur.WriteRune(r)
			tokenStarted = true
			escaped = false
			continue
		}

		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
			tokenStarted = true
			continue
		}

		if inDouble {
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
			tokenStarted = true
			continue
		}

		switch r {
		case '\\':
			escaped = true
			tokenStarted = true
		case '\'':
			inSingle = true
			tokenStarted = true
		case '"':
			inDouble = true
			tokenStarted = true
		case ' ', '\t', '\n', '\r':
			if tokenStarted {
				flush()
			}
		default:
			cur.WriteRune(r)
			tokenStarted = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if tokenStarted {
		flush()
	}

	return parts, nil
}

func prepareShellCobraArgs(args []string, activeProjectID string) []string {
	if activeProjectID == "" || len(args) == 0 {
		return args
	}

	group := strings.ToLower(args[0])
	if group != "node" && group != "work" && group != "session" {
		return args
	}

	if group == "work" && len(args) >= 2 && strings.ToLower(args[1]) == "add" {
		return args
	}

	for _, a := range args {
		if a == "--project" || strings.HasPrefix(a, "--project=") {
			return args
		}
	}

	return append(args, "--project", activeProjectID)
}

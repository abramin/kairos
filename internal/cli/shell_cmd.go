package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	prompt "github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
)

// shellSession holds mutable state across the REPL loop.
type shellSession struct {
	app               *App
	activeProjectID   string
	activeShortID     string
	activeProjectName string
	cache             *shellProjectCache
	helpChatMode      bool
	helpSpecJSON      string
	helpCmdInfos      []intelligence.HelpCommandInfo
	helpConv          *intelligence.HelpConversation
	draftMode         bool
	draftPhase        draftPhase
	draftDescription  string
	draftStartDate    string
	draftDeadline     string
	draftStructure    string
	draftConv         *intelligence.DraftConversation
	wantExit          bool
}

func newShellCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Interactive shell with session state and autocomplete",
		Long: `Start an interactive shell session with project context,
autocomplete, and styled output. Maintains active project state
across commands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(app)
		},
	}
}

func runShell(app *App) error {
	sess := &shellSession{
		app:   app,
		cache: newShellProjectCache(),
	}

	fmt.Print(formatter.FormatShellWelcome())

	p := prompt.New(
		sess.executor,
		sess.completer,
		prompt.OptionLivePrefix(sess.livePrefix),
		prompt.OptionSetExitCheckerOnInput(func(in string, breakline bool) bool {
			return sess.wantExit
		}),
		prompt.OptionTitle("kairos shell"),
		prompt.OptionPrefixTextColor(prompt.Purple),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.Purple),
		prompt.OptionSelectedSuggestionTextColor(prompt.White),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(prompt.LightGray),
		prompt.OptionMaxSuggestion(10),
	)
	p.Run()
	return nil
}

func (s *shellSession) livePrefix() (string, bool) {
	if s.helpChatMode {
		return "help> ", true
	}
	if s.draftMode {
		return "draft> ", true
	}
	if s.activeProjectID == "" {
		return "kairos ❯ ", true
	}
	return fmt.Sprintf("kairos (%s) ❯ ", s.activeShortID), true
}

func (s *shellSession) executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" && !s.draftMode && !s.helpChatMode {
		return
	}
	if s.draftMode {
		s.execDraftTurn(input)
		return
	}
	if s.helpChatMode {
		s.execHelpChatTurn(input)
		return
	}

	parts, err := splitShellArgs(input)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "projects":
		s.execProjects()
	case "use":
		s.execUse(args)
	case "inspect":
		s.execInspect(args)
	case "status":
		s.execStatus()
	case "what-now":
		s.execWhatNow(args)
	case "clear":
		s.execClear()
	case "help":
		if len(args) > 0 && args[0] == "chat" {
			s.execHelpChat(args[1:])
		} else {
			s.execHelp()
		}
	case "exit", "quit":
		fmt.Println(formatter.Dim("Goodbye."))
		s.wantExit = true
	case "shell":
		fmt.Println(formatter.StyleYellow.Render("Already in shell mode."))
	case "project":
		if len(args) > 0 && args[0] == "draft" {
			s.execDraft(args[1:])
		} else {
			s.execCobra(parts)
		}
	default:
		s.execCobra(parts)
	}
}

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

func (s *shellSession) execProjects() {
	ctx := context.Background()
	projects, err := s.app.Projects.List(ctx, false)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	if len(projects) == 0 {
		fmt.Println(formatter.Dim("No projects found."))
		return
	}
	fmt.Printf("%s\n", formatter.FormatProjectList(projects))
}

func (s *shellSession) execUse(args []string) {
	if len(args) == 0 {
		s.activeProjectID = ""
		s.activeShortID = ""
		s.activeProjectName = ""
		fmt.Println(formatter.Dim("Cleared active project."))
		return
	}

	ctx := context.Background()
	projectID, err := resolveProjectID(ctx, s.app, args[0])
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	project, err := s.app.Projects.GetByID(ctx, projectID)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	s.activeProjectID = project.ID
	s.activeShortID = project.ShortID
	if s.activeShortID == "" && len(project.ID) >= 6 {
		s.activeShortID = project.ID[:6]
	}
	s.activeProjectName = project.Name

	fmt.Printf("Active project: %s %s\n",
		formatter.Bold(project.Name),
		formatter.Dim(s.activeShortID),
	)
}

func (s *shellSession) execInspect(args []string) {
	ctx := context.Background()

	var targetID string
	if len(args) > 0 {
		resolved, err := resolveProjectID(ctx, s.app, args[0])
		if err != nil {
			fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
			return
		}
		targetID = resolved
	} else if s.activeProjectID != "" {
		targetID = s.activeProjectID
	} else {
		fmt.Println(formatter.StyleYellow.Render(
			"No active project. Use 'use <id>' to select one, or 'inspect <id>'."))
		return
	}

	output, err := s.buildInspectOutput(ctx, targetID)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	fmt.Printf("%s\n", output)
}

// buildInspectOutput fetches project data and returns formatted output.
// This mirrors the logic in newProjectInspectCmd.
func (s *shellSession) buildInspectOutput(ctx context.Context, projectID string) (string, error) {
	p, err := s.app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}

	rootNodes, _ := s.app.Nodes.ListRoots(ctx, projectID)
	childMap := make(map[string][]*domain.PlanNode)
	workItems := make(map[string][]*domain.WorkItem)

	var fetchChildren func(nodes []*domain.PlanNode)
	fetchChildren = func(nodes []*domain.PlanNode) {
		for _, n := range nodes {
			children, _ := s.app.Nodes.ListChildren(ctx, n.ID)
			if len(children) > 0 {
				childMap[n.ID] = children
				fetchChildren(children)
			}
			items, _ := s.app.WorkItems.ListByNode(ctx, n.ID)
			if len(items) > 0 {
				workItems[n.ID] = items
			}
		}
	}
	fetchChildren(rootNodes)

	data := formatter.ProjectInspectData{
		Project:   p,
		RootNodes: rootNodes,
		ChildMap:  childMap,
		WorkItems: workItems,
	}

	return formatter.FormatProjectInspect(data), nil
}

func (s *shellSession) execStatus() {
	ctx := context.Background()
	req := contract.NewStatusRequest()
	if s.activeProjectID != "" {
		req.ProjectScope = []string{s.activeProjectID}
	}
	resp, err := s.app.Status.GetStatus(ctx, req)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	fmt.Print(formatter.FormatStatus(resp))
}

func (s *shellSession) execWhatNow(args []string) {
	minutes := 60
	if len(args) > 0 {
		m, err := strconv.Atoi(args[0])
		if err != nil || m <= 0 {
			fmt.Println(formatter.StyleRed.Render(
				fmt.Sprintf("Invalid minutes %q — expected a positive integer.", args[0])))
			return
		}
		minutes = m
	}

	ctx := context.Background()
	req := contract.NewWhatNowRequest(minutes)
	if s.activeProjectID != "" {
		req.ProjectScope = []string{s.activeProjectID}
	}

	resp, err := s.app.WhatNow.Recommend(ctx, req)
	if err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	fmt.Print(formatWhatNowResponse(ctx, s.app, resp))
}

// execCobra passes unrecognized input through to the full Cobra command tree,
// giving the shell access to all CLI commands (project add, node, work, session, etc.).
func (s *shellSession) execCobra(args []string) {
	root := NewRootCmd(s.app)
	root.SetArgs(prepareShellCobraArgs(args))
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
	}
}

// prepareShellCobraArgs adjusts command args for shell-mode compatibility.
func prepareShellCobraArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	if !strings.EqualFold(args[0], "ask") {
		return args
	}
	if hasAnyArg(args, "--yes", "-y", "--help", "-h") {
		return args
	}

	out := make([]string, 0, len(args)+1)
	out = append(out, args...)
	out = append(out, "--yes")
	return out
}

func hasAnyArg(args []string, wanted ...string) bool {
	for _, arg := range args {
		for _, w := range wanted {
			if arg == w {
				return true
			}
		}
	}
	return false
}

func (s *shellSession) execClear() {
	fmt.Print("\033[H\033[2J")
}

func (s *shellSession) execHelp() {
	fmt.Print(formatter.FormatShellHelp())
}

func (s *shellSession) execHelpChat(args []string) {
	s.ensureHelpContext()

	if len(args) > 0 {
		question := strings.Join(args, " ")
		answer := resolveHelpAnswer(s.app, question, s.helpSpecJSON, s.helpCmdInfos)
		fmt.Print(formatter.FormatHelpAnswer(answer))
		return
	}

	s.helpChatMode = true
	s.helpConv = nil
	fmt.Print(formatter.FormatHelpChatWelcome())
}

func (s *shellSession) ensureHelpContext() {
	if s.helpSpecJSON != "" && len(s.helpCmdInfos) > 0 {
		return
	}

	root := NewRootCmd(s.app)
	spec := s.app.getCommandSpec(root)
	s.helpSpecJSON = SerializeCommandSpec(spec)
	s.helpCmdInfos = buildHelpCommandInfos(spec)
}

func (s *shellSession) execHelpChatTurn(input string) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "/quit", "/exit", "/q", "quit", "exit":
		s.helpChatMode = false
		s.helpConv = nil
		return
	case "/commands":
		fmt.Print(formatter.FormatCommandList(s.helpCmdInfos))
		return
	}

	if strings.TrimSpace(input) == "" {
		return
	}

	if s.app.Help != nil {
		var answer *intelligence.HelpAnswer
		var err error

		if s.helpConv == nil {
			s.helpConv, answer, err = s.app.Help.StartChat(context.Background(), input, s.helpSpecJSON)
		} else {
			answer, err = s.app.Help.NextTurn(context.Background(), s.helpConv, input)
		}

		if err != nil {
			answer = intelligence.DeterministicHelp(input, s.helpCmdInfos)
		}
		fmt.Print(formatter.FormatHelpAnswer(answer))
		return
	}

	answer := intelligence.DeterministicHelp(input, s.helpCmdInfos)
	fmt.Print(formatter.FormatHelpAnswer(answer))
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
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
	if s.activeProjectID == "" {
		return "kairos ❯ ", true
	}
	return fmt.Sprintf("kairos (%s) ❯ ", s.activeShortID), true
}

func (s *shellSession) executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	parts := strings.Fields(input)
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
		s.execHelp()
	case "exit", "quit":
		fmt.Println(formatter.Dim("Goodbye."))
		os.Exit(0)
	case "shell":
		fmt.Println(formatter.StyleYellow.Render("Already in shell mode."))
	default:
		s.execCobra(parts)
	}
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
		formatter.TruncID(project.ID),
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
	fmt.Print(formatter.FormatWhatNow(resp))
}

// execCobra passes unrecognized input through to the full Cobra command tree,
// giving the shell access to all CLI commands (project add, node, work, session, etc.).
func (s *shellSession) execCobra(args []string) {
	root := NewRootCmd(s.app)
	root.SetArgs(args)
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		fmt.Println(formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err)))
	}
}

func (s *shellSession) execClear() {
	fmt.Print("\033[H\033[2J")
}

func (s *shellSession) execHelp() {
	fmt.Print(formatter.FormatShellHelp())
}

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
)

// captureCobraOutput runs a command through the Cobra tree and captures output.
// It redirects os.Stdout so that direct fmt.Print calls from Cobra handlers
// are captured instead of writing raw bytes into the Bubbletea alternate screen.
func captureCobraOutput(app *App, args []string, activeProjectID, activeShortID string) string {
	origStdout := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		return shellError(err)
	}
	os.Stdout = pw

	root := NewRootCmd(app)
	root.SetOut(pw)
	root.SetErr(pw)
	root.SetArgs(prepareShellCobraArgs(args, activeProjectID))
	root.SilenceUsage = true
	root.SilenceErrors = true

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, pr)
		close(done)
	}()

	if execErr := root.Execute(); execErr != nil {
		errMsg := execErr.Error()
		fmt.Fprint(pw, shellError(execErr))
		if hint := hintForMissingFlag(errMsg, activeProjectID, activeShortID); hint != "" {
			fmt.Fprint(pw, "\n"+hint)
		}
		if strings.Contains(errMsg, "unknown command") && len(args) > 0 {
			fmt.Fprint(pw, "\n"+suggestAlternatives(app, args[0]))
		}
	}

	pw.Close()
	os.Stdout = origStdout
	<-done

	return buf.String()
}

// buildInspectTree builds the inspect output for a project, returning the formatted tree.
func buildInspectTree(app *App, ctx context.Context, projectID string) (string, error) {
	p, err := app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}

	rootNodes, err := app.Nodes.ListRoots(ctx, projectID)
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
			children, err := app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				fetchErr = fmt.Errorf("listing children of node %s: %w", n.ID, err)
				return
			}
			if len(children) > 0 {
				childMap[n.ID] = children
				fetchChildren(children)
			}
			items, err := app.WorkItems.ListByNode(ctx, n.ID)
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

// hintForMissingFlag returns a hint when a cobra command fails due to a missing flag
// that could be inferred from the active project context.
func hintForMissingFlag(errMsg, activeProjectID, activeShortID string) string {
	if !strings.Contains(errMsg, "required flag") {
		return ""
	}
	flagKeywords := []string{"project", "node", "work-item"}
	for _, kw := range flagKeywords {
		if strings.Contains(errMsg, kw) {
			if activeProjectID != "" {
				return formatter.Dim(fmt.Sprintf(
					"Hint: active project is %s — try adding --%s %s",
					activeShortID, kw, activeShortID,
				))
			}
			return formatter.Dim("Hint: set an active project with 'use <id>'")
		}
	}
	return ""
}

// suggestAlternatives returns fuzzy-matched command suggestions for an unrecognized input.
func suggestAlternatives(app *App, input string) string {
	root := NewRootCmd(app)
	spec := app.getCommandSpec(root)
	matches := spec.FuzzyMatch(input, 3)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(formatter.Dim("Did you mean:"))
	for _, match := range matches {
		path := strings.TrimPrefix(match.FullPath, "kairos ")
		b.WriteString(fmt.Sprintf("\n  %s  %s",
			formatter.StyleGreen.Render(path),
			formatter.Dim(match.Short),
		))
	}
	return b.String()
}

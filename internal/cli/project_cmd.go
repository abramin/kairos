package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func resolveProjectID(ctx context.Context, app *App, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("project ID is required")
	}

	projects, err := app.Projects.List(ctx, true)
	if err != nil {
		return "", err
	}

	// 1. Exact short ID match (case-insensitive)
	for _, p := range projects {
		if strings.EqualFold(p.ShortID, input) {
			return p.ID, nil
		}
	}

	// 2. Exact UUID match
	for _, p := range projects {
		if p.ID == input {
			return p.ID, nil
		}
	}

	// 3. UUID prefix match
	var matches []string
	for _, p := range projects {
		if strings.HasPrefix(p.ID, input) {
			matches = append(matches, p.ID)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("project not found: %q", input)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("project ID prefix %q is ambiguous (%d matches)", input, len(matches))
	}
}

func newProjectCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}

	cmd.AddCommand(
		newProjectAddCmd(app),
		newProjectListCmd(app),
		newProjectInspectCmd(app),
		newProjectUpdateCmd(app),
		newProjectArchiveCmd(app),
		newProjectUnarchiveCmd(app),
		newProjectRemoveCmd(app),
		newProjectInitCmd(app),
		newProjectImportCmd(app),
		newProjectDraftCmd(app),
	)

	return cmd
}

func newProjectAddCmd(app *App) *cobra.Command {
	var name, domainStr, start, due, shortID string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			startDate, err := time.Parse("2006-01-02", start)
			if err != nil {
				return fmt.Errorf("invalid start date %q: %w", start, err)
			}

			p := &domain.Project{
				ID:        uuid.New().String(),
				ShortID:   strings.ToUpper(shortID),
				Name:      name,
				Domain:    domainStr,
				StartDate: startDate,
				Status:    domain.ProjectActive,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if due != "" {
				dueDate, err := time.Parse("2006-01-02", due)
				if err != nil {
					return fmt.Errorf("invalid due date %q: %w", due, err)
				}
				p.TargetDate = &dueDate
			}

			if err := app.Projects.Create(context.Background(), p); err != nil {
				return err
			}

			fmt.Printf("Created project %s [%s]\n", p.Name, p.ShortID)
			return nil
		},
	}

	cmd.Flags().StringVar(&shortID, "id", "", "Short ID (3-6 uppercase letters + 2-4 digits, e.g. PHI01)")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&domainStr, "domain", "", "Project domain")
	cmd.Flags().StringVar(&start, "start", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&due, "due", "", "Target due date (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("domain")
	_ = cmd.MarkFlagRequired("start")

	return cmd
}

func newProjectListCmd(app *App) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := app.Projects.List(context.Background(), all)
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println("No projects found.")
				return nil
			}

			fmt.Printf("%s\n", formatter.FormatProjectList(projects))
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Include archived projects")

	return cmd
}

func newProjectInspectCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect ID",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, err := resolveProjectID(ctx, app, args[0])
			if err != nil {
				return err
			}
			p, err := app.Projects.GetByID(ctx, projectID)
			if err != nil {
				return err
			}

			// Fetch tree data for the inspect view.
			rootNodes, _ := app.Nodes.ListRoots(ctx, projectID)
			childMap := make(map[string][]*domain.PlanNode)
			workItems := make(map[string][]*domain.WorkItem)

			// Build tree: fetch children and work items for each node.
			var fetchChildren func(nodes []*domain.PlanNode)
			fetchChildren = func(nodes []*domain.PlanNode) {
				for _, n := range nodes {
					children, _ := app.Nodes.ListChildren(ctx, n.ID)
					if len(children) > 0 {
						childMap[n.ID] = children
						fetchChildren(children)
					}
					items, _ := app.WorkItems.ListByNode(ctx, n.ID)
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

			fmt.Printf("%s\n", formatter.FormatProjectInspect(data))
			return nil
		},
	}
}

func newProjectUpdateCmd(app *App) *cobra.Command {
	var name, domainStr, due, status, shortID string

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, err := resolveProjectID(ctx, app, args[0])
			if err != nil {
				return err
			}
			p, err := app.Projects.GetByID(ctx, projectID)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("id") {
				p.ShortID = strings.ToUpper(shortID)
			}
			if cmd.Flags().Changed("name") {
				p.Name = name
			}
			if cmd.Flags().Changed("domain") {
				p.Domain = domainStr
			}
			if cmd.Flags().Changed("due") {
				dueDate, err := time.Parse("2006-01-02", due)
				if err != nil {
					return fmt.Errorf("invalid due date %q: %w", due, err)
				}
				p.TargetDate = &dueDate
			}
			if cmd.Flags().Changed("status") {
				p.Status = domain.ProjectStatus(status)
			}
			p.UpdatedAt = time.Now()

			if err := app.Projects.Update(ctx, p); err != nil {
				return err
			}

			fmt.Printf("Updated project %s [%s]\n", p.Name, p.ShortID)
			return nil
		},
	}

	cmd.Flags().StringVar(&shortID, "id", "", "Short ID (3-6 uppercase letters + 2-4 digits)")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&domainStr, "domain", "", "Project domain")
	cmd.Flags().StringVar(&due, "due", "", "Target due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&status, "status", "", "Project status (active|paused|done)")

	return cmd
}

func newProjectArchiveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "archive ID",
		Short: "Archive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, err := resolveProjectID(ctx, app, args[0])
			if err != nil {
				return err
			}
			if err := app.Projects.Archive(ctx, projectID); err != nil {
				return err
			}
			fmt.Printf("Archived project %s\n", projectID)
			return nil
		},
	}
}

func newProjectUnarchiveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive ID",
		Short: "Unarchive a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, err := resolveProjectID(ctx, app, args[0])
			if err != nil {
				return err
			}
			if err := app.Projects.Unarchive(ctx, projectID); err != nil {
				return err
			}
			fmt.Printf("Unarchived project %s\n", projectID)
			return nil
		},
	}
}

func newProjectRemoveCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, err := resolveProjectID(ctx, app, args[0])
			if err != nil {
				return err
			}
			if err := app.Projects.Delete(ctx, projectID, force); err != nil {
				return err
			}
			fmt.Printf("Removed project %s\n", projectID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force removal even if project has children")

	return cmd
}

func newProjectInitCmd(app *App) *cobra.Command {
	var templateName, name, shortID, start, due string
	var vars []string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project from a template",
		RunE: func(cmd *cobra.Command, args []string) error {
			varMap := make(map[string]string)
			for _, v := range vars {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid --var format %q, expected key=value", v)
				}
				varMap[parts[0]] = parts[1]
			}

			var duePtr *string
			if due != "" {
				duePtr = &due
			}

			p, err := app.Templates.InitProject(context.Background(), templateName, name, strings.ToUpper(shortID), start, duePtr, varMap)
			if err != nil {
				return err
			}

			fmt.Printf("Initialized project %s [%s] from template %q\n", p.Name, p.ShortID, templateName)
			return nil
		},
	}

	cmd.Flags().StringVar(&shortID, "id", "", "Short ID (3-6 uppercase letters + 2-4 digits, e.g. PHI01)")
	cmd.Flags().StringVar(&templateName, "template", "", "Template reference (integer ID from `template list`, name, schema ID, or file stem)")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&start, "start", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&due, "due", "", "Target due date (YYYY-MM-DD)")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "Template variables (key=value)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("template")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("start")

	return cmd
}

func newProjectImportCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "import FILE",
		Short: "Import a project from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Import.ImportProject(context.Background(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Imported project %s [%s] — %d nodes, %d work items, %d dependencies\n",
				result.Project.Name, result.Project.ShortID,
				result.NodeCount, result.WorkItemCount, result.DependencyCount)
			return nil
		},
	}
}

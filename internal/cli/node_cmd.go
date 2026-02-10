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

func newNodeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage plan nodes",
	}

	cmd.AddCommand(
		newNodeAddCmd(app),
		newNodeInspectCmd(app),
		newNodeUpdateCmd(app),
		newNodeRemoveCmd(app),
	)

	return cmd
}

func newNodeAddCmd(app *App) *cobra.Command {
	var projectID, title, kind, parentID string
	var order int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new plan node",
		RunE: func(cmd *cobra.Command, args []string) error {
			n := &domain.PlanNode{
				ID:         uuid.New().String(),
				ProjectID:  projectID,
				Title:      title,
				Kind:       domain.NodeKind(kind),
				OrderIndex: order,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			if cmd.Flags().Changed("parent") {
				n.ParentID = &parentID
			}

			if err := app.Nodes.Create(context.Background(), n); err != nil {
				return err
			}

			fmt.Printf("Created node %s (%s)\n", n.Title, n.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&title, "title", "", "Node title")
	cmd.Flags().StringVar(&kind, "kind", "", "Node kind (week|module|book|stage|section|assessment|generic)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent node ID")
	cmd.Flags().IntVar(&order, "order", 0, "Order index")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("kind")

	return cmd
}

func newNodeInspectCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "inspect ID",
		Short: "Show node details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			nodeID, err := resolveNodeID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			n, err := app.Nodes.GetByID(ctx, nodeID)
			if err != nil {
				return err
			}
			projectDisplayID := formatter.TruncID(n.ProjectID)
			project, err := app.Projects.GetByID(ctx, n.ProjectID)
			if err == nil && strings.TrimSpace(project.ShortID) != "" {
				projectDisplayID = formatter.Dim(project.ShortID)
			}

			var b strings.Builder

			b.WriteString(fmt.Sprintf("%s  %s\n\n", formatter.Bold(n.Title), formatter.Dim(string(n.Kind))))
			if n.Seq > 0 {
				b.WriteString(fmt.Sprintf("  %s  #%d\n", formatter.Dim("ID     "), n.Seq))
			} else {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("ID     "), formatter.TruncID(n.ID)))
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("PROJECT"), projectDisplayID))
			b.WriteString(fmt.Sprintf("  %s  %d\n", formatter.Dim("ORDER  "), n.OrderIndex))
			if n.ParentID != nil {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("PARENT "), formatter.TruncID(*n.ParentID)))
			}
			if n.DueDate != nil {
				b.WriteString(fmt.Sprintf("  %s  %s %s\n", formatter.Dim("DUE    "),
					formatter.RelativeDateStyled(*n.DueDate),
					formatter.Dim("("+n.DueDate.Format("Jan 2, 2006")+")")))
			}
			if n.NotBefore != nil {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("AFTER  "), formatter.HumanDate(*n.NotBefore)))
			}
			if n.NotAfter != nil {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("BEFORE "), formatter.HumanDate(*n.NotAfter)))
			}
			if n.PlannedMinBudget != nil {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("BUDGET "), formatter.FormatMinutes(*n.PlannedMinBudget)))
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("UPDATED"), formatter.HumanTimestamp(n.UpdatedAt)))

			// List children.
			children, err := app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				return err
			}
			if len(children) > 0 {
				b.WriteString("\n")
				b.WriteString(formatter.Header("Children"))
				b.WriteString("\n")
				headers := []string{"ID", "TITLE", "KIND", "ORDER"}
				rows := make([][]string, 0, len(children))
				for _, c := range children {
					rows = append(rows, []string{
						formatter.TruncID(c.ID),
						c.Title,
						string(c.Kind),
						fmt.Sprintf("%d", c.OrderIndex),
					})
				}
				b.WriteString(formatter.RenderTable(headers, rows))
			}

			fmt.Print(formatter.RenderBox("Plan Node", b.String()))
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

func newNodeUpdateCmd(app *App) *cobra.Command {
	var title, kind, projectFlag string
	var order int

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a plan node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			nodeID, err := resolveNodeID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			n, err := app.Nodes.GetByID(ctx, nodeID)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("title") {
				n.Title = title
			}
			if cmd.Flags().Changed("kind") {
				n.Kind = domain.NodeKind(kind)
			}
			if cmd.Flags().Changed("order") {
				n.OrderIndex = order
			}
			n.UpdatedAt = time.Now()

			if err := app.Nodes.Update(ctx, n); err != nil {
				return err
			}

			fmt.Printf("Updated node %s\n", n.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Node title")
	cmd.Flags().StringVar(&kind, "kind", "", "Node kind (week|module|book|stage|section|assessment|generic)")
	cmd.Flags().IntVar(&order, "order", 0, "Order index")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")

	return cmd
}

func newNodeRemoveCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a plan node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			nodeID, err := resolveNodeID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			if err := app.Nodes.Delete(ctx, nodeID); err != nil {
				return err
			}
			fmt.Printf("Removed node %s\n", nodeID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

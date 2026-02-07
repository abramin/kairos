package cli

import (
	"context"
	"fmt"
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
	cmd.Flags().StringVar(&kind, "kind", "", "Node kind (week|module|book|stage|section|generic)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent node ID")
	cmd.Flags().IntVar(&order, "order", 0, "Order index")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("kind")

	return cmd
}

func newNodeInspectCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect ID",
		Short: "Show node details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			n, err := app.Nodes.GetByID(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Println(formatter.Header("Plan Node"))
			fmt.Printf("  ID:        %s\n", n.ID)
			fmt.Printf("  Project:   %s\n", n.ProjectID)
			fmt.Printf("  Title:     %s\n", formatter.Bold(n.Title))
			fmt.Printf("  Kind:      %s\n", string(n.Kind))
			fmt.Printf("  Order:     %d\n", n.OrderIndex)
			if n.ParentID != nil {
				fmt.Printf("  Parent:    %s\n", *n.ParentID)
			}
			if n.DueDate != nil {
				fmt.Printf("  Due:       %s\n", n.DueDate.Format("2006-01-02"))
			}
			if n.NotBefore != nil {
				fmt.Printf("  Not before: %s\n", n.NotBefore.Format("2006-01-02"))
			}
			if n.NotAfter != nil {
				fmt.Printf("  Not after:  %s\n", n.NotAfter.Format("2006-01-02"))
			}
			if n.PlannedMinBudget != nil {
				fmt.Printf("  Budget:    %d min\n", *n.PlannedMinBudget)
			}
			fmt.Printf("  Created:   %s\n", n.CreatedAt.Format(time.RFC3339))
			fmt.Printf("  Updated:   %s\n", n.UpdatedAt.Format(time.RFC3339))

			// List children.
			children, err := app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				return err
			}
			if len(children) > 0 {
				fmt.Println()
				fmt.Println(formatter.Header("Children"))
				headers := []string{"ID", "Title", "Kind", "Order"}
				rows := make([][]string, 0, len(children))
				for _, c := range children {
					rows = append(rows, []string{
						c.ID[:8],
						c.Title,
						string(c.Kind),
						fmt.Sprintf("%d", c.OrderIndex),
					})
				}
				fmt.Print(formatter.RenderTable(headers, rows))
			}

			return nil
		},
	}
}

func newNodeUpdateCmd(app *App) *cobra.Command {
	var title, kind string
	var order int

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a plan node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			n, err := app.Nodes.GetByID(ctx, args[0])
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
	cmd.Flags().StringVar(&kind, "kind", "", "Node kind (week|module|book|stage|section|generic)")
	cmd.Flags().IntVar(&order, "order", 0, "Order index")

	return cmd
}

func newNodeRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a plan node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Nodes.Delete(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed node %s\n", args[0])
			return nil
		},
	}
}

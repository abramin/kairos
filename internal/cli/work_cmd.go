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

func newWorkCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage work items",
	}

	cmd.AddCommand(
		newWorkAddCmd(app),
		newWorkInspectCmd(app),
		newWorkUpdateCmd(app),
		newWorkDoneCmd(app),
		newWorkArchiveCmd(app),
		newWorkRemoveCmd(app),
	)

	return cmd
}

func newWorkAddCmd(app *App) *cobra.Command {
	var (
		nodeID, title, typ, unitsKind      string
		plannedMin, minSession, maxSession int
		defaultSession, unitsTotal         int
		splittable                         bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new work item",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := &domain.WorkItem{
				ID:                uuid.New().String(),
				NodeID:            nodeID,
				Title:             title,
				Type:              typ,
				Status:            domain.WorkItemTodo,
				PlannedMin:        plannedMin,
				MinSessionMin:     minSession,
				MaxSessionMin:     maxSession,
				DefaultSessionMin: defaultSession,
				Splittable:        splittable,
				UnitsKind:         unitsKind,
				UnitsTotal:        unitsTotal,
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}

			if err := app.WorkItems.Create(context.Background(), w); err != nil {
				return err
			}

			fmt.Printf("Created work item %s (%s)\n", w.Title, w.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&nodeID, "node", "", "Parent node ID")
	cmd.Flags().StringVar(&title, "title", "", "Work item title")
	cmd.Flags().StringVar(&typ, "type", "", "Work item type")
	cmd.Flags().IntVar(&plannedMin, "planned-min", 0, "Planned duration in minutes")
	cmd.Flags().IntVar(&minSession, "min-session", 0, "Minimum session length in minutes")
	cmd.Flags().IntVar(&maxSession, "max-session", 0, "Maximum session length in minutes")
	cmd.Flags().IntVar(&defaultSession, "default-session", 0, "Default session length in minutes")
	cmd.Flags().BoolVar(&splittable, "splittable", false, "Whether the item can be split across sessions")
	cmd.Flags().StringVar(&unitsKind, "units-kind", "", "Kind of units (e.g. pages, problems)")
	cmd.Flags().IntVar(&unitsTotal, "units-total", 0, "Total number of units")
	_ = cmd.MarkFlagRequired("node")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func newWorkInspectCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "inspect ID",
		Short: "Show work item details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			w, err := app.WorkItems.GetByID(ctx, wiID)
			if err != nil {
				return err
			}

			var b strings.Builder

			b.WriteString(fmt.Sprintf("%s  %s\n\n", formatter.Bold(w.Title), formatter.Dim(w.Type)))

			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("STATUS "), formatter.WorkItemStatusPill(w.Status)))
			if w.Seq > 0 {
				b.WriteString(fmt.Sprintf("  %s  #%d\n", formatter.Dim("ID     "), w.Seq))
			} else {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("ID     "), formatter.TruncID(w.ID)))
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("NODE   "), formatter.TruncID(w.NodeID)))
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("MODE   "), formatter.Dim(string(w.DurationMode))))
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("PLANNED"), formatter.FormatMinutes(w.PlannedMin)))
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("LOGGED "), formatter.FormatMinutes(w.LoggedMin)))

			if w.PlannedMin > 0 {
				pct := float64(w.LoggedMin) / float64(w.PlannedMin)
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("PROGRESS"), formatter.RenderProgress(pct, 20)))
			}

			sessionPolicy := fmt.Sprintf("%s-%s (default %s)",
				formatter.FormatMinutes(w.MinSessionMin),
				formatter.FormatMinutes(w.MaxSessionMin),
				formatter.FormatMinutes(w.DefaultSessionMin))
			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("SESSION"), sessionPolicy))

			if w.Splittable {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("SPLIT  "), formatter.StyleGreen.Render("Yes")))
			}

			if w.UnitsKind != "" {
				b.WriteString(fmt.Sprintf("  %s  %d/%d %s\n", formatter.Dim("UNITS  "), w.UnitsDone, w.UnitsTotal, w.UnitsKind))
			}

			if w.DueDate != nil {
				b.WriteString(fmt.Sprintf("  %s  %s %s\n", formatter.Dim("DUE    "),
					formatter.RelativeDateStyled(*w.DueDate),
					formatter.Dim("("+w.DueDate.Format("Jan 2, 2006")+")")))
			}
			if w.NotBefore != nil {
				b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("AFTER  "), formatter.HumanDate(*w.NotBefore)))
			}

			b.WriteString(fmt.Sprintf("  %s  %s\n", formatter.Dim("UPDATED"), formatter.HumanTimestamp(w.UpdatedAt)))

			fmt.Print(formatter.RenderBox("Work Item", b.String()))
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

func newWorkUpdateCmd(app *App) *cobra.Command {
	var (
		title, typ, status, unitsKind, projectFlag string
		plannedMin, minSession, maxSession          int
		defaultSession, unitsTotal, unitsDone       int
		splittable                                  bool
	)

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			w, err := app.WorkItems.GetByID(ctx, wiID)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("title") {
				w.Title = title
			}
			if cmd.Flags().Changed("type") {
				w.Type = typ
			}
			if cmd.Flags().Changed("status") {
				w.Status = domain.WorkItemStatus(status)
			}
			if cmd.Flags().Changed("planned-min") {
				w.PlannedMin = plannedMin
			}
			if cmd.Flags().Changed("min-session") {
				w.MinSessionMin = minSession
			}
			if cmd.Flags().Changed("max-session") {
				w.MaxSessionMin = maxSession
			}
			if cmd.Flags().Changed("default-session") {
				w.DefaultSessionMin = defaultSession
			}
			if cmd.Flags().Changed("splittable") {
				w.Splittable = splittable
			}
			if cmd.Flags().Changed("units-kind") {
				w.UnitsKind = unitsKind
			}
			if cmd.Flags().Changed("units-total") {
				w.UnitsTotal = unitsTotal
			}
			if cmd.Flags().Changed("units-done") {
				w.UnitsDone = unitsDone
			}
			w.UpdatedAt = time.Now()

			if err := app.WorkItems.Update(ctx, w); err != nil {
				return err
			}

			fmt.Printf("Updated work item %s\n", w.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Work item title")
	cmd.Flags().StringVar(&typ, "type", "", "Work item type")
	cmd.Flags().StringVar(&status, "status", "", "Status (todo|in_progress|done|skipped)")
	cmd.Flags().IntVar(&plannedMin, "planned-min", 0, "Planned duration in minutes")
	cmd.Flags().IntVar(&minSession, "min-session", 0, "Minimum session length in minutes")
	cmd.Flags().IntVar(&maxSession, "max-session", 0, "Maximum session length in minutes")
	cmd.Flags().IntVar(&defaultSession, "default-session", 0, "Default session length in minutes")
	cmd.Flags().BoolVar(&splittable, "splittable", false, "Whether the item can be split across sessions")
	cmd.Flags().StringVar(&unitsKind, "units-kind", "", "Kind of units")
	cmd.Flags().IntVar(&unitsTotal, "units-total", 0, "Total number of units")
	cmd.Flags().IntVar(&unitsDone, "units-done", 0, "Number of completed units")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")

	return cmd
}

func newWorkDoneCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "done ID",
		Short: "Mark a work item as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			if err := app.WorkItems.MarkDone(ctx, wiID); err != nil {
				return err
			}
			fmt.Printf("Marked work item %s as done\n", wiID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

func newWorkArchiveCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "archive ID",
		Short: "Archive a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			if err := app.WorkItems.Archive(ctx, wiID); err != nil {
				return err
			}
			fmt.Printf("Archived work item %s\n", wiID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

func newWorkRemoveCmd(app *App) *cobra.Command {
	var projectFlag string
	cmd := &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, args[0], projectID)
			if err != nil {
				return err
			}
			if err := app.WorkItems.Delete(ctx, wiID); err != nil {
				return err
			}
			fmt.Printf("Removed work item %s\n", wiID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	return cmd
}

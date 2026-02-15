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

func newSessionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage work sessions",
	}

	cmd.AddCommand(
		newSessionLogCmd(app),
		newSessionListCmd(app),
		newSessionRemoveCmd(app),
	)

	return cmd
}

func newSessionLogCmd(app *App) *cobra.Command {
	var workItemID, note, projectFlag string
	var minutes, unitsDone int

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Log a work session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
			wiID, err := resolveWorkItemID(ctx, app, workItemID, projectID)
			if err != nil {
				return err
			}

			s := &domain.WorkSessionLog{
				ID:             uuid.New().String(),
				WorkItemID:     wiID,
				StartedAt:      time.Now(),
				Minutes:        minutes,
				UnitsDoneDelta: unitsDone,
				Note:           note,
				CreatedAt:      time.Now(),
			}

			logSession := app.logSessionUseCase()
			if logSession == nil {
				return fmt.Errorf("log-session use case is not configured")
			}
			if err := logSession.LogSession(ctx, s); err != nil {
				return err
			}

			fmt.Printf("Logged %d min session for work item %s (%s)\n", minutes, wiID, s.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&workItemID, "work-item", "", "Work item ID")
	cmd.Flags().IntVar(&minutes, "minutes", 0, "Session duration in minutes")
	cmd.Flags().IntVar(&unitsDone, "units-done", 0, "Number of units completed in this session")
	cmd.Flags().StringVar(&note, "note", "", "Session note")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")
	_ = cmd.MarkFlagRequired("work-item")
	_ = cmd.MarkFlagRequired("minutes")

	return cmd
}

func newSessionListCmd(app *App) *cobra.Command {
	var workItemID, projectFlag string
	var days int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List work sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			var sessions []*domain.WorkSessionLog
			var err error

			if workItemID != "" {
				projectID, _ := resolveProjectForFlag(ctx, app, projectFlag)
				wiID, resolveErr := resolveWorkItemID(ctx, app, workItemID, projectID)
				if resolveErr != nil {
					return resolveErr
				}
				sessions, err = app.Sessions.ListByWorkItem(ctx, wiID)
			} else {
				sessions, err = app.Sessions.ListRecent(ctx, days)
			}
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			headers := []string{"ID", "WORK ITEM", "STARTED", "DURATION", "UNITS", "NOTE"}
			rows := make([][]string, 0, len(sessions))
			for _, s := range sessions {
				notePreview := s.Note
				if len(notePreview) > 40 {
					notePreview = notePreview[:37] + "..."
				}
				rows = append(rows, []string{
					formatter.TruncID(s.ID),
					formatter.TruncID(s.WorkItemID),
					formatter.HumanTimestamp(s.StartedAt),
					formatter.FormatMinutes(s.Minutes),
					fmt.Sprintf("%d", s.UnitsDoneDelta),
					formatter.Dim(notePreview),
				})
			}

			fmt.Print(formatter.RenderBox("Sessions", formatter.RenderTable(headers, rows)))
			return nil
		},
	}

	cmd.Flags().StringVar(&workItemID, "work-item", "", "Filter by work item ID")
	cmd.Flags().IntVar(&days, "days", 7, "Number of recent days to show")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project context for numeric IDs")

	return cmd
}

func newSessionRemoveCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a session log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Sessions.Delete(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed session %s\n", args[0])
			return nil
		},
	}
}

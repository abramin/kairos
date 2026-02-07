package cli

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/spf13/cobra"
)

func newStatusCmd(app *App) *cobra.Command {
	var recalc bool
	var projectID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show project status overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := contract.NewStatusRequest()

			if cmd.Flags().Changed("recalc") {
				req.Recalc = recalc
			}
			if projectID != "" {
				req.ProjectScope = []string{projectID}
			}

			resp, err := app.Status.GetStatus(context.Background(), req)
			if err != nil {
				return err
			}

			fmt.Print(formatter.FormatStatus(resp))
			return nil
		},
	}

	cmd.Flags().BoolVar(&recalc, "recalc", false, "Force recalculation of risk and progress")
	cmd.Flags().StringVar(&projectID, "project", "", "Scope to a single project ID")

	return cmd
}

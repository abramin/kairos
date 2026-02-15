package cli

import (
	"context"
	"fmt"

	kairosapp "github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/spf13/cobra"
)

func newReplanCmd(app *App) *cobra.Command {
	var strategy string

	cmd := &cobra.Command{
		Use:   "replan",
		Short: "Rebalance project schedules",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := kairosapp.NewReplanRequest(domain.TriggerManual)

			if cmd.Flags().Changed("strategy") {
				req.Strategy = strategy
			}

			resp, err := app.Replan.Replan(context.Background(), req)
			if err != nil {
				return err
			}

			fmt.Println(formatter.Header("Replan Results"))
			fmt.Printf("  Trigger:    %s\n", string(resp.Trigger))
			fmt.Printf("  Strategy:   %s\n", resp.Strategy)
			fmt.Printf("  Projects:   %d recomputed\n", resp.RecomputedProjects)
			fmt.Printf("  Mode after: %s\n", string(resp.GlobalModeAfter))
			fmt.Println()

			if len(resp.Deltas) > 0 {
				headers := []string{"Project", "Risk Before", "Risk After", "Daily Min Before", "Daily Min After", "Changes"}
				rows := make([][]string, 0, len(resp.Deltas))
				for _, d := range resp.Deltas {
					rows = append(rows, []string{
						d.ProjectName,
						formatter.RiskIndicator(d.RiskBefore),
						formatter.RiskIndicator(d.RiskAfter),
						fmt.Sprintf("%.0f", d.RequiredDailyMinBefore),
						fmt.Sprintf("%.0f", d.RequiredDailyMinAfter),
						fmt.Sprintf("%d items", d.ChangedItemsCount),
					})
				}
				fmt.Print(formatter.RenderTable(headers, rows))
			} else {
				fmt.Println(formatter.Dim("  No changes needed."))
			}

			// Explanation.
			if resp.Explanation != nil {
				fmt.Println()
				if len(resp.Explanation.CriticalProjects) > 0 {
					fmt.Printf("  Critical projects: %v\n", resp.Explanation.CriticalProjects)
				}
				for _, rule := range resp.Explanation.RulesApplied {
					fmt.Printf("  Rule: %s\n", formatter.Dim(rule))
				}
			}

			// Warnings.
			for _, w := range resp.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "  WARNING: %s\n", w)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&strategy, "strategy", "rebalance", "Replan strategy (rebalance|deadline_first)")

	return cmd
}

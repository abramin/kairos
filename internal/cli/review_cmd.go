package cli

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/spf13/cobra"
)

func newReviewCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review activity and get insights",
	}

	cmd.AddCommand(newReviewWeeklyCmd(app))
	return cmd
}

func newReviewWeeklyCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "weekly",
		Short: "Summarize the past 7 days with actionable insights",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeeklyReview(app)
		},
	}
}

func runWeeklyReview(app *App) error {
	ctx := context.Background()

	// Get status for project-level data.
	statusReq := contract.NewStatusRequest()
	statusResp, err := app.Status.GetStatus(ctx, statusReq)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	// Build weekly review trace from status.
	trace := intelligence.WeeklyReviewTrace{
		PeriodDays: 7,
	}

	totalLogged := 0
	sessionCount := 0
	for _, p := range statusResp.Projects {
		trace.ProjectSummaries = append(trace.ProjectSummaries, intelligence.ProjectWeeklySummary{
			ProjectID:   p.ProjectID,
			ProjectName: p.ProjectName,
			PlannedMin:  p.PlannedMinTotal,
			LoggedMin:   p.LoggedMinTotal,
			RiskLevel:   string(p.RiskLevel),
		})
		totalLogged += p.LoggedMinTotal
	}
	trace.TotalLoggedMin = totalLogged
	trace.SessionCount = sessionCount

	var explanation *intelligence.LLMExplanation
	if app.Explain != nil {
		explanation, err = app.Explain.WeeklyReview(ctx, trace)
		if err != nil {
			explanation = intelligence.DeterministicWeeklyReview(trace)
		}
	} else {
		explanation = intelligence.DeterministicWeeklyReview(trace)
	}

	fmt.Print(formatter.FormatStatus(statusResp))
	fmt.Println()
	fmt.Print(formatter.FormatExplanation(explanation))
	return nil
}

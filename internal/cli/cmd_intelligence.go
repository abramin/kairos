package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
	tea "github.com/charmbracelet/bubbletea"
)

// ── ask command ──────────────────────────────────────────────────────────────

func (c *commandBar) cmdAsk(args []string) tea.Cmd {
	if len(args) == 0 {
		return outputCmd(formatter.StyleYellow.Render("Usage: ask \"<natural language question>\""))
	}

	if c.state.App.Intent == nil {
		return outputCmd(formatter.StyleRed.Render(
			"LLM features are disabled. Use explicit commands:\n" +
				"  what-now 60\n  status\n  project list\n\n" +
				"Enable with: KAIROS_LLM_ENABLED=true"))
	}

	question := strings.Join(args, " ")

	return tea.Batch(
		loadingCmd("Thinking..."),
		asyncOutputCmd(func() string {
			ctx := context.Background()

			resolution, err := c.state.App.Intent.Parse(ctx, question)
			if err != nil {
				return shellError(fmt.Errorf("parse failed: %w", err))
			}

			resolution.CommandHint = CommandHint(resolution.ParsedIntent)
			output := formatter.FormatAskResolution(resolution)

			// Auto-execute read-only intents.
			if resolution.ExecutionState == intelligence.StateExecuted {
				result := c.dispatchIntentTUI(resolution.ParsedIntent)
				if result != "" {
					output += "\n" + result
				}
			}

			return output
		}),
	)
}

// dispatchIntentTUI maps a parsed intent to a service call, returning
// formatted output instead of using fmt.Print.
func (c *commandBar) dispatchIntentTUI(intent *intelligence.ParsedIntent) string {
	ctx := context.Background()

	switch intent.Intent {
	case intelligence.IntentWhatNow:
		min := intArg(intent.Arguments, "available_min", 60)
		req := contract.NewWhatNowRequest(min)
		resp, err := c.state.App.WhatNow.Recommend(ctx, req)
		if err != nil {
			return shellError(err)
		}
		return formatWhatNowResponse(ctx, c.state.App, resp)

	case intelligence.IntentStatus:
		req := contract.NewStatusRequest()
		resp, err := c.state.App.Status.GetStatus(ctx, req)
		if err != nil {
			return shellError(err)
		}
		return formatter.FormatStatus(resp)

	case intelligence.IntentExplainNow:
		min := intArg(intent.Arguments, "minutes", 60)
		return c.runExplainNowTUI(min)

	case intelligence.IntentReviewWeekly:
		return c.runReviewWeeklyTUI()

	default:
		hint := CommandHint(intent)
		if hint != "" {
			return fmt.Sprintf("Run: %s", hint)
		}
		return fmt.Sprintf("Intent %q recognized but has no direct dispatch.", intent.Intent)
	}
}

// ── explain command ──────────────────────────────────────────────────────────

func (c *commandBar) cmdExplain(args []string) tea.Cmd {
	if len(args) == 0 {
		return outputCmd(formatter.StyleYellow.Render("Usage: explain now [minutes] | explain why-not <id>"))
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "now":
		minutes := 60
		if len(args) > 1 {
			if m, err := strconv.Atoi(args[1]); err == nil && m > 0 {
				minutes = m
			}
		}
		return tea.Batch(
			loadingCmd("Generating explanation..."),
			asyncOutputCmd(func() string { return c.runExplainNowTUI(minutes) }),
		)

	case "why-not":
		if len(args) < 2 {
			return outputCmd(formatter.StyleYellow.Render("Usage: explain why-not <project-id or work-item-id>"))
		}
		candidateID := args[1]
		return tea.Batch(
			loadingCmd("Generating explanation..."),
			asyncOutputCmd(func() string { return c.runExplainWhyNotTUI(candidateID) }),
		)

	default:
		// Fall through to cobra for flag-based usage.
		return outputCmd(c.cobraCapture(append([]string{"explain"}, args...)))
	}
}

func (c *commandBar) runExplainNowTUI(minutes int) string {
	ctx := context.Background()

	req := contract.NewWhatNowRequest(minutes)
	resp, err := c.state.App.WhatNow.Recommend(ctx, req)
	if err != nil {
		return shellError(err)
	}

	trace := intelligence.BuildRecommendationTrace(resp)

	explanation := c.explainWithFallback(
		func() (*intelligence.LLMExplanation, error) { return c.state.App.Explain.ExplainNow(ctx, trace) },
		func() *intelligence.LLMExplanation { return intelligence.DeterministicExplainNow(trace) },
	)

	return formatWhatNowResponse(ctx, c.state.App, resp) + "\n" + formatter.FormatExplanation(explanation)
}

func (c *commandBar) runExplainWhyNotTUI(candidateRef string) string {
	ctx := context.Background()

	// Try to resolve as project or work item ID.
	candidateID := candidateRef
	if resolved, err := resolveProjectID(ctx, c.state.App, candidateRef); err == nil {
		candidateID = resolved
	} else if resolved, err := resolveWorkItemID(ctx, c.state.App, candidateRef, c.state.ActiveProjectID); err == nil {
		candidateID = resolved
	}

	req := contract.NewWhatNowRequest(60)
	resp, err := c.state.App.WhatNow.Recommend(ctx, req)
	if err != nil {
		return shellError(err)
	}

	trace := intelligence.BuildRecommendationTrace(resp)

	explanation := c.explainWithFallback(
		func() (*intelligence.LLMExplanation, error) {
			return c.state.App.Explain.ExplainWhyNot(ctx, trace, candidateID)
		},
		func() *intelligence.LLMExplanation { return intelligence.DeterministicWhyNot(trace, candidateID) },
	)

	return formatter.FormatExplanation(explanation)
}

// ── review command ───────────────────────────────────────────────────────────

func (c *commandBar) cmdReview(args []string) tea.Cmd {
	if len(args) == 0 {
		return outputCmd(formatter.StyleYellow.Render("Usage: review weekly"))
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "weekly":
		return tea.Batch(
			loadingCmd("Generating weekly review..."),
			asyncOutputCmd(c.runReviewWeeklyTUI),
		)
	default:
		return outputCmd(c.cobraCapture(append([]string{"review"}, args...)))
	}
}

func (c *commandBar) runReviewWeeklyTUI() string {
	ctx := context.Background()

	statusReq := contract.NewStatusRequest()
	statusResp, err := c.state.App.Status.GetStatus(ctx, statusReq)
	if err != nil {
		return shellError(fmt.Errorf("getting status: %w", err))
	}

	trace := intelligence.WeeklyReviewTrace{
		PeriodDays: 7,
	}

	totalLogged := 0
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

	explanation := c.explainWithFallback(
		func() (*intelligence.LLMExplanation, error) { return c.state.App.Explain.WeeklyReview(ctx, trace) },
		func() *intelligence.LLMExplanation { return intelligence.DeterministicWeeklyReview(trace) },
	)

	return formatter.FormatStatus(statusResp) + "\n" + formatter.FormatExplanation(explanation)
}

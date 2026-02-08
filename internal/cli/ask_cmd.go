package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/spf13/cobra"
)

func newAskCmd(app *App) *cobra.Command {
	autoConfirm := false

	cmd := &cobra.Command{
		Use:   `ask "<natural language>"`,
		Short: "Parse natural language into a command",
		Long:  "Use Ollama to parse natural language into a structured Kairos command.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Intent == nil {
				return fmt.Errorf("LLM features are disabled. Use explicit commands:\n" +
					"  kairos what-now --minutes 60\n" +
					"  kairos status\n" +
					"  kairos project list\n\n" +
					"Enable with: KAIROS_LLM_ENABLED=true")
			}

			resolution, err := app.Intent.Parse(context.Background(), args[0])
			if err != nil {
				if errors.Is(err, llm.ErrTimeout) {
					return fmt.Errorf("parse failed: %w (set KAIROS_LLM_PARSE_TIMEOUT_MS, e.g. 15000)", err)
				}
				return fmt.Errorf("parse failed: %w", err)
			}

			fmt.Print(formatter.FormatAskResolution(resolution))

			switch resolution.ExecutionState {
			case intelligence.StateExecuted:
				return dispatchIntent(app, resolution.ParsedIntent)
			case intelligence.StateNeedsConfirmation:
				if autoConfirm || confirmPromptIO(cmd.InOrStdin(), cmd.OutOrStdout()) {
					return dispatchIntent(app, resolution.ParsedIntent)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
			case intelligence.StateNeedsClarification:
				// Display already handled by formatter.
			case intelligence.StateRejected:
				// Display already handled by formatter.
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "Automatically confirm write actions")
	return cmd
}

// dispatchIntent maps a parsed intent to a v1 service call.
func dispatchIntent(app *App, intent *intelligence.ParsedIntent) error {
	ctx := context.Background()

	switch intent.Intent {
	case intelligence.IntentWhatNow:
		min := intArg(intent.Arguments, "available_min", 60)
		req := contract.NewWhatNowRequest(min)
		resp, err := app.WhatNow.Recommend(ctx, req)
		if err != nil {
			return err
		}
		fmt.Print(formatWhatNowResponse(ctx, app, resp))

	case intelligence.IntentStatus:
		req := contract.NewStatusRequest()
		resp, err := app.Status.GetStatus(ctx, req)
		if err != nil {
			return err
		}
		fmt.Print(formatter.FormatStatus(resp))

	case intelligence.IntentProjectUpdate:
		return dispatchProjectUpdateIntent(app, intent.Arguments)

	case intelligence.IntentExplainNow:
		min := intArg(intent.Arguments, "minutes", 60)
		return runExplainNow(app, min)

	case intelligence.IntentReviewWeekly:
		return runWeeklyReview(app)

	default:
		fmt.Printf("Intent %q dispatched. Use the explicit CLI command to execute.\n", intent.Intent)
	}
	return nil
}

func intArg(args map[string]interface{}, key string, fallback int) int {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return fallback
	}
}

func boolArg(args map[string]interface{}, key string, fallback bool) bool {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

func stringArg(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func dispatchProjectUpdateIntent(app *App, args map[string]interface{}) error {
	ctx := context.Background()

	projectRef, ok := stringArg(args, "project_id")
	if !ok || strings.TrimSpace(projectRef) == "" {
		return fmt.Errorf("project_update requires project_id")
	}

	projectID, err := resolveProjectID(ctx, app, projectRef)
	if err != nil {
		return err
	}

	p, err := app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return err
	}

	updateNeeded := false

	if name, ok := stringArg(args, "name"); ok {
		p.Name = name
		updateNeeded = true
	}

	if raw, exists := args["target_date"]; exists {
		switch v := raw.(type) {
		case nil:
			p.TargetDate = nil
			updateNeeded = true
		case string:
			dateStr := strings.TrimSpace(v)
			if dateStr == "" {
				p.TargetDate = nil
				updateNeeded = true
				break
			}
			dueDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return fmt.Errorf("invalid target_date %q: %w", dateStr, err)
			}
			p.TargetDate = &dueDate
			updateNeeded = true
		default:
			return fmt.Errorf("target_date must be string or null")
		}
	}

	statusArg, hasStatus := stringArg(args, "status")
	if hasStatus {
		switch strings.ToLower(strings.TrimSpace(statusArg)) {
		case string(domain.ProjectArchived):
			if updateNeeded {
				if err := app.Projects.Update(ctx, p); err != nil {
					return err
				}
			}
			if p.ArchivedAt == nil {
				if err := app.Projects.Archive(ctx, p.ID); err != nil {
					return err
				}
			}
		case string(domain.ProjectActive), string(domain.ProjectPaused), string(domain.ProjectDone):
			if p.ArchivedAt != nil {
				if err := app.Projects.Unarchive(ctx, p.ID); err != nil {
					return err
				}
				p, err = app.Projects.GetByID(ctx, p.ID)
				if err != nil {
					return err
				}
			}
			p.Status = domain.ProjectStatus(strings.ToLower(strings.TrimSpace(statusArg)))
			updateNeeded = true
		default:
			return fmt.Errorf("invalid status %q (expected active|paused|done|archived)", statusArg)
		}
	}

	if updateNeeded && !(hasStatus && strings.EqualFold(strings.TrimSpace(statusArg), string(domain.ProjectArchived))) {
		if err := app.Projects.Update(ctx, p); err != nil {
			return err
		}
	}

	updated, err := app.Projects.GetByID(ctx, p.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Updated project %s [%s]\n", updated.Name, updated.ShortID)

	if boolArg(args, "recalc", false) {
		req := contract.NewStatusRequest()
		req.ProjectScope = []string{updated.ID}
		req.Recalc = true
		resp, err := app.Status.GetStatus(ctx, req)
		if err != nil {
			return err
		}
		fmt.Print(formatter.FormatStatus(resp))
	}

	return nil
}

func confirmPrompt() bool {
	return promptYesNoWithDefault("Confirm? [Y/n]: ", true)
}

func confirmPromptIO(in io.Reader, out io.Writer) bool {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return promptYesNoWithDefaultIO(in, out, "Confirm? [Y/n]: ", true)
}

package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/spf13/cobra"
)

func newAskCmd(app *App) *cobra.Command {
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
				if confirmPrompt() {
					return dispatchIntent(app, resolution.ParsedIntent)
				}
				fmt.Println("Cancelled.")
			case intelligence.StateNeedsClarification:
				// Display already handled by formatter.
			case intelligence.StateRejected:
				// Display already handled by formatter.
			}
			return nil
		},
	}
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
		fmt.Print(formatter.FormatWhatNow(resp))

	case intelligence.IntentStatus:
		req := contract.NewStatusRequest()
		resp, err := app.Status.GetStatus(ctx, req)
		if err != nil {
			return err
		}
		fmt.Print(formatter.FormatStatus(resp))

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

func confirmPrompt() bool {
	fmt.Print("Confirm? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))
	return text == "y" || text == "yes"
}

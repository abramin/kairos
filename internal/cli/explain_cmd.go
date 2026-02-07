package cli

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/spf13/cobra"
)

func newExplainCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain recommendations and decisions",
	}

	cmd.AddCommand(
		newExplainNowCmd(app),
		newExplainWhyNotCmd(app),
	)

	return cmd
}

func newExplainNowCmd(app *App) *cobra.Command {
	var minutes int

	cmd := &cobra.Command{
		Use:   "now",
		Short: "Explain current what-now recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplainNow(app, minutes)
		},
	}

	cmd.Flags().IntVar(&minutes, "minutes", 60, "Available minutes for recommendation context")
	return cmd
}

func runExplainNow(app *App, minutes int) error {
	ctx := context.Background()

	// Step 1: Run what-now to get deterministic results + trace.
	req := contract.NewWhatNowRequest(minutes)
	resp, err := app.WhatNow.Recommend(ctx, req)
	if err != nil {
		return err
	}

	// Step 2: Build trace from response.
	trace := intelligence.BuildRecommendationTrace(resp)

	// Step 3: Get explanation (LLM or fallback).
	var explanation *intelligence.LLMExplanation
	if app.Explain != nil {
		explanation, err = app.Explain.ExplainNow(ctx, trace)
		if err != nil {
			// LLM failed; use deterministic fallback.
			explanation = intelligence.DeterministicExplainNow(trace)
		}
	} else {
		explanation = intelligence.DeterministicExplainNow(trace)
	}

	fmt.Print(formatter.FormatWhatNow(resp))
	fmt.Println()
	fmt.Print(formatter.FormatExplanation(explanation))
	return nil
}

func newExplainWhyNotCmd(app *App) *cobra.Command {
	var projectID, workItemID string
	var minutes int

	cmd := &cobra.Command{
		Use:   "why-not",
		Short: "Explain why a specific item was not recommended",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			candidateID := workItemID
			if candidateID == "" {
				candidateID = projectID
			}
			if candidateID == "" {
				return fmt.Errorf("provide --project or --work-item to identify the candidate")
			}

			// Run what-now for trace.
			req := contract.NewWhatNowRequest(minutes)
			resp, err := app.WhatNow.Recommend(ctx, req)
			if err != nil {
				return err
			}

			trace := intelligence.BuildRecommendationTrace(resp)

			var explanation *intelligence.LLMExplanation
			if app.Explain != nil {
				explanation, err = app.Explain.ExplainWhyNot(ctx, trace, candidateID)
				if err != nil {
					explanation = intelligence.DeterministicWhyNot(trace, candidateID)
				}
			} else {
				explanation = intelligence.DeterministicWhyNot(trace, candidateID)
			}

			fmt.Print(formatter.FormatExplanation(explanation))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Project ID to explain")
	cmd.Flags().StringVar(&workItemID, "work-item", "", "Work item ID to explain")
	cmd.Flags().IntVar(&minutes, "minutes", 60, "Available minutes for recommendation context")

	return cmd
}

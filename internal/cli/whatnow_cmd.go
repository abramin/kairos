package cli

import (
	"context"
	"fmt"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/spf13/cobra"
)

func newWhatNowCmd(app *App) *cobra.Command {
	var minutes, maxSlices int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "what-now",
		Short: "Get session recommendations for available time",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := contract.NewWhatNowRequest(minutes)

			if cmd.Flags().Changed("dry-run") {
				req.DryRun = dryRun
			}
			if cmd.Flags().Changed("max-slices") {
				req.MaxSlices = maxSlices
			}

			resp, err := app.WhatNow.Recommend(context.Background(), req)
			if err != nil {
				return err
			}

			fmt.Print(formatter.FormatWhatNow(resp))
			return nil
		},
	}

	cmd.Flags().IntVar(&minutes, "minutes", 0, "Available minutes for the session")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show recommendations without persisting changes")
	cmd.Flags().IntVar(&maxSlices, "max-slices", 3, "Maximum number of work slices to recommend")
	_ = cmd.MarkFlagRequired("minutes")

	return cmd
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/cli/formatter"
)

// RunOneShot executes a single command non-interactively, prints to stdout, and returns.
// Called when kairos is invoked with command-line arguments.
func RunOneShot(a *App, cmd string, args []string) error {
	ctx := context.Background()
	switch cmd {
	case "what-now", "whatnow":
		return oneshotWhatNow(ctx, a, args)
	case "status":
		return oneshotStatus(ctx, a)
	case "chart":
		return oneshotChart(ctx, a, args)
	case "projects":
		return oneshotProjects(ctx, a)
	case "help":
		fmt.Println(formatter.FormatShellHelp())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\nRun 'kairos' (no arguments) for the interactive shell, or 'kairos help' for available commands", cmd)
	}
}

func oneshotWhatNow(ctx context.Context, a *App, args []string) error {
	minutes := 60
	if len(args) > 0 {
		if m, ok := parseDurationArg(args[0]); ok {
			minutes = m
		} else {
			return fmt.Errorf("invalid duration %q (try: 60, 90, 1h30m)", args[0])
		}
	}
	resp, err := a.WhatNow.Recommend(ctx, app.NewWhatNowRequest(minutes))
	if err != nil {
		return err
	}
	fmt.Println(formatWhatNowResponse(ctx, a, resp))
	return nil
}

func oneshotStatus(ctx context.Context, a *App) error {
	resp, err := a.Status.GetStatus(ctx, app.NewStatusRequest())
	if err != nil {
		return err
	}
	fmt.Println(formatter.FormatStatus(resp))
	return nil
}

func oneshotChart(ctx context.Context, a *App, args []string) error {
	numWeeks := 6
	for i, arg := range args {
		if (arg == "--weeks" || arg == "-w") && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
				numWeeks = n
			}
		} else if strings.HasPrefix(arg, "--weeks=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--weeks=")); err == nil && n > 0 {
				numWeeks = n
			}
		}
	}
	breakdown, err := a.Chart.WeeklyBreakdown(ctx, numWeeks)
	if err != nil {
		return err
	}
	fmt.Println(formatter.RenderChart(breakdown, oneshotTermWidth()))
	return nil
}

func oneshotProjects(ctx context.Context, a *App) error {
	projects, err := a.Projects.List(ctx, false)
	if err != nil {
		return err
	}
	fmt.Println(formatter.FormatProjectList(projects))
	return nil
}

// oneshotTermWidth returns the terminal width for one-shot mode.
// Checks COLUMNS env var, then falls back to 100.
func oneshotTermWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 100
}

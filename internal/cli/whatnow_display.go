package cli

import (
	"context"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
)

func formatWhatNowResponse(ctx context.Context, app *App, resp *contract.WhatNowResponse) string {
	if app == nil || app.Projects == nil {
		return formatter.FormatWhatNow(resp)
	}
	projectIDs := loadProjectDisplayIDs(ctx, app)
	return formatter.FormatWhatNowWithProjectIDs(resp, projectIDs)
}

func loadProjectDisplayIDs(ctx context.Context, app *App) map[string]string {
	projects, err := app.Projects.List(ctx, true)
	if err != nil {
		return nil
	}

	projectIDs := make(map[string]string, len(projects))
	for _, p := range projects {
		if displayID := strings.TrimSpace(p.ShortID); displayID != "" {
			projectIDs[p.ID] = displayID
		}
	}
	return projectIDs
}

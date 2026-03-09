package cli

import (
	"context"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/intelligence"
)

func formatWhatNowResponse(ctx context.Context, app *App, resp *contract.WhatNowResponse) string {
	if app == nil || app.Projects == nil {
		return formatter.FormatWhatNow(resp)
	}
	projectNames := loadProjectDisplayNames(ctx, app)
	summaries := loadItemSummaries(ctx, app, resp, projectNames)
	return formatter.FormatWhatNowWithProjectIDs(resp, projectNames, summaries)
}

func loadProjectDisplayNames(ctx context.Context, app *App) map[string]string {
	projects, err := app.Projects.List(ctx, true)
	if err != nil {
		return nil
	}

	names := make(map[string]string, len(projects))
	for _, p := range projects {
		if name := strings.TrimSpace(p.Name); name != "" {
			names[p.ID] = name
		}
	}
	return names
}

func loadItemSummaries(ctx context.Context, app *App, resp *contract.WhatNowResponse, projectNames map[string]string) map[string]string {
	if app.Explain == nil || len(resp.Recommendations) == 0 {
		return nil
	}
	trace := intelligence.BuildRecommendationTrace(resp)
	summaries, _ := app.Explain.SummarizeItems(ctx, trace, projectNames)
	return summaries
}

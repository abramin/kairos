package cli

import (
	"context"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubIntentService struct {
	resolution *intelligence.AskResolution
	err        error
}

func (s stubIntentService) Parse(ctx context.Context, text string) (*intelligence.AskResolution, error) {
	return s.resolution, s.err
}

func TestDispatchIntent_ProjectUpdate_StatusAndTargetDate(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Original Name", testutil.WithShortID("OUU01"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id":  "OUU01",
			"status":      "done",
			"target_date": "2023-12-31",
		},
	}

	require.NoError(t, dispatchIntent(app, intent))

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectDone, updated.Status)
	require.NotNil(t, updated.TargetDate)
	assert.Equal(t, "2023-12-31", updated.TargetDate.Format("2006-01-02"))
}

func TestDispatchIntent_ProjectUpdate_TargetDateNullClearsDate(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	proj := testutil.NewTestProject("Has Date", testutil.WithShortID("OUU02"), testutil.WithTargetDate(due))
	require.NoError(t, app.Projects.Create(ctx, proj))

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id":  "OUU02",
			"target_date": nil,
		},
	}

	require.NoError(t, dispatchIntent(app, intent))

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.TargetDate)
}

func TestDispatchIntent_ProjectUpdate_StatusArchivedUsesArchiveFlow(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Archivable", testutil.WithShortID("OUU03"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentProjectUpdate,
		Arguments: map[string]interface{}{
			"project_id": "OUU03",
			"status":     "archived",
		},
	}

	require.NoError(t, dispatchIntent(app, intent))

	active, err := app.Projects.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, active)

	all, err := app.Projects.List(ctx, true)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, domain.ProjectArchived, all[0].Status)
	require.NotNil(t, all[0].ArchivedAt)
}

func TestAskCmd_YesFlagExecutesNeedsConfirmationIntent(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()

	proj := testutil.NewTestProject("Update Me", testutil.WithShortID("OUU10"))
	require.NoError(t, app.Projects.Create(ctx, proj))

	app.Intent = stubIntentService{
		resolution: &intelligence.AskResolution{
			ParsedIntent: &intelligence.ParsedIntent{
				Intent: intelligence.IntentProjectUpdate,
				Risk:   intelligence.RiskWrite,
				Arguments: map[string]interface{}{
					"project_id":  "OUU10",
					"status":      "done",
					"target_date": "2026-02-09",
				},
				Confidence: 0.9,
			},
			ExecutionState:   intelligence.StateNeedsConfirmation,
			ExecutionMessage: "requires confirmation",
		},
	}

	_, err := executeCmd(t, app, "ask", "--yes", "mark OUU10 done")
	require.NoError(t, err)

	updated, err := app.Projects.GetByID(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProjectDone, updated.Status)
	require.NotNil(t, updated.TargetDate)
	assert.Equal(t, "2026-02-09", updated.TargetDate.Format("2006-01-02"))
}

// =============================================================================
// dispatchIntent Integration Tests — covers routing for non-update intents
// =============================================================================

func TestDispatchIntent_WhatNow(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentWhatNow,
		Arguments: map[string]interface{}{
			"available_min": float64(60),
		},
	}

	// Should succeed — dispatches to WhatNow.Recommend().
	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

func TestDispatchIntent_WhatNow_DefaultMinutes(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	// When available_min is absent, should fall back to 60.
	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentWhatNow,
		Arguments: map[string]interface{}{},
	}

	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

func TestDispatchIntent_Status(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentStatus,
		Arguments: map[string]interface{}{},
	}

	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

func TestDispatchIntent_ExplainNow(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	intent := &intelligence.ParsedIntent{
		Intent: intelligence.IntentExplainNow,
		Arguments: map[string]interface{}{
			"minutes": float64(60),
		},
	}

	// Uses deterministic fallback since app.Explain is nil.
	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

func TestDispatchIntent_ReviewWeekly(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)

	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentReviewWeekly,
		Arguments: map[string]interface{}{},
	}

	// Uses deterministic fallback since app.Explain is nil.
	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

func TestDispatchIntent_UnknownIntentShowsHint(t *testing.T) {
	app := testApp(t)

	intent := &intelligence.ParsedIntent{
		Intent:    intelligence.IntentTemplateList,
		Arguments: map[string]interface{}{},
	}

	// Template-related intents don't have direct dispatch —
	// should succeed (prints hint) without error.
	err := dispatchIntent(app, intent)
	require.NoError(t, err)
}

package cli

import (
	"context"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubIntentTUI struct {
	resolution *intelligence.AskResolution
	err        error
}

func (s *stubIntentTUI) Parse(_ context.Context, _ string) (*intelligence.AskResolution, error) {
	return s.resolution, s.err
}

type stubExplainTUI struct {
	explainNowResp  *intelligence.LLMExplanation
	whyNotResp      *intelligence.LLMExplanation
	weeklyReviewRes *intelligence.LLMExplanation
}

func (s *stubExplainTUI) ExplainNow(_ context.Context, _ intelligence.RecommendationTrace) (*intelligence.LLMExplanation, error) {
	return s.explainNowResp, nil
}

func (s *stubExplainTUI) ExplainWhyNot(_ context.Context, _ intelligence.RecommendationTrace, _ string) (*intelligence.LLMExplanation, error) {
	return s.whyNotResp, nil
}

func (s *stubExplainTUI) WeeklyReview(_ context.Context, _ intelligence.WeeklyReviewTrace) (*intelligence.LLMExplanation, error) {
	return s.weeklyReviewRes, nil
}

func TestCommandBar_AskUsageAndDisabled(t *testing.T) {
	app := testApp(t)
	cb := testCommandBar(t, app)

	output := execCmd(cb, "ask")
	assert.Contains(t, output, "Usage: ask")

	output = execCmd(cb, "ask what should i do")
	assert.Contains(t, output, "LLM features are disabled")
}

func TestCommandBar_AskAutoExecuteReadOnlyIntent(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	cb := testCommandBar(t, app)

	app.Intent = &stubIntentTUI{
		resolution: &intelligence.AskResolution{
			ParsedIntent: &intelligence.ParsedIntent{
				Intent:     intelligence.IntentWhatNow,
				Risk:       intelligence.RiskReadOnly,
				Arguments:  map[string]interface{}{"available_min": float64(60)},
				Confidence: 0.95,
			},
			ExecutionState:   intelligence.StateExecuted,
			ExecutionMessage: "ok",
		},
	}

	output := execCmdAsync(cb, "ask what should i work on")
	assert.Contains(t, output, "Intent:")
	assert.Contains(t, output, "MODE:")
}

func TestCommandBar_ExplainTUIFallbacks(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)
	cb := testCommandBar(t, app)

	output := execCmdAsync(cb, "explain now 60")
	assert.Contains(t, output, "EXPLANATION")

	output = execCmdAsync(cb, "explain why-not "+wiID)
	assert.Contains(t, output, "EXPLANATION")

	output = execCmdAsync(cb, "review weekly")
	assert.Contains(t, output, "EXPLANATION")
}

func TestCommandBar_ExplainAndReviewUseLLMServiceWhenAvailable(t *testing.T) {
	app := testApp(t)
	_, wiID := seedProjectWithWork(t, app)
	cb := testCommandBar(t, app)

	app.Explain = &stubExplainTUI{
		explainNowResp: &intelligence.LLMExplanation{
			Context:         intelligence.ExplainContextWhatNow,
			SummaryShort:    "LLM explain now summary",
			SummaryDetailed: "LLM explain now detail",
			Confidence:      0.9,
		},
		whyNotResp: &intelligence.LLMExplanation{
			Context:         intelligence.ExplainContextWhyNot,
			SummaryShort:    "LLM why-not summary",
			SummaryDetailed: "LLM why-not detail",
			Confidence:      0.8,
		},
		weeklyReviewRes: &intelligence.LLMExplanation{
			Context:         intelligence.ExplainContextWeeklyReview,
			SummaryShort:    "LLM weekly summary",
			SummaryDetailed: "LLM weekly detail",
			Confidence:      0.85,
		},
	}

	output := execCmdAsync(cb, "explain now 60")
	assert.Contains(t, output, "LLM explain now summary")

	output = execCmdAsync(cb, "explain why-not "+wiID)
	assert.Contains(t, output, "LLM why-not summary")

	output = execCmdAsync(cb, "review weekly")
	assert.Contains(t, output, "LLM weekly summary")
}

func TestCommandBar_AskExecutedStatusIntent(t *testing.T) {
	app := testApp(t)
	seedProjectWithWork(t, app)
	cb := testCommandBar(t, app)

	app.Intent = &stubIntentTUI{
		resolution: &intelligence.AskResolution{
			ParsedIntent: &intelligence.ParsedIntent{
				Intent:     intelligence.IntentStatus,
				Risk:       intelligence.RiskReadOnly,
				Arguments:  map[string]interface{}{},
				Confidence: 0.91,
			},
			ExecutionState: intelligence.StateExecuted,
		},
	}

	output := execCmdAsync(cb, "ask show me status")
	require.NotEmpty(t, output)
	assert.Contains(t, output, "Intent:")
	assert.Contains(t, output, "status")
}

func TestCommandBar_ReviewWeekly_ShowsZettelkastenBacklog(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, nodeID, _ := seedProjectCore(t, app, seedOpts{})

	reading := testutil.NewTestWorkItem(nodeID, "Read Ch. 3")
	reading.Type = "reading"
	require.NoError(t, app.WorkItems.Create(ctx, reading))
	require.NoError(t, app.Sessions.LogSession(ctx, testutil.NewTestSession(reading.ID, 75)))

	cb := testCommandBar(t, app)
	output := execCmdAsync(cb, "review weekly")

	assert.Contains(t, output, "ZETTELKASTEN BACKLOG")
	assert.Contains(t, output, "75 min reading / 0 min zettel processing")
	assert.Contains(t, output, "Read Ch. 3")
}

func TestCommandBar_ReviewWeekly_HidesZettelkastenBacklogWhenRatioIsLow(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, nodeID, _ := seedProjectCore(t, app, seedOpts{})

	reading := testutil.NewTestWorkItem(nodeID, "Read Ch. 4")
	reading.Type = "reading"
	require.NoError(t, app.WorkItems.Create(ctx, reading))
	require.NoError(t, app.Sessions.LogSession(ctx, testutil.NewTestSession(reading.ID, 60)))

	zettel := testutil.NewTestWorkItem(nodeID, "Process Ch. 4 notes")
	zettel.Type = "zettel"
	zettel.Status = domain.WorkItemInProgress
	require.NoError(t, app.WorkItems.Create(ctx, zettel))
	require.NoError(t, app.Sessions.LogSession(ctx, testutil.NewTestSession(zettel.ID, 30)))

	cb := testCommandBar(t, app)
	output := execCmdAsync(cb, "review weekly")

	assert.NotContains(t, output, "ZETTELKASTEN BACKLOG")
}

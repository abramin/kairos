package intelligence

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTrace() RecommendationTrace {
	delta := 25.0
	return RecommendationTrace{
		Mode:         "balanced",
		RequestedMin: 60,
		AllocatedMin: 45,
		Recommendations: []RecommendationTraceItem{
			{
				WorkItemID:   "item-1",
				Title:        "Week 3 Study",
				ProjectID:    "proj-1",
				AllocatedMin: 45,
				Score:        72.5,
				RiskLevel:    "at_risk",
				Reasons: []ReasonTraceItem{
					{Code: "DEADLINE_PRESSURE", Message: "Due this week", WeightDelta: &delta},
				},
			},
		},
		RiskProjects: []RiskTraceItem{
			{
				ProjectID:        "proj-1",
				ProjectName:      "OU Module",
				RiskLevel:        "at_risk",
				DaysLeft:         intPtr(5),
				RequiredDailyMin: 45.0,
				SlackMinPerDay:   -5.0,
				ProgressTimePct:  60.0,
			},
		},
	}
}

func TestExplainNow_FallbackWhenLLMDown(t *testing.T) {
	client := &mockLLMClient{err: llm.ErrOllamaUnavailable}
	svc := NewExplainService(client, llm.NoopObserver{})
	trace := testTrace()

	explanation, err := svc.ExplainNow(context.Background(), trace)

	require.NoError(t, err)
	assert.Equal(t, ExplainContextWhatNow, explanation.Context)
	assert.Equal(t, float64(1.0), explanation.Confidence)
	assert.NotEmpty(t, explanation.SummaryShort)
	assert.NotEmpty(t, explanation.Factors)
}

func TestExplainNow_FallbackOnInvalidEvidence(t *testing.T) {
	// LLM returns explanation with bad evidence_ref_key.
	badExplanation := LLMExplanation{
		Context:      ExplainContextWhatNow,
		SummaryShort: "test",
		Factors: []ExplanationFactor{
			{
				Name:           "Fake Factor",
				EvidenceRefKey: "totally.made.up.key",
			},
		},
		Confidence: 0.8,
	}
	data, _ := json.Marshal(badExplanation)
	client := &mockLLMClient{response: string(data)}
	svc := NewExplainService(client, llm.NoopObserver{})
	trace := testTrace()

	explanation, err := svc.ExplainNow(context.Background(), trace)

	require.NoError(t, err)
	// Should get deterministic fallback, not the LLM output.
	assert.Equal(t, float64(1.0), explanation.Confidence)
}

func TestExplainNow_ValidLLMResponse(t *testing.T) {
	trace := testTrace()
	validExplanation := LLMExplanation{
		Context:         ExplainContextWhatNow,
		SummaryShort:    "OU Module study recommended due to upcoming deadline.",
		SummaryDetailed: "Detailed explanation.",
		Factors: []ExplanationFactor{
			{
				Name:            "Deadline pressure",
				Impact:          "high",
				Direction:       "push_for",
				EvidenceRefType: EvidenceScoreFactor,
				EvidenceRefKey:  "rec.item-1.reason.DEADLINE_PRESSURE",
				Summary:         "Due this week",
			},
		},
		Confidence: 0.9,
	}
	data, _ := json.Marshal(validExplanation)
	client := &mockLLMClient{response: string(data)}
	svc := NewExplainService(client, llm.NoopObserver{})

	explanation, err := svc.ExplainNow(context.Background(), trace)

	require.NoError(t, err)
	assert.Equal(t, 0.9, explanation.Confidence)
	assert.Contains(t, explanation.SummaryShort, "deadline")
}

func TestExplainWhyNot_Fallback(t *testing.T) {
	client := &mockLLMClient{err: llm.ErrTimeout}
	svc := NewExplainService(client, llm.NoopObserver{})
	trace := testTrace()
	trace.Blockers = []BlockerTraceItem{
		{EntityID: "w2", Code: "DEPENDENCY", Message: "Has unfinished predecessors"},
	}

	explanation, err := svc.ExplainWhyNot(context.Background(), trace, "w2")

	require.NoError(t, err)
	assert.Equal(t, ExplainContextWhyNot, explanation.Context)
	assert.Contains(t, explanation.SummaryShort, "Blocked")
}

func TestDeterministicExplainNow(t *testing.T) {
	trace := testTrace()
	explanation := DeterministicExplainNow(trace)

	assert.Equal(t, ExplainContextWhatNow, explanation.Context)
	assert.Equal(t, float64(1.0), explanation.Confidence)
	assert.Contains(t, explanation.SummaryShort, "1 item(s)")
	assert.Contains(t, explanation.SummaryShort, "balanced")
	assert.NotEmpty(t, explanation.Factors)
}

func TestDeterministicWhyNot_BlockedItem(t *testing.T) {
	trace := testTrace()
	trace.Blockers = []BlockerTraceItem{
		{EntityID: "w3", Code: "NOT_BEFORE", Message: "Not available until 2025-04-01"},
	}

	explanation := DeterministicWhyNot(trace, "w3")

	assert.Equal(t, ExplainContextWhyNot, explanation.Context)
	assert.Contains(t, explanation.SummaryShort, "Blocked")
	assert.Contains(t, explanation.SummaryDetailed, "NOT_BEFORE")
}

func TestDeterministicWhyNot_NotBlocked(t *testing.T) {
	trace := testTrace()
	explanation := DeterministicWhyNot(trace, "unknown-item")

	assert.Contains(t, explanation.SummaryShort, "not in the top recommendations")
}

func TestDeterministicWeeklyReview(t *testing.T) {
	trace := WeeklyReviewTrace{
		PeriodDays:     7,
		TotalLoggedMin: 300,
		SessionCount:   8,
		ProjectSummaries: []ProjectWeeklySummary{
			{ProjectID: "p1", ProjectName: "OU Module", PlannedMin: 600, LoggedMin: 200, RiskLevel: "at_risk", SessionsCount: 5},
			{ProjectID: "p2", ProjectName: "Calimove", PlannedMin: 300, LoggedMin: 100, RiskLevel: "on_track", SessionsCount: 3},
		},
	}

	explanation := DeterministicWeeklyReview(trace)

	assert.Equal(t, ExplainContextWeeklyReview, explanation.Context)
	assert.Contains(t, explanation.SummaryShort, "8 sessions")
	assert.Contains(t, explanation.SummaryShort, "300 minutes")
	assert.Len(t, explanation.Factors, 2)
}

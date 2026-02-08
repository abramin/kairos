package formatter

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/stretchr/testify/assert"
)

func TestFormatExplanation_IncludesFactorsCounterfactualsAndConfidence(t *testing.T) {
	e := &intelligence.LLMExplanation{
		SummaryShort:    "Focus on Chapter 3 now.",
		SummaryDetailed: "Chapter 3 has the tightest deadline and highest pacing pressure.",
		Factors: []intelligence.ExplanationFactor{
			{
				Name:      "Deadline pressure",
				Impact:    "high",
				Direction: "push_for",
				Summary:   "Due date is near.",
			},
			{
				Name:      "Spacing",
				Impact:    "low",
				Direction: "push_against",
				Summary:   "You worked this area yesterday.",
			},
		},
		Counterfactuals: []intelligence.Counterfactual{
			{
				Label:           "If deadline moved one week later",
				PredictedEffect: "Lower urgency and more balanced sequencing.",
			},
		},
		Confidence: 0.82,
	}

	out := FormatExplanation(e)
	assert.Contains(t, out, "Focus on Chapter 3 now.")
	assert.Contains(t, out, "FACTORS")
	assert.Contains(t, out, "Deadline pressure")
	assert.Contains(t, out, "Spacing")
	assert.Contains(t, out, "WHAT IF")
	assert.Contains(t, out, "If deadline moved one week later")
	assert.Contains(t, out, "Confidence: 82%")
}

func TestFormatExplanation_OmitsDuplicateDetailedSummary(t *testing.T) {
	e := &intelligence.LLMExplanation{
		SummaryShort:    "Use kairos status for risk overview.",
		SummaryDetailed: "Use kairos status for risk overview.",
		Confidence:      1.0,
	}

	out := FormatExplanation(e)
	assert.Contains(t, out, "Use kairos status for risk overview.")
	assert.NotContains(t, out, "What If")
	assert.Contains(t, out, "Confidence: 100%")
}

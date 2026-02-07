package intelligence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEvidenceBindings_AllValid(t *testing.T) {
	keys := map[string]bool{
		"rec.item1.reason.DEADLINE_PRESSURE": true,
		"risk.proj1.required_daily_min":      true,
	}
	factors := []ExplanationFactor{
		{Name: "Deadline", EvidenceRefKey: "rec.item1.reason.DEADLINE_PRESSURE"},
		{Name: "Pace", EvidenceRefKey: "risk.proj1.required_daily_min"},
	}
	assert.NoError(t, ValidateEvidenceBindings(factors, keys))
}

func TestValidateEvidenceBindings_InvalidKeyRejected(t *testing.T) {
	keys := map[string]bool{
		"rec.item1.reason.DEADLINE_PRESSURE": true,
	}
	factors := []ExplanationFactor{
		{Name: "Deadline", EvidenceRefKey: "rec.item1.reason.DEADLINE_PRESSURE"},
		{Name: "Fake", EvidenceRefKey: "nonexistent.key"},
	}
	err := ValidateEvidenceBindings(factors, keys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.key")
}

func TestValidateEvidenceBindings_EmptyKeyRejected(t *testing.T) {
	keys := map[string]bool{"rec.item1.score": true}
	factors := []ExplanationFactor{
		{Name: "Missing", EvidenceRefKey: ""},
	}
	err := ValidateEvidenceBindings(factors, keys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty evidence_ref_key")
}

func TestValidateEvidenceBindings_EmptyFactors(t *testing.T) {
	keys := map[string]bool{"rec.item1.score": true}
	assert.NoError(t, ValidateEvidenceBindings(nil, keys))
	assert.NoError(t, ValidateEvidenceBindings([]ExplanationFactor{}, keys))
}

func TestTraceKeys_ContainsExpectedKeys(t *testing.T) {
	trace := RecommendationTrace{
		Mode:         "balanced",
		RequestedMin: 60,
		AllocatedMin: 45,
		Recommendations: []RecommendationTraceItem{
			{
				WorkItemID: "w1",
				Reasons: []ReasonTraceItem{
					{Code: "DEADLINE_PRESSURE"},
					{Code: "SPACING_OK"},
				},
			},
		},
		Blockers: []BlockerTraceItem{
			{EntityID: "w2", Code: "DEPENDENCY"},
		},
		RiskProjects: []RiskTraceItem{
			{ProjectID: "p1", DaysLeft: intPtr(5)},
		},
	}

	keys := trace.TraceKeys()

	assert.True(t, keys["mode"])
	assert.True(t, keys["requested_min"])
	assert.True(t, keys["allocated_min"])
	assert.True(t, keys["rec.w1.score"])
	assert.True(t, keys["rec.w1.risk_level"])
	assert.True(t, keys["rec.w1.allocated_min"])
	assert.True(t, keys["rec.w1.reason.DEADLINE_PRESSURE"])
	assert.True(t, keys["rec.w1.reason.SPACING_OK"])
	assert.True(t, keys["blocker.w2.DEPENDENCY"])
	assert.True(t, keys["risk.p1.risk_level"])
	assert.True(t, keys["risk.p1.required_daily_min"])
	assert.True(t, keys["risk.p1.days_left"])
	assert.False(t, keys["nonexistent"])
}

func intPtr(n int) *int { return &n }

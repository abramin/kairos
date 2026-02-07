package intelligence

import "fmt"

// DeterministicExplainNow builds an explanation directly from trace data
// without using the LLM. Used as a fallback when Ollama is unavailable or
// when the LLM output fails validation.
func DeterministicExplainNow(trace RecommendationTrace) *LLMExplanation {
	explanation := &LLMExplanation{
		Context:    ExplainContextWhatNow,
		Confidence: 1.0, // deterministic = fully faithful
	}

	n := len(trace.Recommendations)
	explanation.SummaryShort = fmt.Sprintf("Recommended %d item(s) in %s mode, using %d of %d available minutes.",
		n, trace.Mode, trace.AllocatedMin, trace.RequestedMin)

	explanation.SummaryDetailed = explanation.SummaryShort
	if len(trace.RiskProjects) > 0 {
		rp := trace.RiskProjects[0]
		explanation.SummaryDetailed += fmt.Sprintf(" Top risk: %s (%s, %.0f%% complete).",
			rp.ProjectName, rp.RiskLevel, rp.ProgressTimePct)
	}

	for _, rec := range trace.Recommendations {
		for _, reason := range rec.Reasons {
			explanation.Factors = append(explanation.Factors, ExplanationFactor{
				Name:            reason.Code,
				Impact:          impactFromDelta(reason.WeightDelta),
				Direction:       directionFromDelta(reason.WeightDelta),
				EvidenceRefType: EvidenceScoreFactor,
				EvidenceRefKey:  "rec." + rec.WorkItemID + ".reason." + reason.Code,
				Summary:         reason.Message,
			})
		}
	}

	return explanation
}

// DeterministicWhyNot builds a why-not explanation from trace data.
func DeterministicWhyNot(trace RecommendationTrace, candidateID string) *LLMExplanation {
	explanation := &LLMExplanation{
		Context:    ExplainContextWhyNot,
		Confidence: 1.0,
	}

	// Check if item is in blockers.
	for _, b := range trace.Blockers {
		if b.EntityID == candidateID {
			explanation.SummaryShort = fmt.Sprintf("Blocked: %s", b.Message)
			explanation.SummaryDetailed = fmt.Sprintf("This item was not recommended because: %s (blocker code: %s).", b.Message, b.Code)
			explanation.Factors = append(explanation.Factors, ExplanationFactor{
				Name:            b.Code,
				Impact:          "high",
				Direction:       "push_against",
				EvidenceRefType: EvidenceConstraint,
				EvidenceRefKey:  "blocker." + b.EntityID + "." + b.Code,
				Summary:         b.Message,
			})
			return explanation
		}
	}

	explanation.SummaryShort = "This item was not in the top recommendations."
	explanation.SummaryDetailed = "The item scored lower than the recommended items based on deadline pressure, risk level, spacing, and variation factors."
	return explanation
}

// DeterministicWeeklyReview builds a weekly review from trace data.
func DeterministicWeeklyReview(trace WeeklyReviewTrace) *LLMExplanation {
	explanation := &LLMExplanation{
		Context:    ExplainContextWeeklyReview,
		Confidence: 1.0,
	}

	explanation.SummaryShort = fmt.Sprintf("Past %d days: %d sessions, %d minutes logged across %d project(s).",
		trace.PeriodDays, trace.SessionCount, trace.TotalLoggedMin, len(trace.ProjectSummaries))

	explanation.SummaryDetailed = explanation.SummaryShort
	for _, p := range trace.ProjectSummaries {
		pct := float64(0)
		if p.PlannedMin > 0 {
			pct = float64(p.LoggedMin) / float64(p.PlannedMin) * 100
		}
		explanation.Factors = append(explanation.Factors, ExplanationFactor{
			Name:            p.ProjectName,
			Impact:          riskToImpact(p.RiskLevel),
			Direction:       "push_for",
			EvidenceRefType: EvidenceHistory,
			EvidenceRefKey:  "project." + p.ProjectID + ".logged_min",
			Summary:         fmt.Sprintf("%d min logged (%.0f%% of planned), %d sessions, risk: %s", p.LoggedMin, pct, p.SessionsCount, p.RiskLevel),
		})
	}

	return explanation
}

func impactFromDelta(delta *float64) string {
	if delta == nil {
		return "low"
	}
	d := *delta
	if d < 0 {
		d = -d
	}
	switch {
	case d >= 20:
		return "high"
	case d >= 5:
		return "medium"
	default:
		return "low"
	}
}

func directionFromDelta(delta *float64) string {
	if delta == nil {
		return "push_for"
	}
	if *delta < 0 {
		return "push_against"
	}
	return "push_for"
}

func riskToImpact(risk string) string {
	switch risk {
	case "critical":
		return "high"
	case "at_risk":
		return "medium"
	default:
		return "low"
	}
}

package intelligence

import (
	"github.com/alexanderramin/kairos/internal/contract"
)

// RecommendationTrace is a flattened, JSON-serializable view of all deterministic
// data that produced a what-now recommendation. Passed to the LLM as context
// for generating faithful explanations.
type RecommendationTrace struct {
	Mode            string                    `json:"mode"`
	RequestedMin    int                       `json:"requested_min"`
	AllocatedMin    int                       `json:"allocated_min"`
	Recommendations []RecommendationTraceItem `json:"recommendations"`
	Blockers        []BlockerTraceItem        `json:"blockers"`
	RiskProjects    []RiskTraceItem           `json:"risk_projects"`
	PolicyMessages  []string                  `json:"policy_messages"`
}

// RecommendationTraceItem captures one recommended work slice with its scoring trace.
type RecommendationTraceItem struct {
	WorkItemID   string            `json:"work_item_id"`
	Title        string            `json:"title"`
	ProjectID    string            `json:"project_id"`
	AllocatedMin int               `json:"allocated_min"`
	Score        float64           `json:"score"`
	RiskLevel    string            `json:"risk_level"`
	DueDate      *string           `json:"due_date,omitempty"`
	Reasons      []ReasonTraceItem `json:"reasons"`
}

// ReasonTraceItem captures a single scoring reason.
type ReasonTraceItem struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	WeightDelta *float64 `json:"weight_delta,omitempty"`
}

// BlockerTraceItem captures a constraint blocker.
type BlockerTraceItem struct {
	EntityID string `json:"entity_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// RiskTraceItem captures per-project risk data.
type RiskTraceItem struct {
	ProjectID        string  `json:"project_id"`
	ProjectName      string  `json:"project_name"`
	RiskLevel        string  `json:"risk_level"`
	DaysLeft         *int    `json:"days_left,omitempty"`
	RequiredDailyMin float64 `json:"required_daily_min"`
	SlackMinPerDay   float64 `json:"slack_min_per_day"`
	ProgressTimePct  float64 `json:"progress_time_pct"`
}

// BuildRecommendationTrace converts a WhatNowResponse into a trace
// suitable for the explanation service.
func BuildRecommendationTrace(resp *contract.WhatNowResponse) RecommendationTrace {
	trace := RecommendationTrace{
		Mode:           string(resp.Mode),
		RequestedMin:   resp.RequestedMin,
		AllocatedMin:   resp.AllocatedMin,
		PolicyMessages: resp.PolicyMessages,
	}

	for _, rec := range resp.Recommendations {
		item := RecommendationTraceItem{
			WorkItemID:   rec.WorkItemID,
			Title:        rec.Title,
			ProjectID:    rec.ProjectID,
			AllocatedMin: rec.AllocatedMin,
			Score:        rec.Score,
			RiskLevel:    string(rec.RiskLevel),
			DueDate:      rec.DueDate,
		}
		for _, r := range rec.Reasons {
			item.Reasons = append(item.Reasons, ReasonTraceItem{
				Code:        string(r.Code),
				Message:     r.Message,
				WeightDelta: r.WeightDelta,
			})
		}
		trace.Recommendations = append(trace.Recommendations, item)
	}

	for _, b := range resp.Blockers {
		trace.Blockers = append(trace.Blockers, BlockerTraceItem{
			EntityID: b.EntityID,
			Code:     string(b.Code),
			Message:  b.Message,
		})
	}

	for _, r := range resp.TopRiskProjects {
		trace.RiskProjects = append(trace.RiskProjects, RiskTraceItem{
			ProjectID:        r.ProjectID,
			ProjectName:      r.ProjectName,
			RiskLevel:        string(r.RiskLevel),
			DaysLeft:         r.DaysLeft,
			RequiredDailyMin: r.RequiredDailyMin,
			SlackMinPerDay:   r.SlackMinPerDay,
			ProgressTimePct:  r.ProgressTimePct,
		})
	}

	return trace
}

// TraceKeys returns all valid evidence_ref_keys that an explanation may reference.
func (t RecommendationTrace) TraceKeys() map[string]bool {
	keys := map[string]bool{
		"mode": true, "requested_min": true, "allocated_min": true,
	}
	for _, rec := range t.Recommendations {
		prefix := "rec." + rec.WorkItemID
		keys[prefix+".score"] = true
		keys[prefix+".risk_level"] = true
		keys[prefix+".allocated_min"] = true
		for _, r := range rec.Reasons {
			keys[prefix+".reason."+r.Code] = true
		}
	}
	for _, b := range t.Blockers {
		keys["blocker."+b.EntityID+"."+b.Code] = true
	}
	for _, r := range t.RiskProjects {
		prefix := "risk." + r.ProjectID
		keys[prefix+".risk_level"] = true
		keys[prefix+".required_daily_min"] = true
		keys[prefix+".slack_min_per_day"] = true
		keys[prefix+".progress_time_pct"] = true
		if r.DaysLeft != nil {
			keys[prefix+".days_left"] = true
		}
	}
	return keys
}

// WeeklyReviewTrace holds data for a weekly review explanation.
type WeeklyReviewTrace struct {
	PeriodDays       int                     `json:"period_days"`
	ProjectSummaries []ProjectWeeklySummary  `json:"project_summaries"`
	TotalLoggedMin   int                     `json:"total_logged_min"`
	SessionCount     int                     `json:"session_count"`
}

// ProjectWeeklySummary holds per-project weekly data.
type ProjectWeeklySummary struct {
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	PlannedMin    int    `json:"planned_min"`
	LoggedMin     int    `json:"logged_min"`
	RiskLevel     string `json:"risk_level"`
	SessionsCount int    `json:"sessions_count"`
}

// WeeklyTraceKeys returns all valid evidence_ref_keys for a weekly review trace.
func (t WeeklyReviewTrace) WeeklyTraceKeys() map[string]bool {
	keys := map[string]bool{
		"period_days": true, "total_logged_min": true, "session_count": true,
	}
	for _, p := range t.ProjectSummaries {
		prefix := "project." + p.ProjectID
		keys[prefix+".planned_min"] = true
		keys[prefix+".logged_min"] = true
		keys[prefix+".risk_level"] = true
		keys[prefix+".sessions_count"] = true
	}
	return keys
}

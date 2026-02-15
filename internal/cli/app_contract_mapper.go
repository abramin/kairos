package cli

import (
	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/contract"
)

func mapWhatNowResponseToContract(resp *app.WhatNowResponse) *contract.WhatNowResponse {
	if resp == nil {
		return nil
	}

	out := &contract.WhatNowResponse{
		GeneratedAt:    resp.GeneratedAt,
		Mode:           resp.Mode,
		RequestedMin:   resp.RequestedMin,
		AllocatedMin:   resp.AllocatedMin,
		UnallocatedMin: resp.UnallocatedMin,
		PolicyMessages: append([]string(nil), resp.PolicyMessages...),
		Warnings:       append([]string(nil), resp.Warnings...),
	}

	out.Recommendations = make([]contract.WorkSlice, 0, len(resp.Recommendations))
	for _, rec := range resp.Recommendations {
		reasons := make([]contract.RecommendationReason, 0, len(rec.Reasons))
		for _, reason := range rec.Reasons {
			reasons = append(reasons, contract.RecommendationReason{
				Code:        contract.RecommendationReasonCode(reason.Code),
				Message:     reason.Message,
				WeightDelta: reason.WeightDelta,
			})
		}

		out.Recommendations = append(out.Recommendations, contract.WorkSlice{
			WorkItemID:        rec.WorkItemID,
			WorkItemSeq:       rec.WorkItemSeq,
			ProjectID:         rec.ProjectID,
			NodeID:            rec.NodeID,
			Title:             rec.Title,
			AllocatedMin:      rec.AllocatedMin,
			MinSessionMin:     rec.MinSessionMin,
			MaxSessionMin:     rec.MaxSessionMin,
			DefaultSessionMin: rec.DefaultSessionMin,
			Splittable:        rec.Splittable,
			DueDate:           rec.DueDate,
			RiskLevel:         rec.RiskLevel,
			Score:             rec.Score,
			Reasons:           reasons,
		})
	}

	out.Blockers = make([]contract.ConstraintBlocker, 0, len(resp.Blockers))
	for _, blocker := range resp.Blockers {
		out.Blockers = append(out.Blockers, contract.ConstraintBlocker{
			EntityType: blocker.EntityType,
			EntityID:   blocker.EntityID,
			Code:       contract.ConstraintBlockerCode(blocker.Code),
			Message:    blocker.Message,
		})
	}

	out.TopRiskProjects = make([]contract.RiskSummary, 0, len(resp.TopRiskProjects))
	for _, risk := range resp.TopRiskProjects {
		out.TopRiskProjects = append(out.TopRiskProjects, contract.RiskSummary{
			ProjectID:         risk.ProjectID,
			ProjectName:       risk.ProjectName,
			RiskLevel:         risk.RiskLevel,
			DueDate:           risk.DueDate,
			DaysLeft:          risk.DaysLeft,
			PlannedMinTotal:   risk.PlannedMinTotal,
			LoggedMinTotal:    risk.LoggedMinTotal,
			RemainingMinTotal: risk.RemainingMinTotal,
			RequiredDailyMin:  risk.RequiredDailyMin,
			RecentDailyMin:    risk.RecentDailyMin,
			SlackMinPerDay:    risk.SlackMinPerDay,
			ProgressTimePct:   risk.ProgressTimePct,
		})
	}

	return out
}

func mapStatusResponseToContract(resp *app.StatusResponse) *contract.StatusResponse {
	if resp == nil {
		return nil
	}

	out := &contract.StatusResponse{
		Summary: contract.GlobalStatusSummary{
			GeneratedAt:     resp.Summary.GeneratedAt,
			CountsTotal:     resp.Summary.CountsTotal,
			CountsOnTrack:   resp.Summary.CountsOnTrack,
			CountsAtRisk:    resp.Summary.CountsAtRisk,
			CountsCritical:  resp.Summary.CountsCritical,
			GlobalModeIfNow: resp.Summary.GlobalModeIfNow,
			PolicyMessage:   resp.Summary.PolicyMessage,
		},
		Warnings: append([]string(nil), resp.Warnings...),
	}

	out.Projects = make([]contract.ProjectStatusView, 0, len(resp.Projects))
	for _, view := range resp.Projects {
		out.Projects = append(out.Projects, contract.ProjectStatusView{
			ProjectID:             view.ProjectID,
			ProjectName:           view.ProjectName,
			Status:                view.Status,
			RiskLevel:             view.RiskLevel,
			DueDate:               view.DueDate,
			DaysLeft:              view.DaysLeft,
			ProgressTimePct:       view.ProgressTimePct,
			ProgressStructuralPct: view.ProgressStructuralPct,
			PlannedMinTotal:       view.PlannedMinTotal,
			LoggedMinTotal:        view.LoggedMinTotal,
			RemainingMinTotal:     view.RemainingMinTotal,
			RequiredDailyMin:      view.RequiredDailyMin,
			RecentDailyMin:        view.RecentDailyMin,
			SlackMinPerDay:        view.SlackMinPerDay,
			SafeForSecondaryWork:  view.SafeForSecondaryWork,
			Notes:                 append([]string(nil), view.Notes...),
		})
	}

	out.Blockers = make([]contract.ConstraintBlocker, 0, len(resp.Blockers))
	for _, blocker := range resp.Blockers {
		out.Blockers = append(out.Blockers, contract.ConstraintBlocker{
			EntityType: blocker.EntityType,
			EntityID:   blocker.EntityID,
			Code:       contract.ConstraintBlockerCode(blocker.Code),
			Message:    blocker.Message,
		})
	}

	return out
}

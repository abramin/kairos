package scheduler

import (
	"sort"

	"github.com/alexanderramin/kairos/internal/domain"
)

// RiskPriority returns a sort priority (lower = more urgent).
func RiskPriority(r domain.RiskLevel) int {
	switch r {
	case domain.RiskCritical:
		return 0
	case domain.RiskAtRisk:
		return 1
	default:
		return 2
	}
}

// CanonicalSort sorts scored candidates by the deterministic canonical rules:
// 1. Risk: critical > at_risk > on_track
// 2. Due date: earliest first (nil last)
// 3. Score: higher first
// 4. Project name: lexical ascending
// 5. Work item ID: lexical ascending
func CanonicalSort(candidates []ScoredCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]

		// 1. Risk priority
		riskA, riskB := RiskPriority(a.Input.ProjectRisk), RiskPriority(b.Input.ProjectRisk)
		if riskA != riskB {
			return riskA < riskB
		}

		// 2. Due date (earliest first, nil last)
		dueDateA, dueDateB := a.Input.DueDate, b.Input.DueDate
		if (dueDateA == nil) != (dueDateB == nil) {
			return dueDateA != nil // non-nil before nil
		}
		if dueDateA != nil && dueDateB != nil && !dueDateA.Equal(*dueDateB) {
			return dueDateA.Before(*dueDateB)
		}

		// 3. Score (higher first)
		if a.Score != b.Score {
			return a.Score > b.Score
		}

		// 4. Project name (lexical)
		if a.Input.ProjectName != b.Input.ProjectName {
			return a.Input.ProjectName < b.Input.ProjectName
		}

		// 5. Work item ID (lexical)
		return a.Input.WorkItemID < b.Input.WorkItemID
	})
}

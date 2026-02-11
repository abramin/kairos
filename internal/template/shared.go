package template

import (
	"sort"

	"github.com/alexanderramin/kairos/internal/domain"
)

// SessionPolicyField retrieves a named field from a SessionPolicyConfig.
// Returns nil if the policy is nil. Valid fields: "min", "max", "default".
func SessionPolicyField(sp *SessionPolicyConfig, field string) *int {
	if sp == nil {
		return nil
	}
	switch field {
	case "min":
		return sp.MinSessionMin
	case "max":
		return sp.MaxSessionMin
	case "default":
		return sp.DefaultSessionMin
	}
	return nil
}

// SessionPolicySplittable returns the Splittable field from a SessionPolicyConfig,
// or nil if the policy is nil.
func SessionPolicySplittable(sp *SessionPolicyConfig) *bool {
	if sp == nil {
		return nil
	}
	return sp.Splittable
}

// DependencyCandidate represents a work item candidate for linear dependency inference.
// Both the template engine and importer build these from their respective types.
type DependencyCandidate struct {
	ID        string // resolved UUID (or ref-mapped ID)
	NodeOrder int    // ordering index of the parent node
	NodePos   int    // positional index of the parent node in the node list
	ItemPos   int    // positional index of the work item in the work item list
}

// InferLinearDependencies creates a linear chain of dependencies from sorted candidates.
// Items are sorted by (NodeOrder, NodePos, ItemPos) and each consecutive pair becomes
// a predecessor->successor dependency. Pairs with identical IDs are skipped.
func InferLinearDependencies(candidates []DependencyCandidate) []domain.Dependency {
	if len(candidates) < 2 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].NodeOrder != candidates[j].NodeOrder {
			return candidates[i].NodeOrder < candidates[j].NodeOrder
		}
		if candidates[i].NodePos != candidates[j].NodePos {
			return candidates[i].NodePos < candidates[j].NodePos
		}
		return candidates[i].ItemPos < candidates[j].ItemPos
	})

	deps := make([]domain.Dependency, 0, len(candidates)-1)
	for i := 0; i < len(candidates)-1; i++ {
		predID := candidates[i].ID
		succID := candidates[i+1].ID
		if predID == "" || succID == "" || predID == succID {
			continue
		}
		deps = append(deps, domain.Dependency{
			PredecessorWorkItemID: predID,
			SuccessorWorkItemID:   succID,
		})
	}

	return deps
}

package generation

import (
	"fmt"
	"sort"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
)

const dateLayout = "2006-01-02"

// SessionPolicy is a minimal adapter interface shared by template/import schemas.
type SessionPolicy interface {
	MinSessionValue() *int
	MaxSessionValue() *int
	DefaultSessionValue() *int
	SplittableValue() *bool
}

// WorkItemDefaultsInput contains work-item fields participating in defaults cascade.
type WorkItemDefaultsInput struct {
	DurationMode       string
	SessionPolicy      SessionPolicy
	PlannedMin         *int
	EstimateConfidence *float64
}

// ResolvedWorkItemDefaults is the resolved result of defaults cascade.
type ResolvedWorkItemDefaults struct {
	DurationMode       string
	PlannedMin         int
	EstimateConfidence float64
	MinSessionMin      int
	MaxSessionMin      int
	DefaultSessionMin  int
	Splittable         bool
}

// ResolveWorkItemDefaults applies defaults cascade: item > defaults > hardcoded.
func ResolveWorkItemDefaults(item, defaults WorkItemDefaultsInput) ResolvedWorkItemDefaults {
	return ResolvedWorkItemDefaults{
		DurationMode: domain.CoalesceStr(item.DurationMode, defaults.DurationMode, "estimate"),
		PlannedMin: domain.IntFromPtrWithDefault(
			0,
			item.PlannedMin,
		),
		EstimateConfidence: domain.Float64FromPtrWithDefault(
			0.5,
			item.EstimateConfidence,
		),
		MinSessionMin: domain.IntFromPtrWithDefault(
			15,
			minSession(item.SessionPolicy),
			minSession(defaults.SessionPolicy),
		),
		MaxSessionMin: domain.IntFromPtrWithDefault(
			60,
			maxSession(item.SessionPolicy),
			maxSession(defaults.SessionPolicy),
		),
		DefaultSessionMin: domain.IntFromPtrWithDefault(
			30,
			defaultSession(item.SessionPolicy),
			defaultSession(defaults.SessionPolicy),
		),
		Splittable: domain.BoolFromPtrWithDefault(
			true,
			splittable(item.SessionPolicy),
			splittable(defaults.SessionPolicy),
		),
	}
}

// DependencyCandidate represents a work item candidate for linear dependency inference.
type DependencyCandidate struct {
	ID        string
	NodeOrder int
	NodePos   int
	ItemPos   int
}

// InferLinearDependencies infers predecessor->successor links from sorted candidates.
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

// AssignSequentialIDs applies the import/template sequence policy.
func AssignSequentialIDs(nodes []*domain.PlanNode, workItems []*domain.WorkItem) {
	wiByNode := make(map[string][]*domain.WorkItem, len(nodes))
	for _, wi := range workItems {
		wiByNode[wi.NodeID] = append(wiByNode[wi.NodeID], wi)
	}

	seq := 1
	for _, node := range nodes {
		node.Seq = seq
		seq++
		for _, wi := range wiByNode[node.ID] {
			wi.Seq = seq
			seq++
		}
	}
}

// ParseRequiredDate parses a required YYYY-MM-DD date with field-aware errors.
func ParseRequiredDate(value, field string) (time.Time, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: invalid date format %q (expected YYYY-MM-DD)", field, value)
	}
	return t, nil
}

// ParseOptionalDate parses an optional YYYY-MM-DD date with field-aware errors.
func ParseOptionalDate(value *string, field string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	t, err := ParseRequiredDate(*value, field)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func minSession(policy SessionPolicy) *int {
	if policy == nil {
		return nil
	}
	return policy.MinSessionValue()
}

func maxSession(policy SessionPolicy) *int {
	if policy == nil {
		return nil
	}
	return policy.MaxSessionValue()
}

func defaultSession(policy SessionPolicy) *int {
	if policy == nil {
		return nil
	}
	return policy.DefaultSessionValue()
}

func splittable(policy SessionPolicy) *bool {
	if policy == nil {
		return nil
	}
	return policy.SplittableValue()
}

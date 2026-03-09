package generation

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SessionPolicy test helper ---

type testPolicy struct {
	min        *int
	max        *int
	def        *int
	splittable *bool
}

func (p *testPolicy) MinSessionValue() *int    { return p.min }
func (p *testPolicy) MaxSessionValue() *int    { return p.max }
func (p *testPolicy) DefaultSessionValue() *int { return p.def }
func (p *testPolicy) SplittableValue() *bool   { return p.splittable }

func intPtr(v int) *int         { return &v }
func boolPtr(v bool) *bool      { return &v }

// --- ResolveWorkItemDefaults ---

func TestResolveWorkItemDefaults_AllHardcoded(t *testing.T) {
	// No item or defaults → all hardcoded fallbacks
	result := ResolveWorkItemDefaults(WorkItemDefaultsInput{}, WorkItemDefaultsInput{})
	assert.Equal(t, "estimate", result.DurationMode)
	assert.Equal(t, 15, result.MinSessionMin)
	assert.Equal(t, 60, result.MaxSessionMin)
	assert.Equal(t, 30, result.DefaultSessionMin)
	assert.True(t, result.Splittable)
	assert.Equal(t, 0.5, result.EstimateConfidence)
}

func TestResolveWorkItemDefaults_ItemOverridesDefaults(t *testing.T) {
	itemPolicy := &testPolicy{
		min:        intPtr(20),
		max:        intPtr(90),
		def:        intPtr(45),
		splittable: boolPtr(false),
	}
	defaultPolicy := &testPolicy{
		min:        intPtr(10),
		max:        intPtr(50),
		def:        intPtr(30),
		splittable: boolPtr(true),
	}
	item := WorkItemDefaultsInput{
		DurationMode:  "units",
		SessionPolicy: itemPolicy,
		PlannedMin:    intPtr(120),
	}
	defaults := WorkItemDefaultsInput{
		DurationMode:  "estimate",
		SessionPolicy: defaultPolicy,
	}
	result := ResolveWorkItemDefaults(item, defaults)
	assert.Equal(t, "units", result.DurationMode)
	assert.Equal(t, 120, result.PlannedMin)
	assert.Equal(t, 20, result.MinSessionMin, "item policy overrides default")
	assert.Equal(t, 90, result.MaxSessionMin)
	assert.Equal(t, 45, result.DefaultSessionMin)
	assert.False(t, result.Splittable)
}

func TestResolveWorkItemDefaults_DefaultsFillItemGaps(t *testing.T) {
	// Item has no session policy → falls through to defaults policy
	defaultPolicy := &testPolicy{
		min:        intPtr(12),
		max:        intPtr(75),
		def:        intPtr(40),
		splittable: boolPtr(false),
	}
	item := WorkItemDefaultsInput{DurationMode: "estimate"}
	defaults := WorkItemDefaultsInput{SessionPolicy: defaultPolicy}

	result := ResolveWorkItemDefaults(item, defaults)
	assert.Equal(t, 12, result.MinSessionMin)
	assert.Equal(t, 75, result.MaxSessionMin)
	assert.Equal(t, 40, result.DefaultSessionMin)
	assert.False(t, result.Splittable)
}

func TestResolveWorkItemDefaults_DurationModeFallsToDefaults(t *testing.T) {
	item := WorkItemDefaultsInput{}                                 // empty DurationMode
	defaults := WorkItemDefaultsInput{DurationMode: "units"}
	result := ResolveWorkItemDefaults(item, defaults)
	assert.Equal(t, "units", result.DurationMode)
}

// --- InferLinearDependencies ---

func TestInferLinearDependencies_Empty(t *testing.T) {
	assert.Nil(t, InferLinearDependencies(nil))
	assert.Nil(t, InferLinearDependencies([]DependencyCandidate{}))
}

func TestInferLinearDependencies_SingleItem(t *testing.T) {
	result := InferLinearDependencies([]DependencyCandidate{
		{ID: "a", NodeOrder: 1, NodePos: 1, ItemPos: 1},
	})
	assert.Nil(t, result, "single item cannot form a predecessor->successor pair")
}

func TestInferLinearDependencies_TwoItems(t *testing.T) {
	result := InferLinearDependencies([]DependencyCandidate{
		{ID: "b", NodeOrder: 1, NodePos: 1, ItemPos: 2},
		{ID: "a", NodeOrder: 1, NodePos: 1, ItemPos: 1},
	})
	require.Len(t, result, 1)
	assert.Equal(t, "a", result[0].PredecessorWorkItemID)
	assert.Equal(t, "b", result[0].SuccessorWorkItemID)
}

func TestInferLinearDependencies_SortsByNodeOrderThenItemPos(t *testing.T) {
	candidates := []DependencyCandidate{
		{ID: "c", NodeOrder: 2, NodePos: 1, ItemPos: 1},
		{ID: "a", NodeOrder: 1, NodePos: 1, ItemPos: 1},
		{ID: "b", NodeOrder: 1, NodePos: 1, ItemPos: 2},
	}
	result := InferLinearDependencies(candidates)
	require.Len(t, result, 2)
	assert.Equal(t, "a", result[0].PredecessorWorkItemID)
	assert.Equal(t, "b", result[0].SuccessorWorkItemID)
	assert.Equal(t, "b", result[1].PredecessorWorkItemID)
	assert.Equal(t, "c", result[1].SuccessorWorkItemID)
}

func TestInferLinearDependencies_SkipsEmptyIDs(t *testing.T) {
	candidates := []DependencyCandidate{
		{ID: "a", NodeOrder: 1, ItemPos: 1},
		{ID: "", NodeOrder: 1, ItemPos: 2}, // empty ID
		{ID: "b", NodeOrder: 1, ItemPos: 3},
	}
	result := InferLinearDependencies(candidates)
	// a→"" is skipped, ""→b is skipped; only a→"" and ""→b are adjacent
	for _, dep := range result {
		assert.NotEmpty(t, dep.PredecessorWorkItemID)
		assert.NotEmpty(t, dep.SuccessorWorkItemID)
	}
}

// --- AssignSequentialIDs ---

func TestAssignSequentialIDs_Empty(t *testing.T) {
	// Should not panic
	AssignSequentialIDs(nil, nil)
}

func TestAssignSequentialIDs_SingleNodeAndItem(t *testing.T) {
	node := &domain.PlanNode{ID: "n1"}
	wi := &domain.WorkItem{ID: "wi1", NodeID: "n1"}
	AssignSequentialIDs([]*domain.PlanNode{node}, []*domain.WorkItem{wi})
	assert.Equal(t, 1, node.Seq)
	assert.Equal(t, 2, wi.Seq)
}

func TestAssignSequentialIDs_MultipleNodesAndItems(t *testing.T) {
	n1 := &domain.PlanNode{ID: "n1"}
	n2 := &domain.PlanNode{ID: "n2"}
	wi1 := &domain.WorkItem{ID: "wi1", NodeID: "n1"}
	wi2 := &domain.WorkItem{ID: "wi2", NodeID: "n1"}
	wi3 := &domain.WorkItem{ID: "wi3", NodeID: "n2"}

	AssignSequentialIDs(
		[]*domain.PlanNode{n1, n2},
		[]*domain.WorkItem{wi1, wi2, wi3},
	)

	assert.Equal(t, 1, n1.Seq)
	assert.Equal(t, 2, wi1.Seq)
	assert.Equal(t, 3, wi2.Seq)
	assert.Equal(t, 4, n2.Seq)
	assert.Equal(t, 5, wi3.Seq)
}

func TestAssignSequentialIDs_NodeWithNoItems(t *testing.T) {
	n1 := &domain.PlanNode{ID: "n1"}
	n2 := &domain.PlanNode{ID: "n2"}
	wi := &domain.WorkItem{ID: "wi1", NodeID: "n2"}

	AssignSequentialIDs([]*domain.PlanNode{n1, n2}, []*domain.WorkItem{wi})

	assert.Equal(t, 1, n1.Seq)
	assert.Equal(t, 2, n2.Seq)
	assert.Equal(t, 3, wi.Seq)
}

// --- ParseRequiredDate ---

func TestParseRequiredDate_Valid(t *testing.T) {
	result, err := ParseRequiredDate("2025-03-15", "start_date")
	require.NoError(t, err)
	assert.Equal(t, 2025, result.Year())
	assert.Equal(t, 3, int(result.Month()))
	assert.Equal(t, 15, result.Day())
}

func TestParseRequiredDate_Invalid(t *testing.T) {
	_, err := ParseRequiredDate("15-03-2025", "start_date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start_date")
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestParseRequiredDate_Empty(t *testing.T) {
	_, err := ParseRequiredDate("", "target_date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_date")
}

// --- ParseOptionalDate ---

func TestParseOptionalDate_Nil(t *testing.T) {
	result, err := ParseOptionalDate(nil, "due_date")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseOptionalDate_EmptyString(t *testing.T) {
	empty := ""
	result, err := ParseOptionalDate(&empty, "due_date")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseOptionalDate_Valid(t *testing.T) {
	v := "2026-01-01"
	result, err := ParseOptionalDate(&v, "due_date")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2026, result.Year())
}

func TestParseOptionalDate_Invalid(t *testing.T) {
	v := "not-a-date"
	_, err := ParseOptionalDate(&v, "due_date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "due_date")
}

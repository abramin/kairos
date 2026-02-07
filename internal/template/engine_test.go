package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_BasicTemplate(t *testing.T) {
	schema := &TemplateSchema{
		ID:      "test_template",
		Name:    "Test",
		Domain:  "test",
		Version: "1.0",
		Variables: []VariableConfig{
			{Key: "weeks", Type: "int", Default: json.RawMessage("3")},
		},
		Nodes: []NodeConfig{
			{
				ID:     "week_{i}",
				Repeat: mustMarshal(RepeatConfig{Var: "i", From: 1, ToVar: "weeks"}),
				Title:  "Week {i}",
				Kind:   "week",
			},
		},
		WorkItems: []WorkItemConfig{
			{
				ID:         "w{i}_s1",
				Repeat:     mustMarshal(RepeatConfig{Var: "i", From: 1, ToVar: "weeks"}),
				NodeID:     "week_{i}",
				Title:      "Session 1",
				Type:       "task",
				PlannedMin: intPtr(45),
				SessionPolicy: &SessionPolicyConfig{
					MinSessionMin:     intPtr(30),
					MaxSessionMin:     intPtr(60),
					DefaultSessionMin: intPtr(45),
				},
			},
		},
	}

	result, err := Execute(schema, "Test Project", "2025-01-01", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "Test Project", result.Project.Name)
	assert.Equal(t, "test", result.Project.Domain)
	assert.Len(t, result.Nodes, 3, "should create 3 week nodes")
	assert.Len(t, result.WorkItems, 3, "should create 3 work items (1 per week)")

	// Verify node titles
	titles := make([]string, len(result.Nodes))
	for i, n := range result.Nodes {
		titles[i] = n.Title
	}
	assert.Contains(t, titles, "Week 1")
	assert.Contains(t, titles, "Week 2")
	assert.Contains(t, titles, "Week 3")
}

func TestExecute_VariableOverride(t *testing.T) {
	schema := &TemplateSchema{
		ID: "test", Name: "Test", Domain: "test", Version: "1.0",
		Variables: []VariableConfig{
			{Key: "weeks", Type: "int", Default: json.RawMessage("19")},
		},
		Nodes: []NodeConfig{
			{
				ID:     "week_{i}",
				Repeat: mustMarshal(RepeatConfig{Var: "i", From: 1, ToVar: "weeks"}),
				Title:  "Week {i}",
				Kind:   "week",
			},
		},
		WorkItems: []WorkItemConfig{},
	}

	result, err := Execute(schema, "Test", "2025-01-01", nil, map[string]string{"weeks": "5"})
	require.NoError(t, err)
	assert.Len(t, result.Nodes, 5, "override should produce 5 weeks")
}

func TestExecute_NestedRepeat(t *testing.T) {
	schema := &TemplateSchema{
		ID: "test", Name: "Test", Domain: "test", Version: "1.0",
		Variables: []VariableConfig{
			{Key: "weeks", Type: "int", Default: json.RawMessage("2")},
			{Key: "sessions", Type: "int", Default: json.RawMessage("3")},
		},
		Nodes: []NodeConfig{
			{
				ID:     "week_{i}",
				Repeat: mustMarshal(RepeatConfig{Var: "i", From: 1, ToVar: "weeks"}),
				Title:  "Week {i}",
				Kind:   "week",
			},
		},
		WorkItems: []WorkItemConfig{
			{
				ID: "w{i}_s{j}",
				Repeat: mustMarshal([]RepeatConfig{
					{Var: "i", From: 1, ToVar: "weeks"},
					{Var: "j", From: 1, ToVar: "sessions"},
				}),
				NodeID:     "week_{i}",
				Title:      "Session {j}",
				Type:       "task",
				PlannedMin: intPtr(45),
			},
		},
	}

	result, err := Execute(schema, "Test", "2025-01-01", nil, nil)
	require.NoError(t, err)
	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.WorkItems, 6, "2 weeks x 3 sessions = 6 work items")
}

func TestExecute_Constraints(t *testing.T) {
	schema := &TemplateSchema{
		ID: "test", Name: "Test", Domain: "test", Version: "1.0",
		Variables: []VariableConfig{
			{Key: "weeks", Type: "int", Default: json.RawMessage("2")},
		},
		Nodes: []NodeConfig{
			{
				ID:     "week_{i}",
				Repeat: mustMarshal(RepeatConfig{Var: "i", From: 1, ToVar: "weeks"}),
				Title:  "Week {i}",
				Kind:   "week",
				Order:  "{i}",
				Constraints: &ConstraintsConfig{
					NotBeforeOffsetDays: "{(i-1)*7}",
					NotAfterOffsetDays:  "{i*7-1}",
				},
			},
		},
		WorkItems: []WorkItemConfig{},
	}

	result, err := Execute(schema, "Test", "2025-01-06", nil, nil) // Monday
	require.NoError(t, err)
	require.Len(t, result.Nodes, 2)

	// Week 1: not_before = day 0, not_after = day 6
	assert.NotNil(t, result.Nodes[0].NotBefore)
	assert.Equal(t, "2025-01-06", result.Nodes[0].NotBefore.Format("2006-01-02"))
	assert.NotNil(t, result.Nodes[0].NotAfter)
	assert.Equal(t, "2025-01-12", result.Nodes[0].NotAfter.Format("2006-01-02"))

	// Week 2: not_before = day 7, not_after = day 13
	assert.NotNil(t, result.Nodes[1].NotBefore)
	assert.Equal(t, "2025-01-13", result.Nodes[1].NotBefore.Format("2006-01-02"))
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func intPtr(n int) *int {
	return &n
}

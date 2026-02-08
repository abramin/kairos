package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSchema_ValidSchema(t *testing.T) {
	schema := &TemplateSchema{
		ID:     "test_template",
		Name:   "Test Template",
		Domain: "test",
		Nodes: []NodeConfig{
			{ID: "node_1", Title: "Node 1", Kind: "week"},
		},
		WorkItems: []WorkItemConfig{
			{ID: "wi_1", NodeID: "node_1", Title: "Item 1", Type: "task"},
		},
	}

	errs := ValidateSchema(schema)
	assert.Empty(t, errs, "valid schema should have no errors")
}

func TestValidateSchema_MissingRequiredFields(t *testing.T) {
	schema := &TemplateSchema{
		// All required fields missing
		Nodes:     []NodeConfig{},
		WorkItems: []WorkItemConfig{},
	}

	errs := ValidateSchema(schema)
	assert.NotEmpty(t, errs)

	errMsgs := make([]string, len(errs))
	for i, e := range errs {
		errMsgs[i] = e.Error()
	}

	assert.Contains(t, errMsgs, "template id is required")
	assert.Contains(t, errMsgs, "template name is required")
	assert.Contains(t, errMsgs, "template domain is required")
	assert.Contains(t, errMsgs, "at least one node is required")
	assert.Contains(t, errMsgs, "at least one work item is required")
}

func TestValidateSchema_DuplicateNodeID(t *testing.T) {
	schema := &TemplateSchema{
		ID:     "test",
		Name:   "Test",
		Domain: "test",
		Nodes: []NodeConfig{
			{ID: "dupe", Title: "Node 1", Kind: "week"},
			{ID: "dupe", Title: "Node 2", Kind: "week"},
		},
		WorkItems: []WorkItemConfig{
			{ID: "wi_1", NodeID: "dupe", Title: "Item 1", Type: "task"},
		},
	}

	errs := ValidateSchema(schema)
	assert.NotEmpty(t, errs)

	hasDupeErr := false
	for _, e := range errs {
		if e.Error() == `node[1]: duplicate id "dupe"` {
			hasDupeErr = true
		}
	}
	assert.True(t, hasDupeErr, "should detect duplicate node ID")
}

func TestValidateSchema_MissingNodeFields(t *testing.T) {
	schema := &TemplateSchema{
		ID:     "test",
		Name:   "Test",
		Domain: "test",
		Nodes: []NodeConfig{
			{}, // All fields missing
		},
		WorkItems: []WorkItemConfig{
			{ID: "wi_1", NodeID: "n1", Title: "Item", Type: "task"},
		},
	}

	errs := ValidateSchema(schema)
	assert.NotEmpty(t, errs)

	errMsgs := make([]string, len(errs))
	for i, e := range errs {
		errMsgs[i] = e.Error()
	}

	assert.Contains(t, errMsgs, "node[0]: id is required")
	assert.Contains(t, errMsgs, "node[0]: title is required")
	assert.Contains(t, errMsgs, "node[0]: kind is required")
}

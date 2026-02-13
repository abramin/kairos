package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyEditWorkItem_ReturnsOutputMessageAndUpdatesItem(t *testing.T) {
	app := testApp(t)
	ctx := context.Background()
	_, wiID := seedProjectWithWork(t, app)

	fields := &editWorkItemFields{
		title:      "Updated Reading",
		desc:       "new description",
		plannedMin: "90",
		itemType:   "review",
		dueDate:    "2026-03-15",
		notBefore:  "2026-03-01",
		minSession: "20",
		maxSession: "80",
	}

	msg := applyEditWorkItem(app, wiID, fields)
	out, ok := msg.(cmdOutputMsg)
	require.True(t, ok, "expected cmdOutputMsg, got %T", msg)
	assert.Contains(t, out.output, "Updated:")

	updated, err := app.WorkItems.GetByID(ctx, wiID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Reading", updated.Title)
	assert.Equal(t, "new description", updated.Description)
	assert.Equal(t, 90, updated.PlannedMin)
	assert.Equal(t, "review", updated.Type)
	assert.Equal(t, 20, updated.MinSessionMin)
	assert.Equal(t, 80, updated.MaxSessionMin)
	if assert.NotNil(t, updated.DueDate) {
		assert.Equal(t, "2026-03-15", updated.DueDate.Format("2006-01-02"))
	}
	if assert.NotNil(t, updated.NotBefore) {
		assert.Equal(t, "2026-03-01", updated.NotBefore.Format("2006-01-02"))
	}
}

func TestApplyEditWorkItem_ErrorReturnsOutputMessage(t *testing.T) {
	app := testApp(t)
	fields := &editWorkItemFields{
		title:      "Does Not Matter",
		plannedMin: "60",
		itemType:   "task",
	}

	msg := applyEditWorkItem(app, "missing-id", fields)
	out, ok := msg.(cmdOutputMsg)
	require.True(t, ok, "expected cmdOutputMsg, got %T", msg)
	assert.Contains(t, out.output, "Error:")
}


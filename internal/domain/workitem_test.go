package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

func TestIsTerminal(t *testing.T) {
	cases := []struct {
		status   WorkItemStatus
		terminal bool
	}{
		{WorkItemTodo, false},
		{WorkItemInProgress, false},
		{WorkItemDone, true},
		{WorkItemSkipped, true},
		{WorkItemArchived, true},
	}
	for _, tc := range cases {
		w := &WorkItem{Status: tc.status}
		assert.Equal(t, tc.terminal, w.IsTerminal(), "status=%s", tc.status)
	}
}

func TestMarkDone_FromTodo(t *testing.T) {
	w := &WorkItem{Status: WorkItemTodo}
	require.NoError(t, w.MarkDone(testNow))
	assert.Equal(t, WorkItemDone, w.Status)
	require.NotNil(t, w.CompletedAt)
	assert.Equal(t, testNow, *w.CompletedAt)
	assert.Equal(t, testNow, w.UpdatedAt)
}

func TestMarkDone_FromInProgress(t *testing.T) {
	w := &WorkItem{Status: WorkItemInProgress}
	require.NoError(t, w.MarkDone(testNow))
	assert.Equal(t, WorkItemDone, w.Status)
	require.NotNil(t, w.CompletedAt)
	assert.Equal(t, testNow, *w.CompletedAt)
}

func TestMarkDone_AlreadyDone(t *testing.T) {
	earlier := testNow.Add(-time.Hour)
	w := &WorkItem{Status: WorkItemDone, CompletedAt: &earlier}
	require.NoError(t, w.MarkDone(testNow))
	assert.Equal(t, WorkItemDone, w.Status)
	assert.Equal(t, earlier, *w.CompletedAt, "should not overwrite existing CompletedAt")
}

func TestMarkDone_FromArchived(t *testing.T) {
	w := &WorkItem{Status: WorkItemArchived}
	err := w.MarkDone(testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archived")
	assert.Equal(t, WorkItemArchived, w.Status, "status should not change")
}

func TestMarkInProgress_FromTodo(t *testing.T) {
	w := &WorkItem{Status: WorkItemTodo}
	require.NoError(t, w.MarkInProgress(testNow))
	assert.Equal(t, WorkItemInProgress, w.Status)
	assert.Equal(t, testNow, w.UpdatedAt)
}

func TestMarkInProgress_AlreadyInProgress(t *testing.T) {
	w := &WorkItem{Status: WorkItemInProgress}
	require.NoError(t, w.MarkInProgress(testNow))
	assert.Equal(t, WorkItemInProgress, w.Status)
}

func TestMarkInProgress_FromDone(t *testing.T) {
	w := &WorkItem{Status: WorkItemDone}
	err := w.MarkInProgress(testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "done")
}

func TestMarkInProgress_FromArchived(t *testing.T) {
	w := &WorkItem{Status: WorkItemArchived}
	err := w.MarkInProgress(testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archived")
}

func TestReopen_FromDone(t *testing.T) {
	completed := testNow.Add(-time.Hour)
	w := &WorkItem{Status: WorkItemDone, CompletedAt: &completed}
	require.NoError(t, w.Reopen(testNow))
	assert.Equal(t, WorkItemTodo, w.Status)
	assert.Nil(t, w.CompletedAt)
	assert.Equal(t, testNow, w.UpdatedAt)
}

func TestReopen_FromTodo(t *testing.T) {
	w := &WorkItem{Status: WorkItemTodo}
	err := w.Reopen(testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "todo")
}

func TestApplySession_AccumulatesMinutesAndUnits(t *testing.T) {
	w := &WorkItem{Status: WorkItemInProgress, LoggedMin: 30, UnitsDone: 2}
	require.NoError(t, w.ApplySession(15, 1, testNow))
	assert.Equal(t, 45, w.LoggedMin)
	assert.Equal(t, 3, w.UnitsDone)
	assert.Equal(t, WorkItemInProgress, w.Status)
	assert.Equal(t, testNow, w.UpdatedAt)
}

func TestApplySession_AutoTransitionsTodoToInProgress(t *testing.T) {
	w := &WorkItem{Status: WorkItemTodo}
	require.NoError(t, w.ApplySession(20, 0, testNow))
	assert.Equal(t, WorkItemInProgress, w.Status)
	assert.Equal(t, 20, w.LoggedMin)
}

func TestApplySession_KeepsInProgressIfAlready(t *testing.T) {
	w := &WorkItem{Status: WorkItemInProgress}
	require.NoError(t, w.ApplySession(10, 0, testNow))
	assert.Equal(t, WorkItemInProgress, w.Status)
}

func TestApplySession_ErrorOnArchived(t *testing.T) {
	w := &WorkItem{Status: WorkItemArchived}
	err := w.ApplySession(10, 0, testNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archived")
}

func TestEligibleForReestimate(t *testing.T) {
	w := &WorkItem{
		Status:       WorkItemInProgress,
		DurationMode: DurationEstimate,
		UnitsTotal:   10,
		UnitsDone:    3,
	}
	assert.True(t, w.EligibleForReestimate())
}

func TestEligibleForReestimate_DoneItem(t *testing.T) {
	w := &WorkItem{
		Status:       WorkItemDone,
		DurationMode: DurationEstimate,
		UnitsTotal:   10,
		UnitsDone:    10,
	}
	assert.False(t, w.EligibleForReestimate())
}

func TestEligibleForReestimate_NoUnits(t *testing.T) {
	w := &WorkItem{
		Status:       WorkItemInProgress,
		DurationMode: DurationEstimate,
		UnitsTotal:   0,
		UnitsDone:    0,
	}
	assert.False(t, w.EligibleForReestimate())
}

func TestEligibleForReestimate_FixedDuration(t *testing.T) {
	w := &WorkItem{
		Status:       WorkItemInProgress,
		DurationMode: DurationFixed,
		UnitsTotal:   10,
		UnitsDone:    3,
	}
	assert.False(t, w.EligibleForReestimate())
}

func TestApplyReestimate_ChangedValue(t *testing.T) {
	w := &WorkItem{PlannedMin: 60}
	changed := w.ApplyReestimate(75, testNow)
	assert.True(t, changed)
	assert.Equal(t, 75, w.PlannedMin)
	assert.Equal(t, testNow, w.UpdatedAt)
}

func TestApplyReestimate_SameValue(t *testing.T) {
	w := &WorkItem{PlannedMin: 60}
	changed := w.ApplyReestimate(60, testNow)
	assert.False(t, changed)
	assert.Equal(t, 60, w.PlannedMin)
}

func TestEffectiveLoggedMin_InProgress(t *testing.T) {
	w := &WorkItem{Status: WorkItemInProgress, PlannedMin: 60, LoggedMin: 30}
	assert.Equal(t, 30, w.EffectiveLoggedMin())
}

func TestEffectiveLoggedMin_DoneWithLessLogged(t *testing.T) {
	w := &WorkItem{Status: WorkItemDone, PlannedMin: 60, LoggedMin: 40}
	assert.Equal(t, 60, w.EffectiveLoggedMin(), "done items count as at least planned")
}

func TestEffectiveLoggedMin_DoneWithMoreLogged(t *testing.T) {
	w := &WorkItem{Status: WorkItemDone, PlannedMin: 60, LoggedMin: 90}
	assert.Equal(t, 90, w.EffectiveLoggedMin(), "should return actual logged when exceeding planned")
}

func TestEffectiveLoggedMin_SkippedWithLessLogged(t *testing.T) {
	w := &WorkItem{Status: WorkItemSkipped, PlannedMin: 60, LoggedMin: 10}
	assert.Equal(t, 60, w.EffectiveLoggedMin(), "skipped items count as at least planned")
}

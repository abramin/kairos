package service

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAggregateProjectMetrics_DoneItemCountsFullPlanned(t *testing.T) {
	now := time.Now().UTC()
	future := now.AddDate(0, 1, 0)
	proj := &domain.Project{
		ID:         "proj-1",
		StartDate:  now.AddDate(0, -1, 0),
		TargetDate: &future,
	}

	items := []*domain.WorkItem{
		{
			ID:         "wi-1",
			Status:     domain.WorkItemDone,
			PlannedMin: 100,
			LoggedMin:  50, // only half logged, but marked done
		},
		{
			ID:         "wi-2",
			Status:     domain.WorkItemTodo,
			PlannedMin: 100,
			LoggedMin:  0,
		},
	}

	m := aggregateProjectMetrics(items, proj, now)

	// Done item should contribute its full PlannedMin (100), not just LoggedMin (50).
	assert.Equal(t, 100, m.LoggedMin, "done item should contribute PlannedMin as effective logged")
	assert.Equal(t, 200, m.PlannedMin)
	assert.Equal(t, 1, m.DoneCount)
	assert.Equal(t, 2, m.TotalCount)
	assert.Equal(t, 100, m.DonePlannedMin)
}

func TestAggregateProjectMetrics_DoneItemWithExcessLogged(t *testing.T) {
	now := time.Now().UTC()
	proj := &domain.Project{ID: "proj-1", StartDate: now.AddDate(0, -1, 0)}

	items := []*domain.WorkItem{
		{
			ID:         "wi-1",
			Status:     domain.WorkItemDone,
			PlannedMin: 60,
			LoggedMin:  90, // logged more than planned
		},
	}

	m := aggregateProjectMetrics(items, proj, now)

	// When logged exceeds planned, keep the actual logged value.
	assert.Equal(t, 90, m.LoggedMin, "should keep actual logged when it exceeds planned")
}

func TestAggregateProjectMetrics_SkippedItemCountsFullPlanned(t *testing.T) {
	now := time.Now().UTC()
	proj := &domain.Project{ID: "proj-1", StartDate: now.AddDate(0, -1, 0)}

	items := []*domain.WorkItem{
		{
			ID:         "wi-1",
			Status:     domain.WorkItemSkipped,
			PlannedMin: 80,
			LoggedMin:  10,
		},
	}

	m := aggregateProjectMetrics(items, proj, now)

	assert.Equal(t, 80, m.LoggedMin, "skipped item should contribute PlannedMin as effective logged")
}

func TestAggregateProjectMetrics_InProgressItemUsesActualLogged(t *testing.T) {
	now := time.Now().UTC()
	proj := &domain.Project{ID: "proj-1", StartDate: now.AddDate(0, -1, 0)}

	items := []*domain.WorkItem{
		{
			ID:         "wi-1",
			Status:     domain.WorkItemInProgress,
			PlannedMin: 100,
			LoggedMin:  30,
		},
	}

	m := aggregateProjectMetrics(items, proj, now)

	assert.Equal(t, 30, m.LoggedMin, "in-progress item should use actual logged minutes")
}

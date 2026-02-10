package formatter

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatProjectList_UsesShortIDWhenPresent(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "12345678-aaaa-bbbb-cccc-1234567890ab",
			ShortID:   "PSY01",
			Name:      "Psychology OU - Module 1",
			Domain:    "Education",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "PSY01")
	assert.NotContains(t, out, "12345678")
}

func TestFormatProjectList_FallsBackToUUIDPrefixWhenShortIDMissing(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "abcdef12-3456-7890-abcd-ef1234567890",
			ShortID:   "",
			Name:      "Psychology OU - Module 1",
			Domain:    "Education",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "abcdef12")
}

func TestFormatProjectList_UsesPlaceholderWhenIDAndShortIDMissing(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "",
			ShortID:   "",
			Name:      "Untitled",
			Domain:    "General",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "--")
}

// --- RenderTree tests ---

func TestRenderTree_DoneItemHasCheckmark(t *testing.T) {
	items := []TreeItem{
		{Title: "Read Chapter 1", Level: 1, Status: "done", IsLast: true},
	}
	out := RenderTree(items)
	assert.Contains(t, out, "✔")
	assert.Contains(t, out, "Read Chapter 1")
}

func TestRenderTree_InProgressItemHasTriangle(t *testing.T) {
	items := []TreeItem{
		{Title: "Practice Problems", Level: 1, Status: "in_progress", IsLast: true},
	}
	out := RenderTree(items)
	assert.Contains(t, out, "▶")
	assert.Contains(t, out, "Practice Problems")
}

func TestRenderTree_TodoItemHasNoPrefix(t *testing.T) {
	items := []TreeItem{
		{Title: "Final Exam", Level: 1, Status: "todo", IsLast: true},
	}
	out := RenderTree(items)
	assert.NotContains(t, out, "✔")
	assert.NotContains(t, out, "▶")
	assert.Contains(t, out, "Final Exam")
}

func TestRenderTree_BadgeAlignment(t *testing.T) {
	items := []TreeItem{
		{Title: "Short", Level: 1, Detail: "1h"},
		{Title: "A Much Longer Title Here", Level: 1, Detail: "2h", IsLast: true},
	}
	out := RenderTree(items)
	assert.Contains(t, out, "[ 1h ]")
	assert.Contains(t, out, "[ 2h ]")
}

func TestBuildProjectTree_CollapseSingleWorkItem(t *testing.T) {
	nodes := []*domain.PlanNode{
		{ID: "n1", Title: "Homer – The Odyssey", Seq: 1, OrderIndex: 0},
	}
	workItems := map[string][]*domain.WorkItem{
		"n1": {{Title: "Read The Odyssey", Seq: 2, Status: domain.WorkItemDone, PlannedMin: 720}},
	}

	items := buildProjectTree(nodes, nil, workItems, 0)

	assert.Len(t, items, 1, "should collapse node+work item into one item")
	assert.Equal(t, "Homer – The Odyssey", items[0].Title, "should use node title")
	assert.Equal(t, 1, items[0].Seq, "should use node seq")
	assert.Equal(t, string(domain.WorkItemDone), items[0].Status, "should inherit work item status")
	assert.Contains(t, items[0].Detail, "12h", "should inherit work item detail")
}

func TestBuildProjectTree_NoCollapseMultipleWorkItems(t *testing.T) {
	nodes := []*domain.PlanNode{
		{ID: "n1", Title: "Week 1", Seq: 1, OrderIndex: 0},
	}
	workItems := map[string][]*domain.WorkItem{
		"n1": {
			{Title: "Reading", Seq: 2, Status: domain.WorkItemDone, PlannedMin: 60},
			{Title: "Exercises", Seq: 3, Status: domain.WorkItemTodo, PlannedMin: 30},
		},
	}

	items := buildProjectTree(nodes, nil, workItems, 0)

	assert.Len(t, items, 3, "should not collapse: 1 node + 2 work items")
	assert.Equal(t, "Week 1", items[0].Title)
	assert.Equal(t, "Reading", items[1].Title)
	assert.Equal(t, "Exercises", items[2].Title)
}

func TestBuildProjectTree_NoCollapseWithChildNodes(t *testing.T) {
	nodes := []*domain.PlanNode{
		{ID: "n1", Title: "Part 1", Seq: 1, OrderIndex: 0},
	}
	childMap := map[string][]*domain.PlanNode{
		"n1": {{ID: "n2", Title: "Chapter 1", Seq: 2, OrderIndex: 0}},
	}
	workItems := map[string][]*domain.WorkItem{
		"n1": {{Title: "Overview", Seq: 3, Status: domain.WorkItemTodo, PlannedMin: 30}},
	}

	items := buildProjectTree(nodes, childMap, workItems, 0)

	assert.True(t, len(items) > 1, "should not collapse when node has child nodes")
	assert.Equal(t, "Part 1", items[0].Title)
}

func TestBuildTreePanel_ShowsProgressBar(t *testing.T) {
	nodes := []*domain.PlanNode{
		{ID: "n1", Title: "Week 1", OrderIndex: 0},
	}
	workItems := map[string][]*domain.WorkItem{
		"n1": {
			{Title: "Task A", Status: domain.WorkItemDone, PlannedMin: 60},
			{Title: "Task B", Status: domain.WorkItemTodo, PlannedMin: 30},
		},
	}
	out := buildTreePanel(nodes, nil, workItems)
	assert.Contains(t, out, "PLAN")
	assert.Contains(t, out, "50%")
}

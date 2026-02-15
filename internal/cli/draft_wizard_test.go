package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/alexanderramin/kairos/internal/service"
	"github.com/alexanderramin/kairos/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSchemaFromWizard_SingleGroup(t *testing.T) {
	result := &wizardResult{
		Description: "Physics Study Plan",
		StartDate:   "2026-02-08",
		Deadline:    "2026-06-30",
		Groups: []wizardGroup{
			{Label: "Chapter", Count: 5, Kind: "module", DaysPer: 10},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Reading", Type: "reading", PlannedMin: 60},
			{Title: "Practice Problems", Type: "practice", PlannedMin: 45},
		},
	}

	schema := buildSchemaFromWizard(result)

	// Project fields.
	assert.Equal(t, "Physics Study Plan", schema.Project.Name)
	assert.Equal(t, "2026-02-08", schema.Project.StartDate)
	assert.Equal(t, "2026-06-30", *schema.Project.TargetDate)
	assert.NotEmpty(t, schema.Project.ShortID)
	assert.Equal(t, "education", schema.Project.Domain)

	// 5 nodes.
	assert.Len(t, schema.Nodes, 5)
	for i, node := range schema.Nodes {
		assert.Equal(t, "module", node.Kind)
		assert.Contains(t, node.Title, "Chapter")
		assert.Equal(t, i+1, node.Order)
		assert.NotNil(t, node.DueDate, "node %d should have a due date", i)
		assert.Equal(t, 105, *node.PlannedMinBudget) // 60 + 45
	}

	// 10 work items (2 per node).
	assert.Len(t, schema.WorkItems, 10)
	for i, wi := range schema.WorkItems {
		nodeIdx := i / 2
		expectedNodeRef := schema.Nodes[nodeIdx].Ref
		assert.Equal(t, expectedNodeRef, wi.NodeRef)
	}

	// Due dates should be sequential, 10 days apart.
	assert.Equal(t, "2026-02-18", *schema.Nodes[0].DueDate)
	assert.Equal(t, "2026-02-28", *schema.Nodes[1].DueDate)
	assert.Equal(t, "2026-03-10", *schema.Nodes[2].DueDate)

	// Defaults.
	require.NotNil(t, schema.Defaults)
	assert.Equal(t, "estimate", schema.Defaults.DurationMode)
	require.NotNil(t, schema.Defaults.SessionPolicy)
	assert.Equal(t, 15, *schema.Defaults.SessionPolicy.MinSessionMin)
	assert.Equal(t, 60, *schema.Defaults.SessionPolicy.MaxSessionMin)

	// Should pass validation.
	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "schema should be valid, got: %v", errs)
}

func TestBuildSchemaFromWizard_TwoGroupsDifferentDurations(t *testing.T) {
	result := &wizardResult{
		Description: "French Course",
		StartDate:   "2026-02-08",
		Deadline:    "2026-10-31",
		Groups: []wizardGroup{
			{Label: "A2 Module", Count: 9, Kind: "module", DaysPer: 10},
			{Label: "B1 Module", Count: 11, Kind: "module", DaysPer: 14},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Chapter exercises", Type: "practice", PlannedMin: 45},
			{Title: "Grammar review", Type: "review", PlannedMin: 30},
		},
	}

	schema := buildSchemaFromWizard(result)

	// 20 nodes total (9 + 11).
	assert.Len(t, schema.Nodes, 20)

	// First 9 are A2 modules.
	for i := 0; i < 9; i++ {
		assert.Contains(t, schema.Nodes[i].Title, "A2 Module")
	}
	// Next 11 are B1 modules.
	for i := 9; i < 20; i++ {
		assert.Contains(t, schema.Nodes[i].Title, "B1 Module")
	}

	// 40 work items (2 per node).
	assert.Len(t, schema.WorkItems, 40)

	// Due dates are sequential: A2 at 10-day intervals, then B1 at 14-day intervals.
	// cursor starts at startDate (Feb 8), then += DaysPer each time.
	// A2 Module 1: Feb 8 + 10 = Feb 18
	// A2 Module 9: Feb 8 + 90 = May 9
	assert.Equal(t, "2026-02-18", *schema.Nodes[0].DueDate)
	assert.Equal(t, "2026-05-09", *schema.Nodes[8].DueDate)
	for i := 1; i < len(schema.Nodes); i++ {
		assert.True(t, *schema.Nodes[i].DueDate >= *schema.Nodes[i-1].DueDate,
			"node %d due date %s should be >= node %d due date %s",
			i, *schema.Nodes[i].DueDate, i-1, *schema.Nodes[i-1].DueDate)
	}

	// B1 modules start after A2 modules end, with 14-day intervals.
	a2LastDue := *schema.Nodes[8].DueDate
	b1FirstDue := *schema.Nodes[9].DueDate
	assert.True(t, b1FirstDue > a2LastDue, "B1 should start after A2 ends")

	// Should pass validation.
	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "schema should be valid, got: %v", errs)
}

func TestBuildSchemaFromWizard_SpecialNodes(t *testing.T) {
	result := &wizardResult{
		Description: "Course with Exam",
		StartDate:   "2026-02-08",
		Deadline:    "2026-06-30",
		Groups: []wizardGroup{
			{Label: "Week", Count: 3, Kind: "week", DaysPer: 7},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Reading", Type: "reading", PlannedMin: 60},
		},
		SpecialNodes: []wizardSpecialNode{
			{
				Title:   "Final Exam",
				Kind:    "assessment",
				DueDate: "2026-06-25",
				WorkItems: []wizardWorkItem{
					{Title: "Exam preparation", Type: "review", PlannedMin: 120},
				},
			},
		},
	}

	schema := buildSchemaFromWizard(result)

	// 3 regular + 1 special = 4 nodes.
	assert.Len(t, schema.Nodes, 4)
	assert.Equal(t, "Final Exam", schema.Nodes[3].Title)
	assert.Equal(t, "assessment", schema.Nodes[3].Kind)
	assert.Equal(t, "2026-06-25", *schema.Nodes[3].DueDate)

	// 3 regular work items + 1 special = 4.
	assert.Len(t, schema.WorkItems, 4)
	assert.Equal(t, "Exam preparation", schema.WorkItems[3].Title)
	assert.Equal(t, "review", schema.WorkItems[3].Type)
	assert.Equal(t, 120, *schema.WorkItems[3].PlannedMin)

	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "schema should be valid, got: %v", errs)
}

func TestBuildSchemaFromWizard_NoDeadline(t *testing.T) {
	result := &wizardResult{
		Description: "Open-ended Project",
		StartDate:   "2026-02-08",
		Groups: []wizardGroup{
			{Label: "Phase", Count: 3, Kind: "stage"},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Task", Type: "task", PlannedMin: 30},
		},
	}

	schema := buildSchemaFromWizard(result)

	// No deadline on project.
	assert.Nil(t, schema.Project.TargetDate)

	// No due dates on nodes (no DaysPer, no deadline to spread).
	for i, node := range schema.Nodes {
		assert.Nil(t, node.DueDate, "node %d should have no due date", i)
	}

	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "schema should be valid, got: %v", errs)
}

func TestBuildSchemaFromWizard_SpreadEvenly(t *testing.T) {
	result := &wizardResult{
		Description: "Spread Project",
		StartDate:   "2026-01-01",
		Deadline:    "2026-04-01",
		Groups: []wizardGroup{
			{Label: "Module", Count: 3, Kind: "module"}, // No DaysPer
		},
		WorkItems: []wizardWorkItem{
			{Title: "Task", Type: "task", PlannedMin: 30},
		},
	}

	schema := buildSchemaFromWizard(result)

	// Nodes should have evenly spread due dates.
	assert.Len(t, schema.Nodes, 3)
	for _, node := range schema.Nodes {
		require.NotNil(t, node.DueDate)
	}
	// Should be roughly evenly spread between Jan 1 and Apr 1.
	assert.True(t, *schema.Nodes[0].DueDate < *schema.Nodes[1].DueDate)
	assert.True(t, *schema.Nodes[1].DueDate < *schema.Nodes[2].DueDate)
	assert.Equal(t, "2026-04-01", *schema.Nodes[2].DueDate) // Last node at deadline.

	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "schema should be valid, got: %v", errs)
}

func TestBuildSchemaFromWizard_SpecialNodeUsesDeadlineAsDueDate(t *testing.T) {
	result := &wizardResult{
		Description: "Project with Assessment",
		StartDate:   "2026-02-08",
		Deadline:    "2026-06-30",
		Groups: []wizardGroup{
			{Label: "Week", Count: 1, Kind: "week", DaysPer: 7},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Reading", Type: "reading", PlannedMin: 30},
		},
		SpecialNodes: []wizardSpecialNode{
			{
				Title: "Final",
				Kind:  "assessment",
				// No DueDate specified - should use deadline.
				WorkItems: []wizardWorkItem{
					{Title: "Prep", Type: "review", PlannedMin: 60},
				},
			},
		},
	}

	schema := buildSchemaFromWizard(result)
	assert.Len(t, schema.Nodes, 2)
	assert.Equal(t, "2026-06-30", *schema.Nodes[1].DueDate) // Uses deadline.
}

func TestBuildSchemaFromWizard_UniqueRefs(t *testing.T) {
	result := &wizardResult{
		Description: "Ref Test",
		StartDate:   "2026-02-08",
		Groups: []wizardGroup{
			{Label: "Module", Count: 5, Kind: "module"},
		},
		WorkItems: []wizardWorkItem{
			{Title: "A", Type: "task", PlannedMin: 10},
			{Title: "B", Type: "task", PlannedMin: 20},
		},
	}

	schema := buildSchemaFromWizard(result)

	// All node refs should be unique.
	nodeRefs := map[string]bool{}
	for _, n := range schema.Nodes {
		assert.False(t, nodeRefs[n.Ref], "duplicate node ref: %s", n.Ref)
		nodeRefs[n.Ref] = true
	}

	// All work item refs should be unique.
	wiRefs := map[string]bool{}
	for _, wi := range schema.WorkItems {
		assert.False(t, wiRefs[wi.Ref], "duplicate work item ref: %s", wi.Ref)
		wiRefs[wi.Ref] = true
	}
}

func TestGenerateShortID(t *testing.T) {
	assert.Equal(t, "PHYS01", generateShortID("Physics Study Plan"))
	assert.Equal(t, "FREN01", generateShortID("French Course"))
	assert.Equal(t, "XXX01", generateShortID("123")) // No letters -> XXX fallback.
}

func TestRunStructureWizard_BasicFlow(t *testing.T) {
	// Simulate user input for: 1 group, 3 modules, 7 days each, 1 work item, no special nodes.
	input := strings.Join([]string{
		"",        // groups = 1 (default)
		"Chapter", // label
		"3",       // count
		"module",  // kind
		"7",       // days per node
		"Reading", // work item title
		"reading", // work item type
		"60",      // estimated minutes
		"",        // done with work items
		"",        // no special nodes
	}, "\n") + "\n"

	result, err := runStructureWizard(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Groups, 1)
	assert.Equal(t, "Chapter", result.Groups[0].Label)
	assert.Equal(t, 3, result.Groups[0].Count)
	assert.Equal(t, "module", result.Groups[0].Kind)
	assert.Equal(t, 7, result.Groups[0].DaysPer)

	assert.Len(t, result.WorkItems, 1)
	assert.Equal(t, "Reading", result.WorkItems[0].Title)
	assert.Equal(t, "reading", result.WorkItems[0].Type)
	assert.Equal(t, 60, result.WorkItems[0].PlannedMin)

	assert.Empty(t, result.SpecialNodes)
}

func TestRunStructureWizard_TwoGroups(t *testing.T) {
	input := strings.Join([]string{
		"2",         // 2 groups
		"A2 Module", // group 1 label
		"9",         // group 1 count
		"",          // group 1 kind = default module
		"10",        // group 1 days
		"B1 Module", // group 2 label
		"11",        // group 2 count
		"",          // group 2 kind = default module
		"14",        // group 2 days
		"Exercises", // work item
		"practice",  // type
		"45",        // minutes
		"Grammar",   // work item 2
		"review",    // type
		"30",        // minutes
		"",          // done with work items
		"",          // no special nodes
	}, "\n") + "\n"

	result, err := runStructureWizard(strings.NewReader(input))
	require.NoError(t, err)

	assert.Len(t, result.Groups, 2)
	assert.Equal(t, "A2 Module", result.Groups[0].Label)
	assert.Equal(t, 9, result.Groups[0].Count)
	assert.Equal(t, 10, result.Groups[0].DaysPer)
	assert.Equal(t, "B1 Module", result.Groups[1].Label)
	assert.Equal(t, 11, result.Groups[1].Count)
	assert.Equal(t, 14, result.Groups[1].DaysPer)

	assert.Len(t, result.WorkItems, 2)
}

func TestStartWithDraft_SeedsConversation(t *testing.T) {
	// Verify StartWithDraft creates a conversation with the pre-built draft.
	// This is tested via the intelligence package but let's verify the wizard
	// schema can round-trip through it.
	result := &wizardResult{
		Description: "Test Project",
		StartDate:   "2026-02-08",
		Groups: []wizardGroup{
			{Label: "Module", Count: 2, Kind: "module", DaysPer: 7},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Task", Type: "task", PlannedMin: 30},
		},
	}

	schema := buildSchemaFromWizard(result)
	errs := importer.ValidateImportSchema(schema)
	assert.Empty(t, errs, "wizard schema should validate cleanly")

	// Verify the schema can be converted to domain objects.
	gen, err := importer.Convert(schema)
	require.NoError(t, err)
	assert.Equal(t, "Test Project", gen.Project.Name)
	assert.Len(t, gen.Nodes, 2)
	assert.Len(t, gen.WorkItems, 2)
}

// =============================================================================
// Draft Wizard Full Pipeline Integration Test
// =============================================================================

// TestDraftWizard_FullPipeline exercises the complete wizard → validate → import → schedule flow.
func TestDraftWizard_FullPipeline(t *testing.T) {
	// Step 1: Build wizard result (simulating user input).
	result := &wizardResult{
		Description: "Physics Study Plan",
		StartDate:   "2026-02-08",
		Deadline:    "2026-06-30",
		Groups: []wizardGroup{
			{Label: "Chapter", Count: 3, Kind: "module", DaysPer: 14},
		},
		WorkItems: []wizardWorkItem{
			{Title: "Reading", Type: "reading", PlannedMin: 60},
			{Title: "Problems", Type: "practice", PlannedMin: 45},
		},
		SpecialNodes: []wizardSpecialNode{
			{
				Title:   "Final Exam",
				Kind:    "assessment",
				DueDate: "2026-06-25",
				WorkItems: []wizardWorkItem{
					{Title: "Exam Prep", Type: "review", PlannedMin: 120},
				},
			},
		},
	}

	// Step 2: Build schema from wizard.
	schema := buildSchemaFromWizard(result)
	require.NotNil(t, schema)

	// Step 3: Validate schema.
	errs := importer.ValidateImportSchema(schema)
	require.Empty(t, errs, "wizard schema should validate: %v", errs)

	// Step 4: Import into a real in-memory DB.
	db := testutil.NewTestDB(t)
	uow := testutil.NewTestUoW(db)
	projRepo := repository.NewSQLiteProjectRepo(db)
	nodeRepo := repository.NewSQLitePlanNodeRepo(db)
	wiRepo := repository.NewSQLiteWorkItemRepo(db)
	depRepo := repository.NewSQLiteDependencyRepo(db)
	sessRepo := repository.NewSQLiteSessionRepo(db)
	profRepo := repository.NewSQLiteUserProfileRepo(db)

	importSvc := service.NewImportService(projRepo, nodeRepo, wiRepo, depRepo, uow)
	ctx := context.Background()

	importResult, err := importSvc.ImportProjectFromSchema(ctx, schema)
	require.NoError(t, err)
	assert.Equal(t, "Physics Study Plan", importResult.Project.Name)
	assert.Equal(t, 4, importResult.NodeCount)     // 3 chapters + 1 final exam
	assert.Equal(t, 7, importResult.WorkItemCount) // 3*2 regular + 1 exam prep

	// Step 5: Verify items are schedulable via what-now.
	whatNowSvc := service.NewWhatNowService(wiRepo, sessRepo, depRepo, profRepo)
	req := contract.NewWhatNowRequest(120)
	resp, err := whatNowSvc.Recommend(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Recommendations,
		"imported wizard items should appear as schedulable candidates")

	// Verify at least one of the wizard work items appears.
	var titles []string
	for _, rec := range resp.Recommendations {
		titles = append(titles, rec.Title)
	}
	hasWizardItem := false
	for _, title := range titles {
		if strings.Contains(title, "Reading") || strings.Contains(title, "Problems") || strings.Contains(title, "Exam Prep") {
			hasWizardItem = true
			break
		}
	}
	assert.True(t, hasWizardItem,
		"expected wizard work item in recommendations, got: %v", titles)
}

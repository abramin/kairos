package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrStr(s string) *string     { return &s }
func ptrInt(i int) *int           { return &i }
func ptrFloat(f float64) *float64 { return &f }
func ptrBool(b bool) *bool        { return &b }

func validMinimalSchema() *ImportSchema {
	return &ImportSchema{
		Project: ProjectImport{
			ShortID:   "PHI01",
			Name:      "Test Project",
			Domain:    "test",
			StartDate: "2025-02-01",
		},
		Nodes: []NodeImport{
			{Ref: "n1", Title: "Node 1", Kind: "module"},
		},
		WorkItems: []WorkItemImport{
			{Ref: "w1", NodeRef: "n1", Title: "Task 1", Type: "reading"},
		},
	}
}

func TestValidateImportSchema_ValidMinimal(t *testing.T) {
	errs := ValidateImportSchema(validMinimalSchema())
	assert.Empty(t, errs)
}

func TestValidateImportSchema_ValidFull(t *testing.T) {
	schema := &ImportSchema{
		Project: ProjectImport{
			ShortID:    "MATH01",
			Name:       "Mathematics",
			Domain:     "education",
			StartDate:  "2025-02-01",
			TargetDate: ptrStr("2025-06-01"),
		},
		Defaults: &DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &SessionPolicyImport{
				MinSessionMin:     ptrInt(15),
				MaxSessionMin:     ptrInt(60),
				DefaultSessionMin: ptrInt(30),
				Splittable:        ptrBool(true),
			},
		},
		Nodes: []NodeImport{
			{Ref: "ch1", Title: "Chapter 1", Kind: "module", Order: 0, DueDate: ptrStr("2025-03-01"), PlannedMinBudget: ptrInt(300)},
			{Ref: "ch1_s1", ParentRef: ptrStr("ch1"), Title: "Section 1.1", Kind: "section", Order: 0},
			{Ref: "ch2", Title: "Chapter 2", Kind: "module", Order: 1},
		},
		WorkItems: []WorkItemImport{
			{Ref: "w1", NodeRef: "ch1_s1", Title: "Read", Type: "reading", PlannedMin: ptrInt(45), EstimateConfidence: ptrFloat(0.7)},
			{Ref: "w2", NodeRef: "ch1_s1", Title: "Exercises", Type: "assignment", PlannedMin: ptrInt(30)},
			{Ref: "w3", NodeRef: "ch2", Title: "Read Ch2", Type: "reading"},
		},
		Dependencies: []DependencyImport{
			{PredecessorRef: "w1", SuccessorRef: "w2"},
		},
	}
	errs := ValidateImportSchema(schema)
	assert.Empty(t, errs)
}

func TestValidateImportSchema_MissingProjectFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(s *ImportSchema)
		wantMsg string
	}{
		{"missing short_id", func(s *ImportSchema) { s.Project.ShortID = "" }, "project.short_id is required"},
		{"missing name", func(s *ImportSchema) { s.Project.Name = "" }, "project.name is required"},
		{"missing domain", func(s *ImportSchema) { s.Project.Domain = "" }, "project.domain is required"},
		{"missing start_date", func(s *ImportSchema) { s.Project.StartDate = "" }, "project.start_date is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validMinimalSchema()
			tc.mutate(s)
			errs := ValidateImportSchema(s)
			assert.NotEmpty(t, errs)
			assert.Contains(t, errs[0].Error(), tc.wantMsg)
		})
	}
}

func TestValidateImportSchema_InvalidDates(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(s *ImportSchema)
		wantMsg string
	}{
		{"bad start_date", func(s *ImportSchema) { s.Project.StartDate = "not-a-date" }, "invalid date format"},
		{"bad target_date", func(s *ImportSchema) { s.Project.TargetDate = ptrStr("not-a-date") }, "invalid date format"},
		{"target before start", func(s *ImportSchema) { s.Project.TargetDate = ptrStr("2025-01-01") }, "must be after start_date"},
		{"bad node due_date", func(s *ImportSchema) { s.Nodes[0].DueDate = ptrStr("bad") }, "invalid date format"},
		{"bad work item due_date", func(s *ImportSchema) { s.WorkItems[0].DueDate = ptrStr("bad") }, "invalid date format"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validMinimalSchema()
			tc.mutate(s)
			errs := ValidateImportSchema(s)
			assert.NotEmpty(t, errs)
			found := false
			for _, e := range errs {
				if contains(e.Error(), tc.wantMsg) {
					found = true
					break
				}
			}
			assert.True(t, found, "expected error containing %q, got %v", tc.wantMsg, errs)
		})
	}
}

func TestValidateImportSchema_DuplicateNodeRef(t *testing.T) {
	s := validMinimalSchema()
	s.Nodes = append(s.Nodes, NodeImport{Ref: "n1", Title: "Dup", Kind: "module"})
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "duplicate ref")
}

func TestValidateImportSchema_DuplicateWorkItemRef(t *testing.T) {
	s := validMinimalSchema()
	s.WorkItems = append(s.WorkItems, WorkItemImport{Ref: "w1", NodeRef: "n1", Title: "Dup", Type: "task"})
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "duplicate ref") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected duplicate ref error")
}

func TestValidateImportSchema_InvalidParentRef(t *testing.T) {
	s := validMinimalSchema()
	s.Nodes[0].ParentRef = ptrStr("nonexistent")
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "not found")
}

func TestValidateImportSchema_InvalidNodeRef(t *testing.T) {
	s := validMinimalSchema()
	s.WorkItems[0].NodeRef = "nonexistent"
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "not found in nodes") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected node_ref not found error")
}

func TestValidateImportSchema_InvalidDependencyRef(t *testing.T) {
	s := validMinimalSchema()
	s.Dependencies = []DependencyImport{
		{PredecessorRef: "w1", SuccessorRef: "nonexistent"},
	}
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "not found in work_items") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected dependency ref not found error")
}

func TestValidateImportSchema_SelfDependency(t *testing.T) {
	s := validMinimalSchema()
	s.Dependencies = []DependencyImport{
		{PredecessorRef: "w1", SuccessorRef: "w1"},
	}
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "self-dependency") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected self-dependency error")
}

func TestValidateImportSchema_CircularDependency(t *testing.T) {
	s := validMinimalSchema()
	s.WorkItems = append(s.WorkItems,
		WorkItemImport{Ref: "w2", NodeRef: "n1", Title: "Task 2", Type: "task"},
		WorkItemImport{Ref: "w3", NodeRef: "n1", Title: "Task 3", Type: "task"},
	)
	s.Dependencies = []DependencyImport{
		{PredecessorRef: "w1", SuccessorRef: "w2"},
		{PredecessorRef: "w2", SuccessorRef: "w3"},
		{PredecessorRef: "w3", SuccessorRef: "w1"},
	}
	errs := ValidateImportSchema(s)
	assert.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if contains(e.Error(), "circular dependency") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected circular dependency error")
}

func TestValidateImportSchema_InvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(s *ImportSchema)
		wantMsg string
	}{
		{"bad node kind", func(s *ImportSchema) { s.Nodes[0].Kind = "invalid" }, "invalid value"},
		{"bad duration_mode", func(s *ImportSchema) { s.WorkItems[0].DurationMode = "invalid" }, "invalid value"},
		{"bad work item status", func(s *ImportSchema) { s.WorkItems[0].Status = "invalid" }, "invalid value"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validMinimalSchema()
			tc.mutate(s)
			errs := ValidateImportSchema(s)
			assert.NotEmpty(t, errs)
			found := false
			for _, e := range errs {
				if contains(e.Error(), tc.wantMsg) {
					found = true
					break
				}
			}
			assert.True(t, found, "expected error containing %q", tc.wantMsg)
		})
	}
}

func TestValidateImportSchema_AssessmentNodeKind(t *testing.T) {
	s := validMinimalSchema()
	s.Nodes[0].Kind = "assessment"

	errs := ValidateImportSchema(s)
	assert.Empty(t, errs)
}

func TestValidateImportSchema_SessionBoundsViolation(t *testing.T) {
	tests := []struct {
		name    string
		policy  *SessionPolicyImport
		wantMsg string
	}{
		{"min > max", &SessionPolicyImport{MinSessionMin: ptrInt(60), MaxSessionMin: ptrInt(15)}, "min_session_min"},
		{"default < min", &SessionPolicyImport{MinSessionMin: ptrInt(30), DefaultSessionMin: ptrInt(15)}, "default_session_min"},
		{"default > max", &SessionPolicyImport{MaxSessionMin: ptrInt(30), DefaultSessionMin: ptrInt(60)}, "default_session_min"},
		{"negative min", &SessionPolicyImport{MinSessionMin: ptrInt(-1)}, "must be positive"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validMinimalSchema()
			s.WorkItems[0].SessionPolicy = tc.policy
			errs := ValidateImportSchema(s)
			assert.NotEmpty(t, errs)
			found := false
			for _, e := range errs {
				if contains(e.Error(), tc.wantMsg) {
					found = true
					break
				}
			}
			assert.True(t, found, "expected error containing %q, got %v", tc.wantMsg, errs)
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

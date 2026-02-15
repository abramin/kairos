package testutil

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/google/uuid"
)

var testShortIDCounter atomic.Int64

// Project options
type ProjectOption func(*domain.Project)

func WithTargetDate(d time.Time) ProjectOption {
	return func(p *domain.Project) {
		p.TargetDate = &d
	}
}

func WithProjectStatus(s domain.ProjectStatus) ProjectOption {
	return func(p *domain.Project) {
		p.Status = s
	}
}

func WithShortID(id string) ProjectOption {
	return func(p *domain.Project) {
		p.ShortID = id
	}
}

func defaultShortID(name string) string {
	upper := strings.ToUpper(name)
	var letters []byte
	for i := 0; i < len(upper) && len(letters) < 3; i++ {
		if upper[i] >= 'A' && upper[i] <= 'Z' {
			letters = append(letters, upper[i])
		}
	}
	for len(letters) < 3 {
		letters = append(letters, 'X')
	}
	n := testShortIDCounter.Add(1)
	return fmt.Sprintf("%s%02d", string(letters), n)
}

func NewTestProject(name string, opts ...ProjectOption) *domain.Project {
	now := time.Now().UTC()
	p := &domain.Project{
		ID:        uuid.New().String(),
		ShortID:   defaultShortID(name),
		Name:      name,
		Domain:    "test",
		StartDate: now.AddDate(0, -1, 0),
		Status:    domain.ProjectActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// PlanNode options
type NodeOption func(*domain.PlanNode)

func WithNodeKind(k domain.NodeKind) NodeOption {
	return func(n *domain.PlanNode) {
		n.Kind = k
	}
}

func WithParentID(id string) NodeOption {
	return func(n *domain.PlanNode) {
		n.ParentID = &id
	}
}

func WithNodeDueDate(d time.Time) NodeOption {
	return func(n *domain.PlanNode) {
		n.DueDate = &d
	}
}

func WithPlannedMinBudget(m int) NodeOption {
	return func(n *domain.PlanNode) {
		n.PlannedMinBudget = &m
	}
}

func WithOrderIndex(i int) NodeOption {
	return func(n *domain.PlanNode) {
		n.OrderIndex = i
	}
}

func NewTestNode(projectID, title string, opts ...NodeOption) *domain.PlanNode {
	now := time.Now().UTC()
	n := &domain.PlanNode{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		Title:      title,
		Kind:       domain.NodeGeneric,
		OrderIndex: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// WorkItem options
type WorkItemOption func(*domain.WorkItem)

func WithPlannedMin(m int) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.PlannedMin = m
	}
}

func WithLoggedMin(m int) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.LoggedMin = m
	}
}

func WithSessionBounds(min, max, def int) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.MinSessionMin = min
		w.MaxSessionMin = max
		w.DefaultSessionMin = def
	}
}

func WithWorkItemDueDate(d time.Time) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.DueDate = &d
	}
}

func WithNotBefore(d time.Time) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.NotBefore = &d
	}
}

func WithWorkItemStatus(s domain.WorkItemStatus) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.Status = s
	}
}

func WithUnits(kind string, total, done int) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.UnitsKind = kind
		w.UnitsTotal = total
		w.UnitsDone = done
	}
}

func WithDurationMode(m domain.DurationMode) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.DurationMode = m
	}
}

func WithWorkItemType(t string) WorkItemOption {
	return func(w *domain.WorkItem) {
		w.Type = t
	}
}

func NewTestWorkItem(nodeID, title string, opts ...WorkItemOption) *domain.WorkItem {
	now := time.Now().UTC()
	w := &domain.WorkItem{
		ID:                 uuid.New().String(),
		NodeID:             nodeID,
		Title:              title,
		Type:               "task",
		Status:             domain.WorkItemTodo,
		DurationMode:       domain.DurationEstimate,
		PlannedMin:         60,
		LoggedMin:          0,
		DurationSource:     domain.SourceManual,
		EstimateConfidence: 0.5,
		MinSessionMin:      15,
		MaxSessionMin:      60,
		DefaultSessionMin:  30,
		Splittable:         true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Session options
type SessionOption func(*domain.WorkSessionLog)

func WithUnitsDelta(d int) SessionOption {
	return func(s *domain.WorkSessionLog) {
		s.UnitsDoneDelta = d
	}
}

func WithNote(n string) SessionOption {
	return func(s *domain.WorkSessionLog) {
		s.Note = n
	}
}

func WithStartedAt(t time.Time) SessionOption {
	return func(s *domain.WorkSessionLog) {
		s.StartedAt = t
	}
}

func NewTestSession(workItemID string, minutes int, opts ...SessionOption) *domain.WorkSessionLog {
	now := time.Now().UTC()
	s := &domain.WorkSessionLog{
		ID:         uuid.New().String(),
		WorkItemID: workItemID,
		StartedAt:  now,
		Minutes:    minutes,
		CreatedAt:  now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

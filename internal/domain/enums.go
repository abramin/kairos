package domain

type RiskLevel string

const (
	RiskOnTrack  RiskLevel = "on_track"
	RiskAtRisk   RiskLevel = "at_risk"
	RiskCritical RiskLevel = "critical"
)

type PlanMode string

const (
	ModeBalanced PlanMode = "balanced"
	ModeCritical PlanMode = "critical"
)

type ReplanTrigger string

const (
	TriggerManual        ReplanTrigger = "MANUAL"
	TriggerSessionLogged ReplanTrigger = "SESSION_LOGGED"
)

type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectPaused   ProjectStatus = "paused"
	ProjectDone     ProjectStatus = "done"
	ProjectArchived ProjectStatus = "archived"
)

type WorkItemStatus string

const (
	WorkItemTodo       WorkItemStatus = "todo"
	WorkItemInProgress WorkItemStatus = "in_progress"
	WorkItemDone       WorkItemStatus = "done"
	WorkItemSkipped    WorkItemStatus = "skipped"
	WorkItemArchived   WorkItemStatus = "archived"
)

type NodeKind string

const (
	NodeWeek       NodeKind = "week"
	NodeModule     NodeKind = "module"
	NodeAssessment NodeKind = "assessment"
	NodeGeneric    NodeKind = "generic"
)

// ValidNodeKinds is the canonical set of accepted node kind strings.
var ValidNodeKinds = map[string]bool{
	"week": true, "module": true, "book": true, "stage": true,
	"section": true, "generic": true, "assessment": true,
}

// ValidWorkItemTypes is the canonical set of accepted work item type strings.
var ValidWorkItemTypes = map[string]bool{
	"reading": true, "practice": true, "review": true,
	"assignment": true, "task": true, "quiz": true,
	"study": true, "training": true, "activity": true,
	"submission": true,
}

type DurationMode string

const (
	DurationFixed    DurationMode = "fixed"
	DurationEstimate DurationMode = "estimate"
	DurationDerived  DurationMode = "derived"
)

type DurationSource string

const (
	SourceManual   DurationSource = "manual"
	SourceTemplate DurationSource = "template"
)

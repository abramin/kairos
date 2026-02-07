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
	TriggerManual          ReplanTrigger = "MANUAL"
	TriggerDeadlineUpdated ReplanTrigger = "DEADLINE_UPDATED"
	TriggerItemAdded       ReplanTrigger = "ITEM_ADDED"
	TriggerItemRemoved     ReplanTrigger = "ITEM_REMOVED"
	TriggerSessionLogged   ReplanTrigger = "SESSION_LOGGED"
	TriggerTemplateInit    ReplanTrigger = "TEMPLATE_INIT"
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
	NodeWeek    NodeKind = "week"
	NodeModule  NodeKind = "module"
	NodeBook    NodeKind = "book"
	NodeStage   NodeKind = "stage"
	NodeSection NodeKind = "section"
	NodeGeneric NodeKind = "generic"
)

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
	SourceRollup   DurationSource = "rollup"
)

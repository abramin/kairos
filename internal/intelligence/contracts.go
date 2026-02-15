package intelligence

import "strings"

// IntentName enumerates all commands the NL parser can produce.
type IntentName string

const (
	IntentWhatNow             IntentName = "what_now"
	IntentStatus              IntentName = "status"
	IntentReplan              IntentName = "replan"
	IntentProjectAdd          IntentName = "project_add"
	IntentProjectImport       IntentName = "project_import"
	IntentProjectUpdate       IntentName = "project_update"
	IntentProjectArchive      IntentName = "project_archive"
	IntentProjectRemove       IntentName = "project_remove"
	IntentNodeAdd             IntentName = "node_add"
	IntentNodeUpdate          IntentName = "node_update"
	IntentNodeRemove          IntentName = "node_remove"
	IntentWorkAdd             IntentName = "work_add"
	IntentWorkUpdate          IntentName = "work_update"
	IntentWorkDone            IntentName = "work_done"
	IntentWorkRemove          IntentName = "work_remove"
	IntentSessionLog          IntentName = "session_log"
	IntentSessionRemove       IntentName = "session_remove"
	IntentTemplateList        IntentName = "template_list"
	IntentTemplateShow        IntentName = "template_show"
	IntentTemplateDraft       IntentName = "template_draft"
	IntentTemplateValidate    IntentName = "template_validate"
	IntentProjectInitFromTmpl IntentName = "project_init_from_template"
	IntentExplainNow          IntentName = "explain_now"
	IntentExplainWhyNot       IntentName = "explain_why_not"
	IntentReviewWeekly        IntentName = "review_weekly"
	IntentSimulate            IntentName = "simulate"
)

// IntentRisk classifies whether an intent is read-only or mutating.
type IntentRisk string

const (
	RiskReadOnly IntentRisk = "read_only"
	RiskWrite    IntentRisk = "write"
)

// IntentRegistryEntry is the single source of truth for one intent's metadata.
type IntentRegistryEntry struct {
	Name      IntentName
	Risk      IntentRisk
	ArgSchema string // human-readable schema for prompt generation (empty = no args)
}

// intentRegistry is the canonical list of all intents. validIntents, writeIntents,
// and the prompt intent list are all derived from this slice.
var intentRegistry = []IntentRegistryEntry{
	{IntentWhatNow, RiskReadOnly, `{ available_min: number (>0), project_scope?: string[], explain?: boolean }`},
	{IntentStatus, RiskReadOnly, `{ project_scope?: string[], recalc?: boolean }`},
	{IntentReplan, RiskWrite, `{ trigger?: string, project_scope?: string[], strategy?: "rebalance"|"deadline_first" }`},
	{IntentProjectAdd, RiskWrite, `{ name: string, domain?: string, start_date?: "YYYY-MM-DD", target_date?: "YYYY-MM-DD" }`},
	{IntentProjectImport, RiskWrite, `{ file_path: string }`},
	{IntentProjectUpdate, RiskWrite, `{ project_id: string, name?: string, target_date?: string|null, status?: "active"|"paused"|"done"|"archived" }`},
	{IntentProjectArchive, RiskWrite, `{ project_id: string }`},
	{IntentProjectRemove, RiskWrite, `{ project_id: string, hard_delete?: boolean }`},
	{IntentNodeAdd, RiskWrite, ``},
	{IntentNodeUpdate, RiskWrite, ``},
	{IntentNodeRemove, RiskWrite, ``},
	{IntentWorkAdd, RiskWrite, ``},
	{IntentWorkUpdate, RiskWrite, ``},
	{IntentWorkDone, RiskWrite, ``},
	{IntentWorkRemove, RiskWrite, ``},
	{IntentSessionLog, RiskWrite, `{ work_item_id: string, minutes: number (>0), units_done_delta?: number, note?: string }`},
	{IntentSessionRemove, RiskWrite, ``},
	{IntentTemplateList, RiskReadOnly, `{}`},
	{IntentTemplateShow, RiskReadOnly, `{ template_id: string }`},
	{IntentTemplateDraft, RiskWrite, `{ prompt: string }`},
	{IntentTemplateValidate, RiskWrite, ``},
	{IntentProjectInitFromTmpl, RiskWrite, `{ template_id: string, project_name: string, start_date: string }`},
	{IntentExplainNow, RiskReadOnly, `{ minutes?: number }`},
	{IntentExplainWhyNot, RiskReadOnly, `{ project_id?: string, work_item_id?: string, candidate_title?: string }`},
	{IntentReviewWeekly, RiskReadOnly, `{}`},
	{IntentSimulate, RiskReadOnly, `{ scenario_text: string }`},
}

// Derived lookup maps, built once at init.
var (
	validIntents map[IntentName]bool
	writeIntents map[IntentName]bool
)

func init() {
	validIntents = make(map[IntentName]bool, len(intentRegistry))
	writeIntents = make(map[IntentName]bool)
	for _, entry := range intentRegistry {
		validIntents[entry.Name] = true
		if entry.Risk == RiskWrite {
			writeIntents[entry.Name] = true
		}
	}
}

// IsValidIntent returns true if the given name is a known intent.
func IsValidIntent(name IntentName) bool {
	return validIntents[name]
}

// IsWriteIntent returns true if the intent is a write/mutation operation.
func IsWriteIntent(name IntentName) bool {
	return writeIntents[name]
}

// IntentNames returns the list of all known intent names (for prompt generation).
func IntentNames() []string {
	names := make([]string, len(intentRegistry))
	for i, entry := range intentRegistry {
		names[i] = string(entry.Name)
	}
	return names
}

// IntentNamesCSV returns all intent names as a comma-separated string.
func IntentNamesCSV() string {
	return strings.Join(IntentNames(), ", ")
}

// IntentArgSchemas returns "- intent_name: { schema }" lines for intents that have arg schemas.
func IntentArgSchemas() string {
	var b strings.Builder
	for _, entry := range intentRegistry {
		if entry.ArgSchema == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(string(entry.Name))
		b.WriteString(": ")
		b.WriteString(entry.ArgSchema)
		b.WriteString("\n")
	}
	return b.String()
}

// IntentRiskClassification returns "- risk_level: intent1, intent2, ..." lines grouped by risk.
func IntentRiskClassification() string {
	var readOnly, write []string
	for _, entry := range intentRegistry {
		if entry.Risk == RiskReadOnly {
			readOnly = append(readOnly, string(entry.Name))
		} else {
			write = append(write, string(entry.Name))
		}
	}
	return "- read_only: " + strings.Join(readOnly, ", ") + "\n" +
		"- write: ALL other intents (" + strings.Join(write, ", ") + ")"
}

// ParsedIntent is the structured output of NL-to-command parsing.
type ParsedIntent struct {
	Intent               IntentName             `json:"intent"`
	Risk                 IntentRisk             `json:"risk"`
	Arguments            map[string]interface{} `json:"arguments"`
	Confidence           float64                `json:"confidence"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
	ClarificationOptions []string               `json:"clarification_options"`
	Rationale            string                 `json:"rationale,omitempty"`
}

// ParsedIntentErrorCode enumerates parse failure reasons.
type ParsedIntentErrorCode string

const (
	ErrCodeArgSchemaMismatch   ParsedIntentErrorCode = "ARGUMENT_SCHEMA_MISMATCH"
	ErrCodeInvalidOutputFormat ParsedIntentErrorCode = "INVALID_OUTPUT_FORMAT"
)

// ParsedIntentError is returned when NL parsing fails.
type ParsedIntentError struct {
	Code                 ParsedIntentErrorCode `json:"code"`
	Message              string                `json:"message"`
	ClarificationOptions []string              `json:"clarification_options,omitempty"`
}

func (e *ParsedIntentError) Error() string {
	return string(e.Code) + ": " + e.Message
}

// ExecutionState describes what happened after parsing an intent.
type ExecutionState string

const (
	StateExecuted           ExecutionState = "executed"
	StateNeedsConfirmation  ExecutionState = "needs_confirmation"
	StateNeedsClarification ExecutionState = "needs_clarification"
	StateRejected           ExecutionState = "rejected"
)

// AskResolution is the full result of the `ask` command pipeline.
type AskResolution struct {
	ParsedIntent     *ParsedIntent  `json:"parsed_intent"`
	ExecutionState   ExecutionState `json:"execution_state"`
	ExecutionMessage string         `json:"execution_message"`
	CommandHint      string         `json:"command_hint,omitempty"`
}

// EvidenceRefType classifies the kind of evidence an explanation factor references.
type EvidenceRefType string

const (
	EvidenceScoreFactor EvidenceRefType = "score_factor"
	EvidenceConstraint  EvidenceRefType = "constraint"
	EvidenceHistory     EvidenceRefType = "history"
)

// ExplanationFactor is a single factor in an LLM explanation.
type ExplanationFactor struct {
	Name            string          `json:"name"`
	Impact          string          `json:"impact"`    // "high", "medium", "low"
	Direction       string          `json:"direction"` // "push_for", "push_against"
	EvidenceRefType EvidenceRefType `json:"evidence_ref_type"`
	EvidenceRefKey  string          `json:"evidence_ref_key"`
	Summary         string          `json:"summary"`
}

// Counterfactual is a hypothetical scenario in an explanation.
type Counterfactual struct {
	Label           string `json:"label"`
	PredictedEffect string `json:"predicted_effect"`
}

// ExplanationContext identifies what kind of request is being explained.
type ExplanationContext string

const (
	ExplainContextWhatNow      ExplanationContext = "what_now"
	ExplainContextWhyNot       ExplanationContext = "why_not"
	ExplainContextWeeklyReview ExplanationContext = "weekly_review"
)

// LLMExplanation is a narrative explanation grounded in engine trace data.
type LLMExplanation struct {
	Context         ExplanationContext  `json:"context"`
	SummaryShort    string              `json:"summary_short"`
	SummaryDetailed string              `json:"summary_detailed"`
	Factors         []ExplanationFactor `json:"factors"`
	Counterfactuals []Counterfactual    `json:"counterfactuals,omitempty"`
	Confidence      float64             `json:"confidence"`
	Source          string              `json:"source"` // "llm" or "deterministic"
}

// TemplateDraft is the result of LLM-assisted template generation.
type TemplateDraft struct {
	TemplateJSON      map[string]interface{} `json:"template_json"`
	Validation        TemplateDraftValidation `json:"validation"`
	RepairSuggestions []string                `json:"repair_suggestions,omitempty"`
	Confidence        float64                 `json:"confidence"`
}

// TemplateDraftValidation holds the result of deterministic template validation.
type TemplateDraftValidation struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ConfirmationPolicy defines when parsed intents may auto-execute.
type ConfirmationPolicy struct {
	AutoExecuteReadThreshold float64
	AlwaysConfirmWrite       bool // always true
}

// DefaultConfirmationPolicy returns a policy with the given read threshold.
func DefaultConfirmationPolicy(threshold float64) ConfirmationPolicy {
	return ConfirmationPolicy{
		AutoExecuteReadThreshold: threshold,
		AlwaysConfirmWrite:       true,
	}
}

// Evaluate determines the execution state for a parsed intent.
func (p ConfirmationPolicy) Evaluate(intent *ParsedIntent) ExecutionState {
	if intent.Risk == RiskWrite || IsWriteIntent(intent.Intent) {
		return StateNeedsConfirmation
	}
	if intent.Confidence >= p.AutoExecuteReadThreshold {
		return StateExecuted
	}
	return StateNeedsClarification
}

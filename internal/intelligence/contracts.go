package intelligence


// IntentName enumerates all commands the NL parser can produce.
type IntentName string

const (
	IntentWhatNow               IntentName = "what_now"
	IntentStatus                IntentName = "status"
	IntentReplan                IntentName = "replan"
	IntentProjectAdd            IntentName = "project_add"
	IntentProjectImport         IntentName = "project_import"
	IntentProjectUpdate         IntentName = "project_update"
	IntentProjectArchive        IntentName = "project_archive"
	IntentProjectRemove         IntentName = "project_remove"
	IntentNodeAdd               IntentName = "node_add"
	IntentNodeUpdate            IntentName = "node_update"
	IntentNodeRemove            IntentName = "node_remove"
	IntentWorkAdd               IntentName = "work_add"
	IntentWorkUpdate            IntentName = "work_update"
	IntentWorkDone              IntentName = "work_done"
	IntentWorkRemove            IntentName = "work_remove"
	IntentSessionLog            IntentName = "session_log"
	IntentSessionRemove         IntentName = "session_remove"
	IntentTemplateList          IntentName = "template_list"
	IntentTemplateShow          IntentName = "template_show"
	IntentTemplateDraft         IntentName = "template_draft"
	IntentTemplateValidate      IntentName = "template_validate"
	IntentProjectInitFromTmpl   IntentName = "project_init_from_template"
	IntentExplainNow            IntentName = "explain_now"
	IntentExplainWhyNot         IntentName = "explain_why_not"
	IntentReviewWeekly          IntentName = "review_weekly"
	IntentSimulate              IntentName = "simulate"
)

// validIntents is the set of known intent names for validation.
var validIntents = map[IntentName]bool{
	IntentWhatNow: true, IntentStatus: true, IntentReplan: true,
	IntentProjectAdd: true, IntentProjectImport: true, IntentProjectUpdate: true, IntentProjectArchive: true,
	IntentProjectRemove: true, IntentNodeAdd: true, IntentNodeUpdate: true,
	IntentNodeRemove: true, IntentWorkAdd: true, IntentWorkUpdate: true,
	IntentWorkDone: true, IntentWorkRemove: true, IntentSessionLog: true,
	IntentSessionRemove: true, IntentTemplateList: true, IntentTemplateShow: true,
	IntentTemplateDraft: true, IntentTemplateValidate: true,
	IntentProjectInitFromTmpl: true, IntentExplainNow: true,
	IntentExplainWhyNot: true, IntentReviewWeekly: true, IntentSimulate: true,
}

// IsValidIntent returns true if the given name is a known intent.
func IsValidIntent(name IntentName) bool {
	return validIntents[name]
}

// IntentRisk classifies whether an intent is read-only or mutating.
type IntentRisk string

const (
	RiskReadOnly IntentRisk = "read_only"
	RiskWrite    IntentRisk = "write"
)

// writeIntents is the set of intents that always require confirmation.
var writeIntents = map[IntentName]bool{
	IntentReplan: true, IntentProjectAdd: true, IntentProjectUpdate: true,
	IntentProjectImport: true, IntentProjectArchive: true, IntentProjectRemove: true,
	IntentNodeAdd: true, IntentNodeUpdate: true, IntentNodeRemove: true,
	IntentWorkAdd: true, IntentWorkUpdate: true, IntentWorkDone: true,
	IntentWorkRemove: true, IntentSessionLog: true, IntentSessionRemove: true,
	IntentTemplateDraft: true, IntentTemplateValidate: true,
	IntentProjectInitFromTmpl: true,
}

// IsWriteIntent returns true if the intent is a write/mutation operation.
func IsWriteIntent(name IntentName) bool {
	return writeIntents[name]
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
	Context          ExplanationContext  `json:"context"`
	SummaryShort     string              `json:"summary_short"`
	SummaryDetailed  string              `json:"summary_detailed"`
	Factors          []ExplanationFactor `json:"factors"`
	Counterfactuals  []Counterfactual    `json:"counterfactuals,omitempty"`
	Confidence       float64             `json:"confidence"`
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

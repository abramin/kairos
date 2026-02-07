Great. Here’s both:

1. **targeted updates to PRD v2** for your fixed model choice, and
2. `docs/contracts-llm.md` ready to drop in.

---

## PRD v2 updates (delta)

### Update A: Model policy

Replace generic model strategy with:

* Primary model: **Llama 3.2 via Ollama (local)**
* All v2 LLM features use this model initially:

  * NL parsing
  * explanations
  * template drafting
* Future multi-model routing is optional and deferred.

### Update B: Write safety

Add explicit rule:

* NL-derived **write operations are never auto-applied**.
* Any mutation (`add/update/remove/archive`, template save/apply, replan apply) requires confirmation step.
* Read-only intents may auto-run if confidence threshold is met.

### Update C: Runtime targets tuned for local Llama 3.2

Latency SLOs become targets, not hard guarantees:

* Parse target: < 3.0s
* Explain target: < 6.0s
* Template draft target: < 8.0s

Fallback behavior unchanged:

* If Ollama unavailable or schema parse fails, deterministic CLI remains fully operational.

### Update D: Deterministic contract reinforcement

Add:

* LLM outputs are advisory.
* Final command object must pass schema + business validation.
* Scheduler ranking/order remains deterministic and independent of LLM text output.

---

````markdown
# docs/contracts-llm.md

## Purpose

Defines strict LLM-side contracts for v2 intelligence features in Ordo.

These contracts support:

- Natural language to command parsing (`ParsedIntent`)
- Recommendation/status explanations (`LLMExplanation`)
- Template drafting (`TemplateDraft`)

All LLM outputs are **advisory** and must pass validation before execution.

---

## Global Envelope

```ts
type ISODateTime = string; // RFC3339
type UUID = string;

interface LLMEnvelope<T> {
  contract_version: "1.0.0";
  model: "llama3.2";
  generated_at: ISODateTime;
  data: T;
}
````

---

## 1) ParsedIntent Contract (NL -> Structured Command)

### Type Definitions

```ts
type IntentName =
  | "what_now"
  | "status"
  | "replan"
  | "project_add"
  | "project_update"
  | "project_archive"
  | "project_remove"
  | "node_add"
  | "node_update"
  | "node_remove"
  | "work_add"
  | "work_update"
  | "work_done"
  | "work_remove"
  | "session_log"
  | "session_remove"
  | "template_list"
  | "template_show"
  | "template_draft"
  | "template_validate"
  | "project_init_from_template"
  | "explain_now"
  | "explain_why_not"
  | "review_weekly"
  | "simulate";
```

```ts
type IntentRisk = "read_only" | "write";

interface ParsedIntent {
  intent: IntentName;
  risk: IntentRisk;

  // Command args after normalization. Unknown keys are forbidden.
  arguments: Record<string, unknown>;

  // 0..1 confidence that intent+args are correct
  confidence: number;

  // True if caller must ask user to confirm before execution.
  // Required true for all risk=write.
  requires_confirmation: boolean;

  // If confidence is low or ambiguity exists, provide candidate interpretations.
  clarification_options: string[];

  // Optional short explanation of parse decision.
  rationale?: string;
}
```

### Validation Rules

1. `intent` must be a known enum value.
2. `confidence` in `[0,1]`.
3. `risk="write"` => `requires_confirmation=true` (mandatory).
4. `clarification_options` required (min 1) when `confidence < 0.85`.
5. `arguments` must match intent-specific schema (below).
6. No extra top-level fields outside contract.

### Intent-Specific Argument Schemas (v2 minimum)

```ts
interface WhatNowArgs {
  available_min: number;            // >0
  project_scope?: UUID[];
  explain?: boolean;
}
```

```ts
interface StatusArgs {
  project_scope?: UUID[];
  recalc?: boolean;
}
```

```ts
interface ReplanArgs {
  trigger?: "MANUAL" | "DEADLINE_UPDATED" | "ITEM_ADDED" | "ITEM_REMOVED" | "SESSION_LOGGED" | "TEMPLATE_INIT";
  project_scope?: UUID[];
  strategy?: "rebalance" | "deadline_first";
}
```

```ts
interface ProjectAddArgs {
  name: string;
  domain?: string;
  start_date?: string;              // YYYY-MM-DD
  target_date?: string;             // YYYY-MM-DD
}
```

```ts
interface ProjectUpdateArgs {
  project_id: UUID;
  name?: string;
  target_date?: string | null;      // YYYY-MM-DD or null clear
  status?: "active" | "paused" | "done" | "archived";
}
```

```ts
interface ProjectRemoveArgs {
  project_id: UUID;
  hard_delete?: boolean;            // default false
}
```

```ts
interface SessionLogArgs {
  work_item_id: UUID;
  minutes: number;                  // >0
  units_done_delta?: number;        // >=0
  note?: string;
}
```

```ts
interface ProjectInitFromTemplateArgs {
  template_id: string;
  project_name: string;
  start_date: string;               // YYYY-MM-DD
  target_date?: string;             // YYYY-MM-DD
  vars?: Record<string, string | number | boolean>;
}
```

```ts
interface ExplainWhyNotArgs {
  minutes?: number;                 // optional context window
  project_id?: UUID;
  work_item_id?: UUID;
  candidate_title?: string;
}
```

```ts
interface SimulateArgs {
  scenario_text: string;            // user hypothetical
}
```

### Parse Failure Contract

```ts
interface ParsedIntentError {
  code:
    | "LOW_CONFIDENCE"
    | "AMBIGUOUS"
    | "UNSUPPORTED_INTENT"
    | "ARGUMENT_SCHEMA_MISMATCH"
    | "INVALID_OUTPUT_FORMAT";
  message: string;
  clarification_options?: string[];
}
```

---

## 2) LLMExplanation Contract (Narrative over deterministic trace)

### Type Definitions

```ts
type EvidenceRefType = "score_factor" | "risk_metric" | "constraint" | "history";

interface ExplanationFactor {
  name: string;                     // e.g. "Deadline pressure"
  impact: "high" | "medium" | "low";
  direction: "push_for" | "push_against";
  evidence_ref_type: EvidenceRefType;
  evidence_ref_key: string;         // key in deterministic trace payload
  summary: string;
}
```

```ts
interface Counterfactual {
  label: string;                    // e.g. "If you skip today"
  predicted_effect: string;         // narrative
}
```

```ts
interface LLMExplanation {
  context:
    | "what_now"
    | "status"
    | "why_not"
    | "weekly_review"
    | "simulation";

  summary_short: string;            // 1-2 lines
  summary_detailed: string;         // concise paragraph(s)

  factors: ExplanationFactor[];     // must reference real trace keys
  counterfactuals?: Counterfactual[];

  confidence: number;               // model confidence in narrative faithfulness
}
```

### Validation Rules

1. `factors[].evidence_ref_key` must exist in provided deterministic trace map.
2. `summary_*` must not introduce actions absent from engine output.
3. `confidence` in `[0,1]`.
4. If no valid evidence refs, reject explanation output.

### Explanation Failure Contract

```ts
interface LLMExplanationError {
  code:
    | "MISSING_TRACE"
    | "INVALID_EVIDENCE_REF"
    | "UNFAITHFUL_EXPLANATION"
    | "INVALID_OUTPUT_FORMAT";
  message: string;
}
```

---

## 3) TemplateDraft Contract (NL -> Template JSON draft)

### Type Definitions

```ts
interface TemplateDraft {
  // Must conform to Ordo template schema used in templates/*.json
  template_json: Record<string, unknown>;

  // Validation results from deterministic validator
  validation: {
    is_valid: boolean;
    errors: string[];
    warnings: string[];
  };

  // Optional model-proposed fixes (advisory)
  repair_suggestions?: string[];

  confidence: number;               // confidence in draft usefulness
}
```

### Validation Rules

1. `template_json` must pass JSON schema validation before save.
2. `validation.is_valid=false` blocks apply/save unless user forces `--allow-invalid` (off by default).
3. `confidence` in `[0,1]`.
4. `repair_suggestions` optional but recommended when invalid.

### Template Draft Failure Contract

```ts
interface TemplateDraftError {
  code:
    | "SCHEMA_INVALID"
    | "SEMANTIC_INVALID"
    | "UNSAFE_DEFAULTS"
    | "INVALID_OUTPUT_FORMAT";
  message: string;
  errors?: string[];
}
```

---

## 4) Confirmation Policy Contract

### Rule Set

```ts
interface ConfirmationPolicy {
  auto_execute_read_confidence_threshold: number; // default 0.85
  always_confirm_write: true;                     // hard true
}
```

Operational rules:

1. If `risk=read_only` and `confidence >= threshold`, command may run directly.
2. If `risk=read_only` and confidence below threshold, require clarification.
3. If `risk=write`, always require confirmation, regardless of confidence.

---

## 5) CLI Integration Contract

### `ask` command result

```ts
interface AskResolution {
  parsed_intent: ParsedIntent;
  execution_state:
    | "executed"
    | "needs_confirmation"
    | "needs_clarification"
    | "rejected";
  execution_message: string;
}
```

### Invariants

* `execution_state="executed"` is impossible when `parsed_intent.risk="write"` unless explicit user confirmation was captured in same flow.
* Rejected outcomes must include actionable next step.

---

## 6) Observability Event Shapes

```ts
interface LLMCallEvent {
  event: "llm_call";
  task: "parse" | "explain" | "template_draft";
  model: "llama3.2";
  latency_ms: number;
  success: boolean;
  error_code?: string;
}
```

```ts
interface ParseQualityEvent {
  event: "parse_quality";
  intent: string;
  confidence: number;
  needed_clarification: boolean;
  confirmed_by_user?: boolean;
}
```

---

## 7) Versioning

* `contract_version = "1.0.0"` for all LLM contracts in v2 initial release.
* Backward-incompatible changes require version bump and migration notes.

---

# PRD v2: KAiros Intelligence Layer (Ollama Integration)

## 1) Goal

Add an LLM-powered intelligence layer (via Ollama) to Ordo that improves input speed, explainability, and planning assistance, while keeping the scheduler deterministic and rule-based.

Core principle: **LLM suggests, engine decides**.

---

## 2) Why v2

v1 already schedules well with explicit data. v2 should reduce friction and increase trust by enabling:

* natural-language capture
* smart template authoring
* clearer “why” explanations
* guided weekly reviews
* scenario simulation

without giving the model authority over final scheduling.

---

## 3) Scope

## In scope (v2)

1. Natural language command parsing to structured commands
2. LLM-assisted template generation and validation hints
3. Recommendation explanations in plain language
4. Weekly review summaries with actionable adjustments
5. “What-if” simulation assistant
6. Confidence + guardrails + fallback behavior

## Out of scope (v2)

* Autonomous background execution of actions without confirmation
* Replacing deterministic scorer with LLM ranking
* Cloud LLM dependencies (Ollama only in v2 baseline)
* Multi-user collaboration

---

## 4) User Stories

1. “I have 50 min and low energy, what should I do?”

   * System maps this to `what-now --minutes 50 --energy low`, runs engine, then explains.

2. “Create a 12-week Spanish B1 plan with 4 study days/week.”

   * LLM drafts template JSON, validator checks, user confirms, then template saved.

3. “Why did you choose OU over philosophy today?”

   * Engine reasons + score components are translated into readable explanation.

4. “If I miss today, what happens?”

   * Simulation computes risk changes and proposes recovery options.

---

## 5) Architecture Additions

## 5.1 New modules

* `llm/ollama_client`

  * model invocation, timeout, retries, local endpoint handling

* `llm/prompt_router`

  * chooses prompt type: parse, explain, summarize, template_draft, simulate_narrative

* `llm/structured_output`

  * strict JSON schema parsing, validation, coercion rules

* `llm/safety_guardrails`

  * prompt injection defenses, tool scope limits, max token/latency policies

* `app/intelligence_service`

  * orchestration layer between CLI, deterministic engine, and LLM

## 5.2 Deterministic boundary (hard rule)

* LLM cannot write directly to DB.
* LLM outputs must be transformed into typed commands and pass validation.
* Final recommendation order always produced by deterministic scheduler.

---

## 6) Functional Requirements

## FR1: Natural Language to Command (NL2C)

* Input: free text from CLI `ask "..."`.
* Output: structured intent:

  * command name
  * arguments
  * confidence score
  * clarification_needed flag
* If confidence < threshold, return 2-3 explicit interpretations (no silent execution).

## FR2: Explainability Layer

* For any `what-now` result, provide:

  * short explanation
  * detailed explanation with score factors
  * “why not X” support

Source of truth is engine traces, not model hallucination.

## FR3: Template Copilot

* Convert prompt to template JSON draft.
* Run schema + semantic validator:

  * bounds checks
  * dependency cycle checks
  * repeat expansion sanity
* Present diff/preview before save.

## FR4: Weekly Review Assistant

* Summarize past 7 days:

  * planned vs logged
  * risk changes
  * missed sessions
* Suggest parameter adjustments:

  * buffer_pct
  * session defaults
  * variation weight
* Suggestions are opt-in patches.

## FR5: What-if Simulator

* User asks scenario.
* Engine runs hypothetical changes.
* LLM narrates outcomes and tradeoffs.
* No DB mutation unless explicit `apply`.

---

## 7) Non-Functional Requirements

* Local-first via Ollama endpoint.
* P95 latency targets:

  * NL2C parse: < 2.5s
  * Explain summary: < 2.0s
  * Template draft: < 6.0s
* Full offline fallback:

  * if Ollama unavailable, CLI continues with deterministic features.
* Observability:

  * log prompt type, model, latency, token estimates, parse success/failure.

---

## 8) Data Contracts (new)

## 8.1 ParsedIntent

* `intent`: enum (`what_now`, `status`, `replan`, `project_add`, etc.)
* `arguments`: object
* `confidence`: 0..1
* `requires_confirmation`: bool
* `clarification_options`: string[]

## 8.2 LLMExplanation

* `summary`
* `factors`: array of `{name, impact, evidence_ref}`
* `counterfactuals`: optional

## 8.3 TemplateDraft

* `json_draft`
* `validation_errors`
* `validation_warnings`
* `repair_suggestions`

---

## 9) CLI Additions

* `ask "<natural language>"`
* `explain now`
* `explain why-not --project <id>|--work-item <id>`
* `review weekly`
* `simulate "<scenario>"`
* `template draft "<prompt>"`
* `template validate <file>`

Flags:

* `--llm off` to disable LLM per command
* `--model <name>`
* `--json` for machine-readable output

---

## 10) Guardrails and Safety

1. No destructive command auto-execution from NL parse.
2. Any delete/archive/update from NL requires explicit confirmation.
3. Prompt injection mitigation:

   * fixed system instructions
   * no secret/tool disclosure
   * allowlist tool actions per prompt type
4. Strict JSON schema validation for all structured outputs.
5. Rate limiting and timeout fallback.

---

## 11) Model Strategy (Ollama)

Baseline model roles:

* Parser model (small, fast): NL2C
* Explainer model (mid-size): reasoning narration
* Template model (mid/large): JSON drafting

Selection policy:

* default model per task
* user override via config
* auto fallback chain if parse fails

---

## 12) Configuration

`config/llm.json`:

* enabled: true/false
* endpoint
* model map by task
* timeout_ms
* max_retries
* confidence thresholds
* confirmation policy

---

## 13) Acceptance Criteria

1. 90%+ of common `ask` commands map to correct structured intent in test corpus.
2. All LLM outputs pass schema validation or fail gracefully with actionable message.
3. Scheduler outputs are unchanged whether LLM is on/off for same structured request.
4. `explain why-not` cites real scoring factors from engine trace.
5. Template draft flow can create valid Calimove/OU-style templates with preview and confirm.
6. If Ollama is down, core v1 commands remain fully functional.

---

## 14) Test Plan (v2)

* Contract tests for ParsedIntent and TemplateDraft schemas
* Injection/prompt-escape adversarial tests
* Determinism test: same structured input => same recommendation ordering
* Fallback tests: simulated Ollama timeout/unavailable
* Golden tests for explanations aligned to engine traces
* E2E flows:

  * ask → parse → command → output
  * template draft → validate → save → init project

---

## 15) Rollout Plan

Phase 1:

* NL2C for read-only commands (`status`, `what-now`, `explain`)

Phase 2:

* Template copilot + weekly review

Phase 3:

* Simulations + guarded mutation intents with confirmation

Feature flags on each phase.

---

## 16) Risks

* Hallucinated command arguments
  Mitigation: strict schema + confidence gating + confirmation

* Drift between explanation and real logic
  Mitigation: explanation must consume engine traces only

* Latency frustration on local hardware
  Mitigation: task-model routing + fast parser model + fallback to non-LLM mode

---
PRD v2 updates (delta)
Update A: Model policy
Replace generic model strategy with:
Primary model: Llama 3.2 via Ollama (local)
All v2 LLM features use this model initially:
NL parsing
explanations
template drafting
Future multi-model routing is optional and deferred.
Update B: Write safety
Add explicit rule:
NL-derived write operations are never auto-applied.
Any mutation (add/update/remove/archive, template save/apply, replan apply) requires confirmation step.
Read-only intents may auto-run if confidence threshold is met.
Update C: Runtime targets tuned for local Llama 3.2
Latency SLOs become targets, not hard guarantees:
Parse target: < 3.0s
Explain target: < 6.0s
Template draft target: < 8.0s
Fallback behavior unchanged:
If Ollama unavailable or schema parse fails, deterministic CLI remains fully operational.
Update D: Deterministic contract reinforcement
Add:
LLM outputs are advisory.
Final command object must pass schema + business validation.
Scheduler ranking/order remains deterministic and independent of LLM text output.
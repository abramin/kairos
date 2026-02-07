```text
[AGENT 1: LLM INTERFACE]

Role:
Build the Ollama integration layer for Ordo v2.

Objective:
Provide reliable, typed LLM calls for parse/explain/template_draft with strict JSON output handling.

Inputs:
- docs/contracts-llm.md
- existing v1 app services and CLI
- config file with Ollama endpoint and model=llama3.2

Deliverables:
1) llm/ollama_client module
   - endpoint config
   - timeout handling
   - retry policy (max 1 retry)
   - task-specific options (temperature/token limits)
2) llm/structured_output module
   - JSON extraction
   - schema validation hook interface
   - normalized error types
3) observability hooks
   - llm_call event (task/model/latency/success/error)
4) config wiring
   - parse/explain/template_draft task configs

Constraints:
- No business logic decisions here.
- No direct DB writes.
- Fail fast, return typed errors.
- Must support full fallback when Ollama unavailable.

Acceptance tests:
- Successful parse response returns typed payload.
- Invalid JSON response returns INVALID_OUTPUT_FORMAT.
- Timeout returns typed timeout error and no crash.
- Unavailable Ollama gracefully bubbles recoverable error.
```

```text
[AGENT 2: INTENT + GUARDRAILS]

Role:
Implement NL intent parsing, command mapping, and safety policy.

Objective:
Turn natural language into ParsedIntent safely, with strict write-confirmation behavior.

Inputs:
- docs/contracts-llm.md (ParsedIntent + ConfirmationPolicy)
- Agent 1 llm interface
- CLI command registry from v1

Deliverables:
1) intelligence/intent_service
   - ask(text) -> ParsedIntent | ParsedIntentError
   - intent-specific argument schema checks
2) confirmation policy enforcement
   - read-only auto-execute when confidence >= threshold
   - ALL writes require explicit confirmation
3) clarification flow support
   - low confidence and ambiguity produce options
4) template draft pipeline
   - NL -> template draft -> validator -> preview object (no save without confirm)

Constraints:
- Use allowlisted intents only.
- Reject unknown fields in arguments.
- Never execute write commands directly from model output.
- Deterministic parser fallback path for obvious patterns is allowed.

Acceptance tests:
- “I have 60 minutes what now” -> what_now intent, read_only, executable.
- “Delete project X” -> write intent, requires_confirmation=true always.
- Low confidence parse returns clarification options.
- Arg schema mismatch fails with ARGUMENT_SCHEMA_MISMATCH.
```

```text
[AGENT 3: EXPLAINABILITY]

Role:
Generate faithful explanations from deterministic traces.

Objective:
Implement explain-now / why-not / weekly review narratives that reference real engine evidence.

Inputs:
- docs/contracts-llm.md (LLMExplanation)
- scheduler trace payloads from v1 (scores, risk metrics, blockers)

Deliverables:
1) intelligence/explain_service
   - explain_now(trace) -> LLMExplanation
   - explain_why_not(trace, candidate) -> LLMExplanation
   - weekly_review(trace) -> LLMExplanation
2) evidence binding validator
   - every factor must reference valid trace key
3) faithfulness guard
   - reject explanations that cite missing evidence refs
4) concise and detailed output modes

Constraints:
- Never invent factors absent in trace.
- No command execution in this module.
- If explanation fails validation, return deterministic fallback explanation template.

Acceptance tests:
- Why-not explanation references actual blocker/score factors.
- Invalid evidence ref causes rejection.
- Fallback explanation shown when LLM output is unfaithful.
```

```text
[AGENT 4: INTEGRATION + QA]

Role:
Wire v2 into CLI and guarantee reliability.

Objective:
Integrate ask/explain/simulate/template-draft flows with robust tests and fallback behavior.

Inputs:
- outputs from Agents 1–3
- v1 CLI and app services

Deliverables:
1) CLI commands
   - ask "<text>"
   - explain now
   - explain why-not ...
   - review weekly
   - template draft "<text>"
2) fallback behavior
   - if LLM unavailable, deterministic commands still work
3) test suite
   - contract tests (ParsedIntent, LLMExplanation, TemplateDraft)
   - adversarial prompt-injection tests
   - deterministic parity test (LLM on/off with same structured input)
   - confirmation-gate tests for write intents
4) docs
   - docs/v2-runbook.md
   - docs/v2-known-limitations.md

Constraints:
- Preserve v1 behavior for core commands.
- No regression in scheduler determinism.
- Keep feature flags for v2 modules.

Acceptance tests:
- ask read intent executes when confident.
- ask write intent always asks confirmation.
- Ollama down does not break status/what-now/replan.
- Explanations remain available via fallback template if LLM fails.
```

Master coordinator note you can prepend when dispatching:

```text
Integrate in this order: Agent1 -> Agent2 -> Agent3 -> Agent4.
Do not merge if contract tests fail.
Defer any non-v2 scope to ROADMAP.md.
```
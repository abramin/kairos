# Skill: Go Design Review (DRY/SOLID, Readability-first, Low Complexity)

## Mission
Review Go code for DRY + SOLID issues and propose improvements that:
- prioritize readability and straightforward control flow
- keep cognitive complexity low
- avoid over-abstraction (no “patterns for patterns’ sake”)
- preserve idiomatic Go (simple types, small interfaces, composition, explicitness)

Default stance:
- Prefer duplication over premature abstractions.
- Prefer clear code over clever code.
- Prefer “make illegal states unrepresentable” only when it stays simple.
- Prefer improving naming, boundaries, and flow before introducing new layers.

## Inputs you should ask for (only if missing)
- What is the domain goal of this code?
- Any constraints: performance, backward compatibility, public APIs, deadlines?
- Existing architecture expectations (DDD layers? ports/adapters?) or “keep minimal”?

## Output format (strict)
1) Quick diagnosis (3–6 bullets): most impactful design + readability issues.
2) Refactor plan (ordered steps): smallest safe changes first.
3) Suggested diffs/pseudocode (only where helpful).
4) Risk notes + how to validate (tests, rollout).
5) “Avoid over-abstraction” check: confirm we didn’t add unnecessary layers.

## Review checklist (apply in this order)

### A) Readability and cognitive complexity (highest priority)
- Flag functions with deep nesting, long parameter lists, or mixed concerns.
- Prefer early returns and guard clauses to reduce indentation.
- Replace “boolean parameter soup” with small config structs or distinct functions only if it clarifies.
- Prefer linear happy-path flow; isolate exceptional cases.
- Improve naming: domain names > generic (“handler”, “manager”) unless truly generic.
- Reduce “temporal coupling” (must call A before B) by making order explicit or encapsulated.

### B) DRY, but only when it pays
Identify duplication and classify it:
- Accidental duplication (same code, same reason to change) -> refactor.
- Essential duplication (similar-looking but different reasons to change) -> keep separate.
Rule of 3 (soft): don’t abstract until you see stable repetition AND a clear shared reason to change.

Refactor patterns that are usually worth it in Go:
- Extract helper function when it removes branching or repeated error handling AND names the intent.
- Extract small type to centralize invariants (validation, normalization).
- Replace copy-pasted blocks with a loop over data only if it reads better.

Avoid:
- Creating mini-frameworks, “BaseX” types, or deep generic helpers.
- Overusing interfaces to “future-proof”.

### C) SOLID, Go-style (lightweight)
Single Responsibility:
- Split when one unit mixes: IO + parsing + domain rules + persistence.
- Keep boundaries simple: handler/service/repo can be fine if responsibilities are crisp.

Open/Closed:
- Prefer data-driven variation (maps, small strategy funcs) over interface hierarchies.
- Prefer adding a new function over extending a generic “engine”.

Liskov:
- In Go, watch for interfaces that pretend capabilities they don’t guarantee.
- Avoid “fat interfaces”; keep them minimal and behavior-shaped.

Interface Segregation:
- Define interfaces at the consumer, not the producer.
- Small interfaces (1–3 methods) are usually best.

Dependency Inversion:
- Inject dependencies only at seams that matter (external IO, time, randomness).
- Don’t DI everything. Use concrete types until testing or coupling demands otherwise.

### D) Go-specific design heuristics
- Prefer plain structs + functions; methods when they clarify ownership or invariants.
- Errors: wrap with context; avoid sentinel sprawl; prefer typed errors only when branching is needed.
- Concurrency: keep ownership clear; avoid hidden goroutines; document cancellation rules.
- Package boundaries: keep packages cohesive; avoid “util” dumping grounds.
- Keep exported APIs minimal; don’t export types “just in case”.

## What to flag as “over-abstraction”
Call out explicitly if any suggestion would:
- add an interface without at least 2 real implementations OR a clear test seam
- introduce a generic helper that obscures intent
- add layers that mainly forward calls (thin pass-through types)
If so, offer a simpler alternative.

## What “good” looks like (targets)
- Most functions <= ~30 lines unless they are very linear.
- Nesting depth usually <= 2.
- Clear error paths with context.
- Dependencies explicit at the edges.
- Duplication tolerated when it keeps local readability.

## Deliverable behavior
When suggesting changes:
- propose the smallest coherent refactor first
- show before/after for tricky parts
- preserve public API unless asked otherwise
- include a quick validation plan: what tests to add or run

## Final gate questions (answer them)
- Did we lower cognitive complexity?
- Did we improve readability for a new team member?
- Did we avoid introducing a new abstraction layer unless it clearly pays off?
- Is duplication left in place for good reasons (different reasons to change)?

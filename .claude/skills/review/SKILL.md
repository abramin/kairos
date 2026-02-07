# Code Review Skill
1. Analyze the specified area (frontend/backend) for issues
2. Present findings in a numbered list with severity
3. Wait for user approval before implementing fixes
4. After fixes: verify build passes, run all tests, grep for stale imports/references
5. Check that API response serialization matches any domain model changes

# Skill: Inverted Testing Pyramid Coverage Review

## Purpose
Audit a codebase’s test strategy using an **inverted testing pyramid**:

1. Contract/E2E tests (highest value, user-visible behavior)
2. Integration tests (real boundaries: DB, HTTP, queues, storage, browser APIs)
3. Component/API contract tests (thin seams)
4. Unit tests (only for critical invariants)

Goal: identify **coverage opportunities** that improve confidence per test-cost, not just increase test count.

---

## Inputs
- Repository source code
- Existing test suites
- API specs / contracts (OpenAPI, protobuf, GraphQL, Gherkin, etc.)
- CI config and test reports (if available)
- Known critical user journeys

Optional:
- Production incident list
- Flaky test history
- Code ownership map

---

## Review Rules
- Prefer behavior-visible and boundary tests over implementation-detail unit tests.
- Avoid mock-heavy tests when real dependencies are feasible.
- Treat contracts as authoritative. If code and contract diverge, note explicitly.
- Classify each recommendation by **risk**, **confidence gain**, and **effort**.
- Focus first on paths that can cause user harm, data corruption, auth/security failures, or money-impacting issues.

---

## Process

### Step 1: Inventory and classify tests
Create a complete test inventory and classify each test as:
- `contract_e2e`
- `integration`
- `component_contract`
- `unit`

For each, capture:
- file/path
- feature/flow covered
- dependencies used (real/mock)
- speed (slow/medium/fast)
- flakiness signals
- current CI stage

### Step 2: Build behavior map
List top user-visible flows and system boundaries.
Examples:
- auth/login/session refresh
- checkout/payment
- create/update/delete lifecycle
- async job processing
- external API integration
- permission/role checks
- failure/retry/idempotency paths

Map flows to existing tests and mark:
- fully covered
- partially covered
- not covered

### Step 3: Gap analysis by pyramid level
For each flow, identify missing tests at each level:
1. Missing contract/E2E behavior assertions
2. Missing integration tests across real boundaries
3. Missing API/component contracts
4. Missing unit invariants (only where justified)

### Step 4: Mock audit
Identify tests where mocks reduce confidence.
Flag opportunities to replace mocks with:
- ephemeral DB/container
- local queue/storage
- fake-but-protocol-accurate test servers
- clock injection (instead of global time mocking)

### Step 5: Risk-prioritized recommendations
Produce a ranked backlog with:
- recommendation
- pyramid level
- affected flow
- risk addressed
- expected confidence gain (High/Med/Low)
- effort (S/M/L)
- suggested test shape and location
- “why now”

### Step 6: Minimal migration plan
Provide a 2-4 week phased plan:
- Week 1: highest risk + fastest wins
- Week 2+: deeper integration/contract hardening
Include explicit de-prioritization of low-value tests.

---

## Required Output Format

## A) Coverage Scorecard
- Contract/E2E: X%
- Integration: X%
- Component/Contract: X%
- Unit: X%
- Overall confidence posture: Weak / Medium / Strong

(Use qualitative scoring if exact

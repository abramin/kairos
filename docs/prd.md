## Product Requirements Document

## Project: CLI Project Planner and Session Recommender

## Version: v1 (build now), v2 (planned)

### 1) Objective

Build a CLI tool that answers:

* “I have X minutes now, what should I do?”

It recommends session blocks across projects using:

* hard deadlines
* on-track status
* anti-cram spacing
* variation across projects

So you can work secondary goals (for example philosophy) without guilt when high-priority work is on track.

---

### 2) Primary User

Single user (Alex).
No multi-user in v1.

---

### 3) Scope

#### In v1

* Project/node/work-item management
* Template-based project scaffolding (including upfront generation)
* Session logging with unit-based progress
* Status and on-track analysis
* `what-now --minutes X` recommendation engine
* Replan by explicit command
* Rich CLI output (color + structured views)

#### Out of scope in v1

* Calendar sync
* Continuous background replanning daemon
* Collaboration
* Web/mobile UI

---

### 4) Key Product Rules

1. **No continuous background auto-rescheduling. Replanning is user-triggered or command-triggered.**

2. **Balance with constraints**

   * If critical deadline risk exists, prioritize that work.
   * If critical work is on track, mix in secondary projects.

3. **Track both time and scope**

   * Time logged (minutes)
   * Units completed (pages/chapters/exercises)

4. **Templates first**

   * Repeated structures (Calimove 19 weeks, OU week patterns) should scaffold upfront.

---

### 5) Domain Model

## 5.1 Entities

### Project

* `id`
* `name`
* `domain`
* `start_date`
* `target_date` (optional)
* `status` (`active|paused|done|archived`)
* `archived_at` (nullable)

### PlanNode (tree)

* `id`
* `project_id`
* `parent_id` (nullable)
* `title`
* `kind` (`week|module|book|stage|section|generic`)
* `order_index`
* constraints: `due_date`, `not_before`, `not_after` (nullable)
* `planned_min_budget` (nullable)

### WorkItem (schedulable)

* `id`
* `node_id`
* `title`
* `type`
* `status` (`todo|in_progress|done|skipped|archived`)
* `archived_at` (nullable)

Duration:

* `duration_mode` (`fixed|estimate|derived`)
* `planned_min`
* `logged_min`
* `duration_source` (`manual|template|rollup`)
* `estimate_confidence` (0..1)

Session policy:

* `min_session_min`
* `max_session_min`
* `default_session_min`
* `splittable` (bool)

Scope progress:

* `units_kind` (pages, chapters, drills, etc.)
* `units_total`
* `units_done`

Constraints:

* `due_date` (nullable)
* `not_before` (nullable)

### Dependency

* `predecessor_work_item_id`
* `successor_work_item_id`

### WorkSessionLog

* `id`
* `work_item_id`
* `started_at`
* `minutes`
* `units_done_delta`
* `note` (optional)

### Template

* `id`
* `name`
* `domain`
* `version`
* `config_json` (scaffold rules)

### UserProfile

* `buffer_pct`
* scoring weights
* default display prefs

---

### 6) Duration, Progress, and Completion Rules

## 6.1 Duration precedence

1. Fixed duration wins.
2. Child WorkItem estimates override parent budget.
3. Parent budget is a target/constraint.
4. If child sum exceeds budget, warn.

## 6.2 Progress recording

After each session:

* `logged_min += minutes`
* `units_done += units_done_delta`

Re-estimation (when units available):

* Compute implied total time from pace.
* Smooth update: `new_planned = 0.7*old + 0.3*implied`
* Never hard-jump unless user requests.

## 6.3 Completion

* Structural completion from statuses.
* Time completion from planned vs logged.
* Project progress is **time-weighted** by default.

---

### 7) Scheduling and Recommendation

## 7.1 Modes

### Balanced mode (default)

Used when no critical risk:

* respect urgency
* enforce spacing
* enforce variation across projects
* allow secondary projects when primary is on track

### Critical mode

Triggered when due horizon and remaining load indicate risk:

* focus only critical project/node/items until risk drops

## 7.2 what-now behavior

`what-now --minutes 60`

* Pick candidate WorkItems where min session fits.
* Score by urgency + slip + spacing + variation.
* Allocate within session bounds.
* Fill leftover with next best candidate.
* Return reasons for each suggestion.

---

### 8) Replanning Model

* No background daemon.
* Replanning occurs on:

  * `what-now`
  * `status --recalc`
  * explicit `replan` command
  * mutation commands (optional immediate recalculation flag)

---

### 9) CLI UX Requirements (v1)

Rich, colored, readable output (TTY-aware, no color in non-interactive mode).

Views required:

1. **Projects list**

   * name, due date, status, on-track state, progress bar

2. **Project inspect**

   * summary metrics
   * node tree
   * upcoming due items
   * risk flags

3. **Node inspect**

   * child nodes/items
   * budget vs planned vs logged

4. **WorkItem inspect**

   * session policy, units progress, logs, dependencies

5. **Status dashboard**

   * global at-risk/critical projects
   * available slack for secondary work

6. **what-now output**

   * proposed block(s), duration, explanation

Recommended stack:

* Python: `typer` + `rich` + `textual` optional later
  or Go: `cobra` + colored table/tree library

---

### 10) CRUD and Lifecycle Commands (v1)

#### Project

* `project add`
* `project update`
* `project archive` / `project unarchive`
* `project remove` (hard delete with `--force`)

#### Node

* `node add`
* `node update`
* `node archive/remove`

#### WorkItem

* `work add`
* `work update`
* `work done`
* `work archive/remove`

#### Logs

* `session log`
* `session remove`
* `session list`

#### Planning

* `what-now --minutes X`
* `status`
* `replan`

#### Templates

* `template list`
* `template show <name>`
* `project init --template <name> --start <date> [--due <date>]`

---

### 11) Template System (v1)

Templates scaffold **upfront** (full structure at creation time).

## 11.1 Template capabilities

* Create full node tree
* Create WorkItems with defaults
* Set session policies
* Set weekly/stage budgets
* Optional due-date offsets from project start

## 11.2 Example: Calimove 19 weeks

Template generates upfront:

* Week 1..19 nodes
* each week: target session range 2..4
* default session duration bounds (e.g. 35..60)
* WorkItems created upfront for planned sessions or minimum baseline + optional extras

## 11.3 Example: OU module template

* Week nodes
* reading WorkItems (units in pages/chapters)
* TMA phase WorkItems with due-date offsets

---

### 12) Success Criteria

1. Daily usage friction is low (fast input, clear output).
2. `what-now` recommendations are trustworthy and explainable.
3. Critical deadlines are protected.
4. Secondary projects are surfaced when safe.
5. Template scaffolding avoids repetitive data entry.

---

### 13) Acceptance Tests (minimum)

1. Critical switch test: due tomorrow + high remaining load => only critical project suggested.
2. Balanced test: primary on track => mixed recommendation includes secondary project.
3. Session bounds test: 60 min window never proposes sub-min chunks.
4. Unit-based re-estimation test: pace updates planned minutes smoothly.
5. Template upfront test: Calimove 19-week structure fully generated.
6. CRUD mutation test: deadline update changes risk and next recommendation.
7. Archive/remove test: archived items not recommended.

---

## v2 Roadmap (Planned)

### Ollama integration (assistive, not authoritative)

* Natural-language command parsing:

  * “I have 45 min, low energy, what now?”
* Plan explanation:

  * “Why not Spanish today?”
* Optional template generation from natural language:

  * “Create a 12-week exam prep project”

Guardrail:

* LLM proposes interpretation, deterministic engine executes final scheduling.

### Other v2 candidates

* Calendar integration
* Adaptive scoring from behavior
* Weekly review auto-report
* Optional TUI panels

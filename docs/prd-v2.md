This is a complete, developer-ready Product Requirements Document (PRD) for **Kairos v2.0**. It incorporates the shell-first philosophy, the visual requirements, and fully specifies the technical "missing pieces" (Schema, Algorithm, State Machine) identified in our previous exchange.

---

# Product Requirements Document: Kairos v2.0 (Shell-First)

**Version:** 2.1 (Technical Build-Ready)
**Date:** 2026-02-11
**Status:** Approved for Development
**Core Philosophy:** "Zero Memorization." The user navigates via fuzzy search, interactive menus, and visual cues. The application is a persistent shell, not a series of one-off commands.

---

## 1. System Architecture

### 1.1 Tech Stack

* **Language:** Go (Golang) 1.23+
* **Database:** SQLite 3 (via `mattn/go-sqlite3` or modern CGo-free alternative like `modernc.org/sqlite`).
* **UI Framework:** The "Charm" Stack is mandatory for the required aesthetic:
* **TUI Engine:** `github.com/charmbracelet/bubbletea` (Elm architecture for state).
* **Styling:** `github.com/charmbracelet/lipgloss` (Colors, borders, padding).
* **Forms:** `github.com/charmbracelet/huh` (Wizards for "Log" and "Create").
* **REPL/Prompt:** `github.com/c-bata/go-prompt` or `github.com/charmbracelet/bubbles/textinput` with custom autocomplete logic.


* **Config:** `spf13/viper` for `~/.kairos/config.yaml`.

### 1.2 Application Lifecycle

1. **Boot:** Load Config -> Open SQLite DB (Run Migrations) -> Initialize Global State.
2. **Shell Loop:** The app runs a continuous `bubbletea` program. It does not exit after a command unless explicitly told to.
3. **State Machine:**
* **View State:** `Dashboard` | `ProjectList` | `TaskList` | `Timer` | `Form` | `Recommendation`
* **Data State:** `ActiveProjectID` (nullable), `ActiveTaskID` (nullable), `UserConfig`.



---

## 2. Database Schema (SQLite)

To support the "What Now" engine and "low friction" entry, the schema uses a modified hierarchy where a project *always* has a root/default node hidden from the user if not needed.

```sql
-- 1. PROJECTS: The high-level buckets
CREATE TABLE projects (
    id TEXT PRIMARY KEY,                  -- UUID
    short_id TEXT UNIQUE NOT NULL,        -- Human-readable (e.g. "SPA26")
    name TEXT NOT NULL,
    domain TEXT NOT NULL,                 -- e.g. "education", "fitness"
    status TEXT DEFAULT 'active',         -- active, paused, archived, done
    start_date DATETIME NOT NULL,
    target_date DATETIME,                 -- Optional hard deadline
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    archived_at DATETIME
);

-- 2. PLAN NODES: Structural containers (Phases, Weeks, Modules)
-- Note: Every project has at least one "ROOT" node.
CREATE TABLE plan_nodes (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id TEXT REFERENCES plan_nodes(id), 
    title TEXT NOT NULL,
    kind TEXT DEFAULT 'generic',          -- stage, week, module, default
    is_default BOOLEAN DEFAULT 0,         -- IF TRUE, UI hides this node (Direct Project->Item feel)
    due_date DATETIME
);

-- 3. WORK ITEMS: The actual schedulable units
CREATE TABLE work_items (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES plan_nodes(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT DEFAULT 'todo',           -- todo, in_progress, done, skipped
    
    -- Estimation & Progress
    duration_mode TEXT DEFAULT 'estimate', -- fixed, estimate
    planned_min INTEGER DEFAULT 30,       -- The budget
    logged_min INTEGER DEFAULT 0,         -- Actual time spent
    
    -- Units (Optional for "Read 10 pages")
    units_total INTEGER DEFAULT 0,
    units_done INTEGER DEFAULT 0,
    units_kind TEXT,                      -- pages, chapters, reps

    -- Constraints
    due_date DATETIME,                    -- Overrides node due_date if set
    not_before DATETIME,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- 4. DEPENDENCIES: Blocking logic for "What Now"
CREATE TABLE dependencies (
    predecessor_item_id TEXT REFERENCES work_items(id),
    successor_item_id TEXT REFERENCES work_items(id),
    PRIMARY KEY (predecessor_item_id, successor_item_id)
);

-- 5. LOGS: The history needed for "Variation" scoring
CREATE TABLE work_session_logs (
    id TEXT PRIMARY KEY,
    work_item_id TEXT NOT NULL REFERENCES work_items(id),
    started_at DATETIME NOT NULL,
    minutes INTEGER NOT NULL,
    units_completed INTEGER DEFAULT 0,
    note TEXT
);

```

---

## 3. "What Now" Recommendation Algorithm

This is the core logic that answers the user's question. It runs purely on the current DB state.

### 3.1 Filtering (The "Can I do it?" Phase)

Query `work_items` where:

1. `status` is `todo` OR `in_progress`.
2. `project.status` is `active`.
3. `not_before` is NULL or `< NOW()`.
4. **Dependency Check:** NO `dependencies` exist where `predecessor.status != 'done'`.

### 3.2 Scoring (The "Should I do it?" Phase)

Each candidate item starts with **Score = 0**. Apply modifiers:

| Factor | Logic | Score Impact |
| --- | --- | --- |
| **Urgency** | `DaysUntilDue` < 2 | +50 points |
| **Urgency** | `DaysUntilDue` < 7 | +20 points |
| **Risk** | Project `(TimeLeft / WorkLeft)` < 1.0 (Behind Schedule) | +30 points |
| **Momentum** | Item status is `in_progress` | +15 points |
| **Staleness** | Last log for this *Project* was > 3 days ago | +10 points (Boosts variation) |
| **Burnout** | Last log for this *Project* was < 12 hours ago | -20 points (Enforces spacing) |
| **Fit** | `planned_min` <= `user_available_time` | +100 (Hard requirement filter optional) |

### 3.3 Output Structure

The engine returns a generic struct to the UI:

```go
type Recommendation struct {
    ItemID      string
    ProjectName string
    TaskTitle   string
    Reason      string  // e.g. "Critical Deadline in 2 days"
    Score       int
    Duration    int     // Suggested duration
}

```

---

## 4. Feature Specifications & UI Flows

### 4.1 The Dashboard (Home View)

**Visual:** Two-pane layout.

* **Top Header:** "Kairos Shell v2.0" | Status: Online | Today's Logged: 2h 15m.
* **Main Pane:** "Active Projects" table (Name, Progress Bar, Next Deadline).
* **Bottom Bar:** The `go-prompt` input.
* **Shortcuts:** `p` (Projects), `?` (What Now), `q` (Quit).

### 4.2 Project Navigation (`projects` command)

**Visual:** `bubbletea` List component.

* **Items:** All active projects.
* **Interaction:**
* `Enter`: Drill down into **Task View**.
* `/`: Trigger fuzzy search on project name.
* `n`: New Project Wizard (`huh` form).



### 4.3 Task View (Drill Down)

**Visual:** Tree or indented List view.

* **Items:** Nodes (bold) and Items (normal).
* **Interaction:**
* `Enter` on Task: Open **Action Menu**.
* `Space`: Toggle Done/Todo.
* `a`: Add new task (auto-associates with current node/project).



### 4.4 The Action Menu (Modal)

Triggered when selecting a task. A small popup window offering:

1. **Start Focus Timer** (Primary Action)
2. **Log Past Session**
3. **Edit Task Details**
4. **View Context** (Show parent node/project info)

### 4.5 Focus Timer Mode (`start` command)

**Visual:** Full screen takeover (Zen Mode).

* **Center:** Huge Digital Clock `24:59`.
* **Bottom:** Progress Bar of `TimeElapsed / EstimatedDuration`.
* **Controls:**
* `p`: Pause/Resume.
* `Enter`: **Stop & Log**.
* `Esc`: Cancel (No log).


* **Post-Stop Flow:**
* Popup: "Session Duration: 25m. Save? (Y/n)"
* Popup: "Mark 'Reading Chapter 1' as Done? (y/N)"
* *If No:* "Update remaining estimate? (Old: 30m) -> [ Input ]"



### 4.6 Manual Logging (`log` command)

**Visual:** `huh` Form Overlay.

* Uses a "Sentence Builder" pattern if possible:
* "I worked on [ Select Task ] for [ Duration ] on [ Today/Yesterday ]."


* **Validation:** Duration parses natural language ("1h 15m", "90m").

---

## 5. Development Roadmap (MVP)

### Phase 1: The Skeleton (Days 1-2)

* [ ] Set up `main.go` with `bubbletea`.
* [ ] Implement SQLite `init` and schema migration.
* [ ] Create the "Router" (Switch between Dashboard/List views).

### Phase 2: CRUD & Data (Days 3-4)

* [ ] Implement `project add` wizard.
* [ ] Implement `task add` (using Default Node logic).
* [ ] Build the `projects` list view with database fetching.

### Phase 3: The "What Now" Engine (Days 5-6)

* [ ] Write the Scoring Algorithm (Go struct logic).
* [ ] Build the `what-now` result view (Interactive List).

### Phase 4: Timer & Logging (Days 7-8)

* [ ] Build the Timer Model (using `time.Tick`).
* [ ] Implement the "Stop & Log" transaction logic.
* [ ] Polish the UI with `lipgloss` styles (Borders, Colors).

---

## 6. Appendix: User Interaction "Cheatsheet"

| Action | Shortcut / Command | Context |
| --- | --- | --- |
| **Find work** | `?` or `what-now` | Global |
| **Go to Project** | `p` or `projects` | Global |
| **Search Anything** | `Ctrl+P` (Fuzzy Finder) | Global |
| **Start Timer** | `start <task>` | Shell |
| **Quick Log** | `log <task>` | Shell |
| **Add Task** | `a` | Inside Project/Task List |
| **Back/Up** | `Esc` | Menus/Lists |
| **Quit App** | `Ctrl+C` or `exit` | Global |

This PRD provides the exact blueprint to build the Shell-First Kairos app. It resolves the "Node Tax" by introducing default nodes and focuses heavily on the `bubbletea` interactive components rather than static CLI flags.
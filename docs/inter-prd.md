This is a comprehensive Product Requirement Document (PRD) designed to overhaul your Shell Mode. You can hand this directly to an engineering team (or use it as a master prompt for an AI coding assistant).

---

# PRD: Interactive Shell & Wizard Standardization

**Project:** CLI Personal Productivity Tool

**Component:** Shell Mode (REPL)

**Date:** 2026-02-10

**Status:** Draft

## 1. Problem Statement

The current Shell Mode is **stateless** and **high-friction**. Users must memorize complex flag combinations (e.g., `log --project=LIT --item=4 --minutes=60`) or risk command failure. This breaks flow and feels like a "system administration" task rather than a productivity aid.

## 2. Objective

Implement a standard **"Progressive Disclosure"** pattern across all Shell Mode commands.

1. **Smart Context:** The shell remembers the user's "active focus" (Project/Item).
2. **Interactive Wizards:** If required arguments are missing, the system prompts for them interactively instead of erroring out.
3. **Visual Consistency:** All prompts must match the existing "Retro/Hacker" aesthetic (Teal/Amber).

---

## 3. User Experience (UX)

### 3.1 The "Wizard" Logic

For any command (e.g., `log`, `start`, `finish`):

1. **Check Context:** Is there an active Project/Item in the shell state? Use it as the default.
2. **Check Args:** Did the user type explicit flags? (e.g., `--time=60`). Use them.
3. **Prompt for Missing:** If data is still missing, trigger a TUI form (Wizard).

### 3.2 Visual Style

* **Library:** `charmbracelet/huh` (Go) for forms.
* **Theme:**
* **Background:** Transparent/Dark (matches terminal).
* **Accent Color:** `#F0A63F` (Amber/Orange) for active selection.
* **Secondary:** `#4CBF99` (Teal/Green) for success/confirmation.
* **Dimmed:** `#6C7A89` (Grey) for completed/inactive items.


* **Typography:** Monospace, consistent with existing headers.

### 3.3 The "Active Context" (New Concept)

The Shell must maintain a state struct:

```go
type ShellState struct {
    ActiveProjectID string // e.g., "LIT-NARR01"
    ActiveItemID    int    // e.g., 4
    LastDuration    int    // e.g., 60 (smart default for next log)
}

```

* **Updating Context:**
* Explicitly: `use LIT-NARR01`
* Implicitly: interacting with a project (e.g., `inspect LIT-NARR01`) sets it as active.



---

## 4. Command Behavior Specifications

| Command | Current Behavior | Target Behavior (Wizard) |
| --- | --- | --- |
| **`log`** | Fails without `--project`, `--item`, `--time` | 1. **Project:** Auto-fill from Context OR Prompt list.<br>

<br>2. **Item:** Auto-fill from Context OR Prompt list (filtered by Project).<br>

<br>3. **Time:** Prompt "Duration?" (Default: 60m). |
| **`start`** | Fails without ID | 1. **Project/Item:** Prompt hierarchy.<br>

<br>2. **Action:** Starts timer/sets status to "In Progress". |
| **`finish`** | Fails without ID | 1. **Item:** Prompt "Select active item to finish?"<br>

<br>2. **Action:** Sets status "Done". |
| **`context`** | *Does not exist* | **New Command.** Displays current active Project/Item. Allows clearing or setting manually. |

---

## 5. Technical Implementation Strategy

### 5.1 Architecture Refactor

Move from simple `cobra.Command` execution to a **Middleware Pattern**:

```go
// Pseudo-code wrapper for commands
func WizardMiddleware(cmd *cobra.Command, args []string, needed []Requirement) {
    // 1. Parse existing flags
    params := ParseFlags(cmd)

    // 2. Fill from Context
    if params.Project == "" { params.Project = ShellState.ActiveProjectID }

    // 3. Prompt for missing (The Wizard Loop)
    if params.Project == "" {
        params.Project = RunProjectSelector() // returns ID
        ShellState.ActiveProjectID = params.Project // Update Context
    }

    if params.Item == "" {
        params.Item = RunItemSelector(params.Project) // filtered list
    }

    // 4. Execute actual logic
    ExecuteLog(params)
}

```

### 5.2 Key Libraries

* **Forms:** `github.com/charmbracelet/huh`
* *Why:* Native accessibility, key-bindings (j/k navigation), and easy theming.


* **State Management:** Native Go `struct` passed into the Shell Loop.

---

## 6. Success Metrics

* **Reduced Keystrokes:** Logging a session should take ~3 keystrokes ( `log` -> `Enter` -> `Enter` ) instead of ~40.
* **Error Rate:** "Missing Flag" errors should drop to near zero.

---

## 7. Mockups

**Scenario: User types `log` (No args, No context)**

```text
> log

  ? Which Project?
    > Narrative, Identity, and Self-Conscious Fiction
      History of Psychology
      Spanish B1

  ? Which Item?
    > Read Don Quixote (Active)
      Read Tristram Shandy

  ? Duration?
    [ 60 ] min

  > Logged 60m to Don Quixote.

```

**Scenario: User types `log` (Context: Narrative Project is active)**

```text
> log

  (Skipped Project Selection - using "LIT-NARR01")

  ? Which Item?
    > Read Don Quixote
      Read Tristram Shandy
  ...

```
# PRD: Shell-First Kairos

## 1) Objective

Make the interactive shell (`kairos shell`) the primary and most complete way to use Kairos.

Shell-first means users should be able to create, plan, execute, review, and maintain projects without leaving the shell, while still keeping non-interactive CLI commands available for scripts/automation.

## 2) Why This Change

Current CLI usage is split between one-shot commands and shell mode. The shell already supports active project context and command pass-through, but it is not yet the default mental model or a fully guided workflow.

A shell-first product should optimize for:
- Fast repeated usage in a single session.
- Reduced argument repetition via context.
- Better discoverability and guidance.
- Safe execution for destructive operations.
- Full feature coverage without mode switching.

## 3) Scope

### In scope

- Shell as default interactive entrypoint.
- Full command parity for all user workflows.
- Shell-native context, guidance, and autocomplete.
- Shell-native safety controls (confirmation, dry-run where applicable).
- Shell-native creation/edit flows (guided where useful).
- Help and explainability inside shell.
- Robust shell UX for long sessions (history, prompt state, clear errors).

### Out of scope

- Replacing scriptable one-shot CLI usage.
- GUI/TUI dashboard rewrite.
- Cloud sync or multi-device sessions.

## 4) Primary Users

- Individual planners running Kairos daily in terminal.
- Power users managing multiple active projects.
- Users who need quick recommendation loops (`what-now`, `status`, `replan`) several times per day.

## 5) Product Principles

1. Context before repetition.
2. Interactive by default, explicit for automation.
3. Safe by default for destructive commands.
4. Deterministic outputs and predictable command behavior.
5. Graceful fallback to standard Cobra command execution.

## 6) Core User Stories

1. As a user, I can open Kairos and immediately land in shell mode.
2. As a user, I can pick an active project once and run most commands without repeating IDs.
3. As a user, I can discover commands and flags from inside shell without reading docs.
4. As a user, I can run all core CRUD, planning, and review workflows from shell.
5. As a user, I get clear, actionable errors when context is missing or input is invalid.
6. As a user, I get confirmation prompts for destructive actions.
7. As a user, I can still run non-interactive commands for scripts and CI.

## 7) Functional Requirements

### FR1: Shell-first entrypoint

- `kairos` with no arguments should launch shell mode by default.
- `kairos shell` remains available and equivalent.
- A non-interactive command (e.g. `kairos status`, `kairos project list`) must continue to work exactly as today.

### FR2: Session context model

Shell must maintain mutable session state:
- `activeProjectID` (required for context-scoped operations).
- `activeShortID` and display name for prompt rendering.
- mode state (normal, help-chat, draft/wizard).
- transient last-operation context (last recommended work item, last inspected project).

Rules:
- Explicit IDs/flags override active context.
- If no context is set for context-required command, shell provides guided recovery (`use <id>` suggestion).

### FR3: Command parity and shell-native shortcuts

All core capabilities available in one-shot CLI must be executable in shell.

Required shell-native shortcuts:
- `projects`
- `use <id|prefix>`
- `inspect [id]`
- `status [scope options]`
- `what-now [minutes]`
- `clear`
- `help`, `help chat`
- `exit` / `quit`

Required pass-through parity groups:
- `project *`
- `node *`
- `work *`
- `session *`
- `replan`
- `template *`
- `ask`, `explain`, `review`

### FR4: Completion and input ergonomics

- Autocomplete for shell-native commands.
- Context-aware suggestions for project identifiers and known argument patterns.
- Completion descriptions should explain command intent.
- Parser must support quoted args and escapes.
- Shell must handle malformed quoting with clear inline error messages.

### FR5: Guided workflows

For high-friction operations, shell should provide guided prompts when required flags are missing:
- Project creation/import/init.
- Work item creation.
- Session logging.
- Replan with optional scope/strategy choices.

Guided mode should produce a deterministic command preview before execution.

### FR6: Safety and confirmations

Destructive and irreversible actions require confirmation in shell:
- remove/archive operations where data loss risk exists.
- bulk mutation commands.

Requirements:
- explicit confirmation prompt with target summary.
- `--yes` still supported for automation/power usage.
- confirmation behavior must be consistent between shell-native and pass-through paths.

### FR7: Help and discoverability in-shell

- `help` lists shell-native commands and key pass-through examples.
- `help chat` provides interactive Q&A over command spec.
- Unknown command errors must provide nearest valid alternatives.
- On first shell launch, show a concise onboarding snippet (3-5 commands to get started).

### FR8: Prompt and status feedback

Prompt must always communicate context:
- no active project: `kairos ❯`
- active project: `kairos (<shortID>) ❯`
- special modes: `help>`, `draft>`

After context-changing commands, shell prints confirmation:
- active project set/cleared.
- mode entered/exited.

### FR9: Output consistency

- Shell uses same deterministic formatters as CLI commands.
- `status`, `what-now`, and inspect views remain readable and stable.
- Output should be TTY-aware and preserve non-color behavior where needed.

### FR10: Reliability and interruption handling

- Ctrl+C cancels the current operation without crashing shell.
- Ctrl+D / `exit` quits cleanly.
- Errors in one command do not corrupt session context.
- Shell should recover from transient service/repository errors and continue loop.

## 8) Non-Functional Requirements

- Startup latency to interactive prompt: <= 300ms on warm local DB.
- Command completion latency for cached project suggestions: <= 50ms target.
- Project suggestion cache TTL configurable (default 5s).
- No regressions in one-shot CLI command behavior.
- Full shell command behavior covered by deterministic tests.

## 9) Command Coverage Matrix

### Must be first-class in shell

- Project selection and inspection: `projects`, `use`, `inspect`.
- Daily execution loop: `what-now`, `session log`, `status`.
- Planning maintenance: `replan`, `work update/done`, `node update`.
- Intake/creation: `project init`, `project import`, `project draft`.

### Must remain script-friendly outside shell

- All existing Cobra command paths and flags.

## 10) UX Details

### Welcome and onboarding

On shell start, show:
- short brand banner.
- one-line explanation of active-context model.
- 3-5 starter commands.

### Error style

All errors should include:
- what failed.
- why (if known).
- next valid action.

### Help affordances

- `Tab`: autocomplete.
- `help`: concise command list.
- `help chat`: natural-language assistance.

## 11) Data and State Requirements

Shell session state is in-memory per process.

Persistence requirements:
- command history persisted across sessions.
- no persistent mutation of active project context required for v1 shell-first.

Future option (not required now): persisted last active project via local profile.

## 12) Migration Requirements

- Update docs to present shell as primary path.
- Keep existing one-shot command docs for automation users.
- Update examples and walkthroughs to start with `kairos`/`kairos shell` first.

## 13) Acceptance Criteria

1. Running `kairos` with no args enters shell and displays welcome text.
2. User can complete end-to-end workflow entirely in shell:
   1. list projects
   2. select project
   3. inspect
   4. run `what-now`
   5. log session
   6. run `status`
   7. run `replan`
3. Every existing command path is executable from shell via pass-through.
4. Context-dependent shell commands use active project when no explicit ID is provided.
5. Missing context produces guided error with recovery hint.
6. Destructive actions trigger confirmation unless `--yes` provided.
7. Autocomplete suggests project IDs/names for `use` and `inspect`.
8. Shell remains responsive after invalid input and service errors.
9. Existing non-shell command behavior remains backward compatible.

## 14) Test Plan

- Unit tests:
  - shell arg parsing and quoting.
  - command routing (native vs pass-through).
  - context resolution precedence.
  - confirmation gating.
  - completer suggestion correctness.

- Integration tests:
  - multi-command shell session with context switching.
  - full daily loop scenario.
  - error recovery and continued command execution.

- Regression tests:
  - one-shot CLI behavior unchanged.
  - formatter outputs stable for status/what-now/inspect.

## 15) Rollout Plan

### Phase 1: Parity hardening

- Ensure all core commands behave correctly inside shell.
- Fill shell gaps (guided prompts, confirmations, better errors).

### Phase 2: Shell-default launch

- Make `kairos` enter shell when no args.
- Keep explicit one-shot invocation unchanged.

### Phase 3: Discoverability and polish

- Improve onboarding and contextual hints.
- Expand autocomplete and command suggestion quality.
- Add usage telemetry hooks (optional) for shell command adoption.

## 16) Risks and Mitigations

- Risk: Shell behavior diverges from CLI behavior.
  - Mitigation: route through shared Cobra/service paths wherever possible.

- Risk: Added interactivity hurts automation users.
  - Mitigation: only default to shell on zero args; preserve explicit command paths.

- Risk: Confirmation prompts become inconsistent.
  - Mitigation: centralize confirmation policy and test across command families.

## 17) Open Decisions

1. Should the last active project persist across shell restarts?
2. Should `status` and `what-now` default to active project only, or global scope with active-project priority?
3. Should guided wizards be opt-in (`--interactive`) or automatic on missing required flags?

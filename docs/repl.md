---

**Context & Objective**
I am building "Kairos," a CLI Project Planner in Go using `cobra`. I currently have standard commands (e.g., `kairos project list`, `kairos project inspect`).

I want to implement a **REPL / Interactive Shell** mode (`kairos shell`) that mimics the feel of `psql` or an advanced router CLI. This shell should allow the user to maintain state (like a "currently selected project") to avoid repetitive typing.

**Tech Stack Requirements**

* **Shell Logic:** Use `github.com/c-bata/go-prompt` for the REPL loop, history, and autocomplete.
* **Styling:** Use `github.com/charmbracelet/lipgloss` for all output.
* **Forms/Interaction:** Use `github.com/charmbracelet/bubbletea` if a command requires complex input (like creating a project).

**Visual Design Guidelines (Strict)**
The shell must look "High-End Dashboard" style (Tokyo Night / Gruvbox palette).

* **Prompt:** Should not be plain text. It should be a styled glyph, e.g., `kairos ❯`.
* **Context:** If a project is selected, the prompt changes to: `kairos (proj: 9e1d55) ❯` with the project ID dimmed.
* **Tables:** Use rounded borders, header rows with distinct colors, and padding.
* **Colors:** Soft purples (`#9d7cd8`), blues (`#7aa2f7`), and muted greens (`#73daca`). Avoid harsh defaults.

**Functional Requirements**

1. **Global State:** Implement a struct to hold the session state (e.g., `ActiveProjectID`).
2. **Commands to Implement inside the Shell:**
* `projects`: Renders the high-fidelity project list table (reuse existing render logic if possible).
* `use <partial_id>`: Sets the `ActiveProjectID`. It should support fuzzy matching or at least prefix matching.
* `inspect`:
* If `ActiveProjectID` is set: Inspects that project.
* If no ID set: Returns a styled error asking for an ID.
* `inspect <id>`: Inspects a specific project regardless of context.


* `clear`: Clears the screen.
* `what-now`: Runs the session recommendation engine for the *active* project.
* `exit`: Quits the shell cleanly.


3. **Autocomplete (IntelliSense):**
* When typing `use` or `inspect`, the shell should dynamically suggest available Project IDs and Names from the database/storage.
* When typing commands, offer descriptions (e.g., "what-now: Generate next session block").



**Deliverables**
Please write the Go code for `cmd/shell.go`.

* Include the `Executor` (logic) and `Completer` (suggestions).
* Include a `RenderWelcomeMessage()` function using Lipgloss to show a banner when the shell starts.
* Show how to bridge the existing Cobra commands so I don't have to duplicate the logic (e.g., wrapping `RunListProjects`).

---
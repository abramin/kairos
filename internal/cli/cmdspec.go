package cli

import (
	"encoding/json"
	"strings"
)

// CommandSpec is a structured representation of all shell commands,
// used as LLM grounding context for help and intent parsing.
type CommandSpec struct {
	Commands []CommandEntry `json:"commands"`
}

// CommandEntry describes a single command or subcommand.
type CommandEntry struct {
	FullPath    string      `json:"full_path"`
	Short       string      `json:"short"`
	Flags       []FlagEntry `json:"flags,omitempty"`
	Examples    string      `json:"examples,omitempty"`
	Subcommands []string    `json:"subcommands,omitempty"`
}

// FlagEntry describes a single flag on a command.
type FlagEntry struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ShellCommandSpec returns a static CommandSpec describing all available
// shell commands. This replaces the old Cobra tree walk.
func ShellCommandSpec() *CommandSpec {
	return &CommandSpec{
		Commands: []CommandEntry{
			{FullPath: "projects", Short: "List all projects"},
			{FullPath: "use", Short: "Set active project context", Flags: []FlagEntry{{Name: "id", Type: "string", Description: "Project short ID or UUID"}}},
			{FullPath: "inspect", Short: "Show project tree for active project"},
			{FullPath: "status", Short: "Show status overview across all projects"},
			{FullPath: "what-now", Short: "Get work recommendations for available time", Flags: []FlagEntry{{Name: "minutes", Type: "int", Default: "60", Description: "Available minutes"}}},
			{FullPath: "log", Short: "Log a completed work session", Flags: []FlagEntry{{Name: "item", Type: "string", Description: "Work item ref (#N or ID)"}, {Name: "minutes", Type: "int", Description: "Duration in minutes"}}},
			{FullPath: "start", Short: "Start working on an item (sets status to in-progress)"},
			{FullPath: "finish", Short: "Mark a work item as done"},
			{FullPath: "add", Short: "Quick-add a work item to active project"},
			{FullPath: "replan", Short: "Rebalance project schedules", Flags: []FlagEntry{{Name: "strategy", Type: "string", Default: "rebalance", Description: "Replan strategy (rebalance|deadline_first)"}}},
			{FullPath: "import", Short: "Import a project from a JSON file"},
			{FullPath: "draft", Short: "Start interactive project drafting wizard"},
			{FullPath: "context", Short: "Show or set active project/item context"},
			{FullPath: "help", Short: "Show available commands"},
			{FullPath: "help chat", Short: "Interactive LLM-powered help session"},
			{FullPath: "ask", Short: "Ask a natural language question (LLM)", Flags: []FlagEntry{{Name: "question", Type: "string", Description: "Natural language question"}}},
			{FullPath: "explain now", Short: "Explain current recommendations with LLM narrative"},
			{FullPath: "explain why-not", Short: "Explain why a specific item was not recommended"},
			{FullPath: "review weekly", Short: "Summarize the past 7 days with actionable insights"},
			// Entity group commands
			{FullPath: "project list", Short: "List all projects", Flags: []FlagEntry{{Name: "all", Type: "bool", Description: "Include archived projects"}}},
			{FullPath: "project inspect", Short: "Show project tree"},
			{FullPath: "project add", Short: "Create a new project", Flags: []FlagEntry{{Name: "id", Type: "string", Description: "Short ID", Required: true}, {Name: "name", Type: "string", Description: "Project name", Required: true}, {Name: "domain", Type: "string", Description: "Domain", Required: true}, {Name: "start", Type: "string", Description: "Start date (YYYY-MM-DD)", Required: true}, {Name: "due", Type: "string", Description: "Due date (YYYY-MM-DD)"}}},
			{FullPath: "project update", Short: "Update project fields"},
			{FullPath: "project archive", Short: "Archive a project"},
			{FullPath: "project unarchive", Short: "Unarchive a project"},
			{FullPath: "project remove", Short: "Delete a project"},
			{FullPath: "project init", Short: "Initialize project from template", Flags: []FlagEntry{{Name: "template", Type: "string", Description: "Template reference", Required: true}, {Name: "id", Type: "string", Description: "Short ID", Required: true}, {Name: "name", Type: "string", Description: "Project name", Required: true}, {Name: "start", Type: "string", Description: "Start date", Required: true}}},
			{FullPath: "project import", Short: "Import project from JSON file"},
			{FullPath: "project draft", Short: "Start interactive project drafting"},
			{FullPath: "node add", Short: "Create a new plan node", Flags: []FlagEntry{{Name: "project", Type: "string", Description: "Project ID"}, {Name: "title", Type: "string", Description: "Node title", Required: true}, {Name: "kind", Type: "string", Description: "Node kind (module|milestone|week)", Required: true}}},
			{FullPath: "node inspect", Short: "Show node details"},
			{FullPath: "node update", Short: "Update node fields"},
			{FullPath: "node remove", Short: "Delete a plan node"},
			{FullPath: "work add", Short: "Create a new work item", Flags: []FlagEntry{{Name: "node", Type: "string", Description: "Parent node ID", Required: true}, {Name: "title", Type: "string", Description: "Item title", Required: true}, {Name: "type", Type: "string", Description: "Item type (task|reading|exercise|zettel)", Required: true}, {Name: "planned-min", Type: "int", Description: "Planned minutes"}, {Name: "due-date", Type: "string", Description: "Due date (YYYY-MM-DD)"}}},
			{FullPath: "work inspect", Short: "Show work item details"},
			{FullPath: "work update", Short: "Update work item fields"},
			{FullPath: "work done", Short: "Mark work item as done"},
			{FullPath: "work archive", Short: "Archive a work item"},
			{FullPath: "work remove", Short: "Delete a work item"},
			{FullPath: "session log", Short: "Log a work session", Flags: []FlagEntry{{Name: "work-item", Type: "string", Description: "Work item ID", Required: true}, {Name: "minutes", Type: "int", Description: "Duration in minutes", Required: true}, {Name: "note", Type: "string", Description: "Session note"}, {Name: "units-done", Type: "int", Description: "Units completed"}}},
			{FullPath: "session list", Short: "List recent sessions", Flags: []FlagEntry{{Name: "work-item", Type: "string", Description: "Filter by work item"}, {Name: "days", Type: "int", Default: "7", Description: "Number of days"}}},
			{FullPath: "session remove", Short: "Delete a session"},
			{FullPath: "template list", Short: "List available templates"},
			{FullPath: "template show", Short: "Show template details"},
			{FullPath: "clear", Short: "Clear the screen"},
			{FullPath: "exit", Short: "Exit the shell"},
		},
	}
}

// ValidateCommandPath checks that a command path exists in the spec.
func (spec *CommandSpec) ValidateCommandPath(path string) bool {
	for _, cmd := range spec.Commands {
		if cmd.FullPath == path {
			return true
		}
	}
	return false
}

// ValidateFlag checks that a flag exists for a given command path.
func (spec *CommandSpec) ValidateFlag(cmdPath, flagName string) bool {
	for _, cmd := range spec.Commands {
		if cmd.FullPath != cmdPath {
			continue
		}
		for _, f := range cmd.Flags {
			if f.Name == flagName {
				return true
			}
		}
		return false
	}
	return false
}

// FindCommand returns the CommandEntry for a given path, or nil.
func (spec *CommandSpec) FindCommand(path string) *CommandEntry {
	for i := range spec.Commands {
		if spec.Commands[i].FullPath == path {
			return &spec.Commands[i]
		}
	}
	return nil
}

// FuzzyMatch returns up to n commands whose paths or descriptions
// contain any of the query terms (case-insensitive).
func (spec *CommandSpec) FuzzyMatch(query string, n int) []CommandEntry {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	type scored struct {
		entry CommandEntry
		hits  int
	}

	var matches []scored
	for _, cmd := range spec.Commands {
		lowerPath := strings.ToLower(cmd.FullPath)
		lowerShort := strings.ToLower(cmd.Short)
		hits := 0
		for _, term := range terms {
			if strings.Contains(lowerPath, term) || strings.Contains(lowerShort, term) {
				hits++
			}
		}
		if hits > 0 {
			matches = append(matches, scored{entry: cmd, hits: hits})
		}
	}

	// Sort by hit count descending (simple selection for small N).
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].hits > matches[i].hits {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	result := make([]CommandEntry, 0, n)
	for i := 0; i < len(matches) && i < n; i++ {
		result = append(result, matches[i].entry)
	}
	return result
}

// SerializeCommandSpec serializes the spec to compact JSON for embedding in LLM prompts.
func SerializeCommandSpec(spec *CommandSpec) string {
	data, err := json.Marshal(spec)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// BuildValidationMaps pre-computes lookup maps from a CommandSpec for use
// by grounding validation. Returns (validCommands, validFlags).
func BuildValidationMaps(spec *CommandSpec) (map[string]bool, map[string]map[string]bool) {
	cmds := make(map[string]bool, len(spec.Commands))
	flags := make(map[string]map[string]bool, len(spec.Commands))

	for _, cmd := range spec.Commands {
		cmds[cmd.FullPath] = true
		if len(cmd.Flags) > 0 {
			fm := make(map[string]bool, len(cmd.Flags))
			for _, f := range cmd.Flags {
				fm[f.Name] = true
			}
			flags[cmd.FullPath] = fm
		}
	}

	return cmds, flags
}

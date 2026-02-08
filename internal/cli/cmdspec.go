package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandSpec is a structured representation of the full Cobra command tree,
// generated at runtime for use as LLM grounding context.
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

// BuildCommandSpec walks the Cobra command tree and produces a CommandSpec.
func BuildCommandSpec(root *cobra.Command) *CommandSpec {
	spec := &CommandSpec{}
	walkCommand(root, spec)
	return spec
}

func walkCommand(cmd *cobra.Command, spec *CommandSpec) {
	entry := CommandEntry{
		FullPath: cmd.CommandPath(),
		Short:    cmd.Short,
		Examples: cmd.Example,
	}

	// Collect flags (local only, not inherited).
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		fe := FlagEntry{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		}
		if ann, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && len(ann) > 0 {
			fe.Required = true
		}
		entry.Flags = append(entry.Flags, fe)
	})

	// Collect child command names.
	for _, child := range cmd.Commands() {
		if !shouldIncludeCommand(child) {
			continue
		}
		entry.Subcommands = append(entry.Subcommands, child.CommandPath())
	}

	spec.Commands = append(spec.Commands, entry)

	// Recurse into children.
	for _, child := range cmd.Commands() {
		if !shouldIncludeCommand(child) {
			continue
		}
		walkCommand(child, spec)
	}
}

func shouldIncludeCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	// Cobra may mark custom help commands hidden; keep them in the help spec.
	if cmd.Name() == "help" {
		return true
	}
	return !cmd.Hidden && cmd.IsAvailableCommand()
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

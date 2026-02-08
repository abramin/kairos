package intelligence

import (
	"strings"
)

// ValidateHelpGrounding filters a HelpAnswer to remove any examples or
// next_commands that reference commands or flags not present in the spec.
// Returns the cleaned answer and whether any items were removed.
func ValidateHelpGrounding(answer *HelpAnswer, validCmds map[string]bool, validFlags map[string]map[string]bool) (*HelpAnswer, bool) {
	if answer == nil {
		return &HelpAnswer{Source: "deterministic"}, true
	}

	anyRemoved := false

	// Filter examples.
	var cleanExamples []ShellExample
	for _, ex := range answer.Examples {
		cmdPath, flags := parseCommandString(ex.Command)
		if cmdPath == "" || !validCmds[cmdPath] {
			anyRemoved = true
			continue
		}
		// Check flags if the command has a known flag set.
		if fm, ok := validFlags[cmdPath]; ok {
			valid := true
			for _, f := range flags {
				if !fm[f] {
					valid = false
					break
				}
			}
			if !valid {
				anyRemoved = true
				continue
			}
		}
		cleanExamples = append(cleanExamples, ex)
	}
	answer.Examples = cleanExamples

	// Filter next_commands.
	var cleanNext []string
	for _, nc := range answer.NextCommands {
		cmdPath, _ := parseCommandString(nc)
		if cmdPath == "" || !validCmds[cmdPath] {
			anyRemoved = true
			continue
		}
		cleanNext = append(cleanNext, nc)
	}
	answer.NextCommands = cleanNext

	// If grounding stripped all actionable output, treat this as deterministic fallback mode.
	if len(answer.Examples) == 0 && len(answer.NextCommands) == 0 && answer.Source == "llm" {
		answer.Source = "deterministic"
	}

	return answer, anyRemoved
}

// parseCommandString extracts the command path and flag names from a shell
// command string like "kairos session log --work-item abc --minutes 30".
// Returns the longest matching command path and a list of flag names.
func parseCommandString(cmd string) (string, []string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", nil
	}

	// Build the command path: take all leading non-flag tokens.
	var pathParts []string
	var flags []string
	inFlags := false

	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			inFlags = true
		}
		if inFlags {
			if strings.HasPrefix(p, "--") {
				name := strings.TrimPrefix(p, "--")
				// Strip =value if present.
				if idx := strings.Index(name, "="); idx >= 0 {
					name = name[:idx]
				}
				flags = append(flags, name)
			} else if strings.HasPrefix(p, "-") && len(p) == 2 {
				// Short flag — we don't validate these since the spec
				// stores long names; skip.
			}
			// Skip flag values.
		} else {
			pathParts = append(pathParts, p)
		}
	}

	cmdPath := strings.Join(pathParts, " ")
	return cmdPath, flags
}

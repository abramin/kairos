package intelligence

import (
	"fmt"
	"strings"
)

// DeterministicHelp produces a help answer without LLM by fuzzy-matching
// the question against the command spec and glossary.
func DeterministicHelp(question string, commands []HelpCommandInfo) *HelpAnswer {
	terms := strings.Fields(strings.ToLower(question))
	if len(terms) == 0 {
		return defaultHelpAnswer(commands)
	}

	// Score each command by how many query terms match its path or description.
	type scored struct {
		cmd  HelpCommandInfo
		hits int
	}
	var matches []scored
	for _, cmd := range commands {
		lowerPath := strings.ToLower(cmd.FullPath)
		lowerShort := strings.ToLower(cmd.Short)
		hits := 0
		for _, term := range terms {
			if strings.Contains(lowerPath, term) || strings.Contains(lowerShort, term) {
				hits++
			}
		}
		if hits > 0 {
			matches = append(matches, scored{cmd: cmd, hits: hits})
		}
	}

	// Sort by hits descending.
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].hits > matches[i].hits {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Check glossary for concept matches.
	var glossaryHits []string
	for term, def := range HelpGlossary {
		for _, q := range terms {
			if strings.Contains(strings.ToLower(term), q) || strings.Contains(q, strings.ToLower(term)) {
				glossaryHits = append(glossaryHits, fmt.Sprintf("%s: %s", term, def))
				break
			}
		}
	}

	// Build the answer.
	var answer strings.Builder
	if len(glossaryHits) > 0 {
		for i, g := range glossaryHits {
			if i > 0 {
				answer.WriteString("\n\n")
			}
			answer.WriteString(g)
		}
	}
	if len(matches) > 0 {
		if answer.Len() > 0 {
			answer.WriteString("\n\n")
		}
		answer.WriteString("Relevant commands:")
	} else if answer.Len() == 0 {
		answer.WriteString("I couldn't find an exact match. Try 'kairos help <command>' for details on a specific command.")
	}

	// Build examples from top matches.
	var examples []ShellExample
	limit := 3
	if len(matches) < limit {
		limit = len(matches)
	}
	for i := 0; i < limit; i++ {
		cmd := matches[i].cmd
		examples = append(examples, ShellExample{
			Command:     cmd.FullPath,
			Description: cmd.Short,
		})
	}

	// Suggest safe next commands.
	var nextCmds []string
	safeDefaults := []string{"kairos status", "kairos what-now", "kairos help"}
	for _, s := range safeDefaults {
		nextCmds = append(nextCmds, s)
	}

	return &HelpAnswer{
		Answer:       strings.TrimSpace(answer.String()),
		Examples:     examples,
		NextCommands: nextCmds,
		Confidence:   1.0,
		Source:       "deterministic",
	}
}

func defaultHelpAnswer(commands []HelpCommandInfo) *HelpAnswer {
	var examples []ShellExample
	// Show a few top-level commands as defaults.
	defaults := []string{"kairos what-now", "kairos status", "kairos project list", "kairos session log"}
	for _, d := range defaults {
		for _, cmd := range commands {
			if cmd.FullPath == d {
				examples = append(examples, ShellExample{
					Command:     cmd.FullPath,
					Description: cmd.Short,
				})
				break
			}
		}
	}

	return &HelpAnswer{
		Answer:       "Here are some common commands to get started. Use 'kairos help <command>' for details on any command.",
		Examples:     examples,
		NextCommands: []string{"kairos help", "kairos status", "kairos what-now"},
		Confidence:   1.0,
		Source:       "deterministic",
	}
}

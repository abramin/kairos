package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	tea "github.com/charmbracelet/bubbletea"
)

// destructiveCommands maps command groups to subcommands that require confirmation.
var destructiveCommands = map[string]map[string]bool{
	"project": {"remove": true, "archive": true},
	"node":    {"remove": true},
	"work":    {"remove": true, "archive": true},
	"session": {"remove": true},
}

func RunShell(app *App) error {
	m := newAppModel(app)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// shellError formats an error for display in the shell.
func shellError(err error) string {
	return formatter.StyleRed.Render(fmt.Sprintf("Error: %v", err))
}

// parseDurationArg parses a duration string into minutes.
// Accepted formats: "120" (bare minutes), "2h", "30m", "1h30m".
// Returns (minutes, true) on success, (0, false) if not a valid duration.
func parseDurationArg(s string) (int, bool) {
	if s == "" {
		return 0, false
	}

	// Bare integer → minutes.
	if v, err := strconv.Atoi(s); err == nil {
		if v > 0 {
			return v, true
		}
		return 0, false
	}

	s = strings.ToLower(s)
	total := 0

	// Try NhNm, Nh, Nm patterns.
	if hi := strings.Index(s, "h"); hi >= 0 {
		h, err := strconv.Atoi(s[:hi])
		if err != nil || h < 0 {
			return 0, false
		}
		total += h * 60
		s = s[hi+1:]
	}
	if len(s) == 0 {
		if total > 0 {
			return total, true
		}
		return 0, false
	}
	if mi := strings.Index(s, "m"); mi >= 0 && mi+1 == len(s) {
		m, err := strconv.Atoi(s[:mi])
		if err != nil || m < 0 {
			return 0, false
		}
		total += m
		if total > 0 {
			return total, true
		}
		return 0, false
	}

	return 0, false
}

// splitShellArgs splits a shell input string into tokens, respecting quotes and escapes.
func splitShellArgs(input string) ([]string, error) {
	var parts []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false
	tokenStarted := false

	flush := func() {
		parts = append(parts, cur.String())
		cur.Reset()
		tokenStarted = false
	}

	for _, r := range input {
		if escaped {
			cur.WriteRune(r)
			tokenStarted = true
			escaped = false
			continue
		}

		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
			tokenStarted = true
			continue
		}

		if inDouble {
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
			tokenStarted = true
			continue
		}

		switch r {
		case '\\':
			escaped = true
			tokenStarted = true
		case '\'':
			inSingle = true
			tokenStarted = true
		case '"':
			inDouble = true
			tokenStarted = true
		case ' ', '\t', '\n', '\r':
			if tokenStarted {
				flush()
			}
		default:
			cur.WriteRune(r)
			tokenStarted = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if tokenStarted {
		flush()
	}

	return parts, nil
}


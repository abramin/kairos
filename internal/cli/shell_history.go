package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const maxHistoryLines = 500

// shellHistoryPath returns the path to the shell history file.
func shellHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kairos", "shell_history")
}

// loadShellHistory reads command history from the default path.
func loadShellHistory() []string {
	path := shellHistoryPath()
	if path == "" {
		return nil
	}
	return loadHistoryFromPath(path)
}

// loadHistoryFromPath reads command history from the given file.
// Returns nil if the file does not exist or cannot be read.
func loadHistoryFromPath(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Keep only the most recent entries.
	if len(lines) > maxHistoryLines {
		lines = lines[len(lines)-maxHistoryLines:]
	}
	return lines
}

// appendShellHistory appends a single line to the default history file.
func appendShellHistory(line string) {
	path := shellHistoryPath()
	if path == "" {
		return
	}
	appendHistoryToPath(path, line)
}

// appendHistoryToPath appends a single line to the given history file.
// Errors are silently ignored — history is best-effort.
func appendHistoryToPath(path, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.WriteString(line + "\n")
}

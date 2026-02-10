package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadShellHistory_FileNotFound_ReturnsNil(t *testing.T) {
	// loadShellHistory uses the real home dir path, but we can test the
	// function by verifying that a non-existent file returns nil gracefully.
	// For isolation, we'll test the core logic via a temp dir.
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "shell_history")
	lines := loadHistoryFromPath(path)
	assert.Nil(t, lines)
}

func TestLoadShellHistory_ReadsLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell_history")
	content := "projects\nuse PHI01\nwhat-now 60\nstatus\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	lines := loadHistoryFromPath(path)
	assert.Equal(t, []string{"projects", "use PHI01", "what-now 60", "status"}, lines)
}

func TestLoadShellHistory_TruncatesOverMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell_history")

	// Write 600 lines.
	var b strings.Builder
	for i := 0; i < 600; i++ {
		b.WriteString("line\n")
	}
	require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o644))

	lines := loadHistoryFromPath(path)
	assert.Len(t, lines, maxHistoryLines)
}

func TestAppendShellHistory_AppendsLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell_history")

	appendHistoryToPath(path, "first command")
	appendHistoryToPath(path, "second command")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first command\nsecond command\n", string(data))
}

func TestAppendShellHistory_SkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell_history")

	appendHistoryToPath(path, "")
	appendHistoryToPath(path, "   ")

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "file should not be created for empty lines")
}

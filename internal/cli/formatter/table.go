package formatter

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderTable renders a simple aligned table with a header separator line.
// Headers are rendered with the Header style. Columns are padded to the
// maximum width found in each column across both headers and rows.
func RenderTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	cols := len(headers)

	// Compute max width per column, accounting for ANSI escape sequences
	// by measuring visible width.
	widths := make([]int, cols)
	for i, h := range headers {
		w := lipgloss.Width(h)
		if w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			w := lipgloss.Width(row[i])
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Add padding between columns.
	const colGap = 2

	var b strings.Builder

	// Render header row.
	for i, h := range headers {
		styled := StyleHeader.Render(h)
		pad := widths[i] - lipgloss.Width(h)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(styled)
		if i < cols-1 {
			b.WriteString(strings.Repeat(" ", pad+colGap))
		}
	}
	b.WriteString("\n")

	// Render separator line.
	for i, w := range widths {
		b.WriteString(StyleDim.Render(strings.Repeat("─", w)))
		if i < cols-1 {
			b.WriteString(strings.Repeat(" ", colGap))
		}
	}
	b.WriteString("\n")

	// Render data rows.
	for _, row := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			visible := lipgloss.Width(cell)
			pad := widths[i] - visible
			if pad < 0 {
				pad = 0
			}
			b.WriteString(cell)
			if i < cols-1 {
				b.WriteString(strings.Repeat(" ", pad+colGap))
			}
		}
		b.WriteString("\n")
	}

	return fmt.Sprint(b.String())
}

package formatter

import (
	"fmt"
	"strings"
)

// TreeItem represents a single node in a tree display.
type TreeItem struct {
	Title  string
	Level  int
	IsLast bool
	Status string
	Detail string
}

const (
	treeBranch = "├─ "
	treeCorner = "└─ "
	treePipe   = "│  "
)

// RenderTree renders a list of TreeItems as an indented tree using
// box-drawing characters for connectors.
func RenderTree(items []TreeItem) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder

	for _, item := range items {
		// Build the indentation prefix from the item's level.
		var prefix string
		if item.Level > 0 {
			// Indent with pipe or space for each parent level above 1.
			for i := 1; i < item.Level; i++ {
				prefix += treePipe
			}
			// Add the branch or corner connector for this level.
			if item.IsLast {
				prefix += treeCorner
			} else {
				prefix += treeBranch
			}
		}

		// Format the title. Completed items are dimmed.
		title := item.Title
		isCompleted := strings.EqualFold(item.Status, "done") ||
			strings.EqualFold(item.Status, "completed")

		if isCompleted {
			title = Dim(title)
		}

		// Build the line.
		line := prefix + title

		// Append status detail badges like [ DUE 2d ] or [ 30/90 pages ].
		if item.Detail != "" {
			badge := StyleBlue.Render(fmt.Sprintf("[ %s ]", item.Detail))
			line += "  " + badge
		}

		b.WriteString(line + "\n")
	}

	return b.String()
}

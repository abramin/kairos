package formatter

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TreeItem represents a single node in a tree display.
type TreeItem struct {
	Title  string
	Seq    int // project-scoped sequential ID; 0 means don't display
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
// box-drawing characters for connectors. Done items get a green ✔ prefix,
// in-progress items get an amber ▶ prefix, and detail badges are right-aligned.
func RenderTree(items []TreeItem) string {
	if len(items) == 0 {
		return ""
	}

	type lineInfo struct {
		content string // prefix + statusPrefix + title (styled)
		badge   string // styled badge or ""
	}

	lines := make([]lineInfo, len(items))
	maxContentWidth := 0

	// Pass 1: build each line's content and track max visible width.
	for idx, item := range items {
		var prefix string
		if item.Level > 0 {
			for i := 1; i < item.Level; i++ {
				prefix += treePipe
			}
			if item.IsLast {
				prefix += treeCorner
			} else {
				prefix += treeBranch
			}
		}

		title := item.Title
		if item.Seq > 0 {
			title = StyleDim.Render(fmt.Sprintf("#%d ", item.Seq)) + title
		}
		statusPrefix := ""

		isCompleted := strings.EqualFold(item.Status, "done") ||
			strings.EqualFold(item.Status, "completed")
		isActive := strings.EqualFold(item.Status, "in_progress")

		if isCompleted {
			statusPrefix = StyleGreen.Render("✔ ")
			title = Dim(title)
		} else if isActive {
			statusPrefix = StyleYellowBold.Render("▶ ")
			title = StyleYellowBold.Render(title)
		}

		content := prefix + statusPrefix + title
		lines[idx].content = content

		if item.Detail != "" {
			lines[idx].badge = StyleBlue.Render(fmt.Sprintf("[ %s ]", item.Detail))
		}

		if w := lipgloss.Width(content); w > maxContentWidth {
			maxContentWidth = w
		}
	}

	// Pass 2: render with right-aligned badges.
	var b strings.Builder
	for _, li := range lines {
		if li.badge != "" {
			pad := maxContentWidth - lipgloss.Width(li.content)
			if pad < 0 {
				pad = 0
			}
			b.WriteString(li.content + strings.Repeat(" ", pad) + "  " + li.badge + "\n")
		} else {
			b.WriteString(li.content + "\n")
		}
	}

	return b.String()
}

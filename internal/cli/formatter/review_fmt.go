package formatter

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
)

// ZettelBacklogData holds the computed data for the zettelkasten backlog section.
type ZettelBacklogData struct {
	ReadingMin   int
	ZettelMin    int
	ReadingItems []domain.SessionSummaryByType // only type=="reading" items
}

// ShouldShowZettelBacklog returns true if the backlog section should be displayed.
func ShouldShowZettelBacklog(data ZettelBacklogData) bool {
	if data.ReadingMin <= 0 {
		return false
	}
	if data.ZettelMin == 0 {
		return true
	}
	return float64(data.ReadingMin)/float64(data.ZettelMin) > 3.0
}

// FormatZettelBacklog renders the zettelkasten backlog nudge for the weekly review.
func FormatZettelBacklog(data ZettelBacklogData) string {
	var b strings.Builder

	// Ratio line
	ratio := "\u221e:1" // ∞:1
	if data.ZettelMin > 0 {
		ratio = fmt.Sprintf("%.1f:1", float64(data.ReadingMin)/float64(data.ZettelMin))
	}
	b.WriteString(StyleYellowBold.Render(fmt.Sprintf("%d min reading / %d min zettel processing", data.ReadingMin, data.ZettelMin)))
	b.WriteString(Dim(fmt.Sprintf("  (ratio: %s)", ratio)))
	b.WriteString("\n\n")

	// Reading items
	if len(data.ReadingItems) > 0 {
		b.WriteString(Header("Reading This Week"))
		b.WriteString("\n")
		for _, item := range data.ReadingItems {
			b.WriteString(fmt.Sprintf("  %s %s  %s\n",
				StyleYellow.Render("\u2022"),
				item.WorkItemTitle,
				Dim(fmt.Sprintf("(%s)", FormatMinutes(item.TotalMinutes))),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(Dim("Consider reviewing your reading notes for atomic note candidates."))
	b.WriteString("\n")

	return RenderBox("Zettelkasten Backlog", b.String())
}

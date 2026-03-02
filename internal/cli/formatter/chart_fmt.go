package formatter

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// Fixed Gruvbox colors for workout categories.
var workoutCategoryColor = map[string]lipgloss.Color{
	"Qigong":       lipgloss.Color("#fb4934"),
	"Calisthenics": lipgloss.Color("#fe8019"),
	"Running":      lipgloss.Color("#fabd2f"),
	"Kettlebell":   lipgloss.Color("#d65d0e"),
	"GMB Movement": lipgloss.Color("#cc241d"),
	"Stretching":   lipgloss.Color("#689d6a"),
	"Other":        lipgloss.Color("#928374"),
}

// Palette for project colors — assigned by name hash.
var projectColorPalette = []lipgloss.Color{
	lipgloss.Color("#d79921"), // yellow
	lipgloss.Color("#458588"), // blue
	lipgloss.Color("#b16286"), // purple
	lipgloss.Color("#98971a"), // green
	lipgloss.Color("#83a598"), // light blue
	lipgloss.Color("#d3869b"), // pink
	lipgloss.Color("#8ec07c"), // aqua
	lipgloss.Color("#ebdbb2"), // fg
}

// segmentColor returns the lipgloss color for a chart segment.
func segmentColor(seg domain.CategorySegment) lipgloss.Color {
	if seg.Kind == domain.SegmentWorkout {
		if c, ok := workoutCategoryColor[seg.Label]; ok {
			return c
		}
		return lipgloss.Color("#928374") // gray fallback
	}
	// Project: deterministic hash into palette.
	return projectColorFromName(seg.Label)
}

func projectColorFromName(name string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	idx := int(h.Sum32()) % len(projectColorPalette)
	return projectColorPalette[idx]
}

const (
	chartLabelWidth = 12
	chartTotalWidth = 10
	chartPadding    = 4
	chartMinWidth   = 60
)

// RenderChart renders a stacked horizontal bar chart of weekly time breakdown.
// If termWidth < 60, falls back to a compact table.
func RenderChart(breakdown []domain.WeeklyBreakdown, termWidth int) string {
	if len(breakdown) == 0 {
		return Dim("  No data for the selected period.")
	}

	if termWidth < chartMinWidth {
		return renderChartCompact(breakdown)
	}

	var b strings.Builder

	numWeeks := len(breakdown)
	title := fmt.Sprintf("Time Breakdown (%d weeks)", numWeeks)
	b.WriteString(Header(title))
	b.WriteString("\n\n")

	barWidth := termWidth - chartLabelWidth - chartTotalWidth - chartPadding
	if barWidth < 10 {
		barWidth = 10
	}

	// Find max total for scaling.
	maxTotal := 0
	for _, wk := range breakdown {
		if wk.TotalMin > maxTotal {
			maxTotal = wk.TotalMin
		}
	}
	if maxTotal == 0 {
		maxTotal = 1
	}

	// Track unique labels for legend.
	type legendEntry struct {
		label string
		kind  domain.SegmentKind
		color lipgloss.Color
	}
	legendSeen := make(map[string]bool)
	var legends []legendEntry

	for _, wk := range breakdown {
		// Week label, right-aligned in label column.
		label := fmt.Sprintf("%*s", chartLabelWidth-1, wk.WeekLabel)
		b.WriteString(StyleDim.Render(label))
		b.WriteString(" ")

		if wk.TotalMin == 0 {
			empty := strings.Repeat("░", barWidth)
			b.WriteString(StyleDim.Render(empty))
			b.WriteString(fmt.Sprintf("  %*s", chartTotalWidth-2, FormatMinutes(0)))
			b.WriteString("\n")
			continue
		}

		// Render each segment as colored blocks.
		usedWidth := 0
		for i, seg := range wk.Segments {
			color := segmentColor(seg)

			// Track for legend.
			if !legendSeen[seg.Label] {
				legendSeen[seg.Label] = true
				legends = append(legends, legendEntry{label: seg.Label, kind: seg.Kind, color: color})
			}

			// Calculate segment width proportional to minutes.
			segWidth := int(float64(seg.Minutes) / float64(maxTotal) * float64(barWidth))
			if segWidth < 1 && seg.Minutes > 0 {
				segWidth = 1
			}
			// Last segment: fill remaining allocated width.
			if i == len(wk.Segments)-1 {
				totalAllocated := int(float64(wk.TotalMin) / float64(maxTotal) * float64(barWidth))
				if totalAllocated > barWidth {
					totalAllocated = barWidth
				}
				segWidth = totalAllocated - usedWidth
				if segWidth < 1 {
					segWidth = 1
				}
			}
			if usedWidth+segWidth > barWidth {
				segWidth = barWidth - usedWidth
			}
			if segWidth <= 0 {
				continue
			}

			style := lipgloss.NewStyle().Foreground(color)
			b.WriteString(style.Render(strings.Repeat(filledBlock, segWidth)))
			usedWidth += segWidth
		}

		// Fill remaining bar width with empty blocks.
		if usedWidth < barWidth {
			b.WriteString(StyleDim.Render(strings.Repeat("░", barWidth-usedWidth)))
		}

		total := fmt.Sprintf("  %*s", chartTotalWidth-2, FormatMinutes(wk.TotalMin))
		b.WriteString(total)
		b.WriteString("\n")
	}

	// Legend.
	b.WriteString("\n")
	var projLegend, workoutLegend []string
	for _, le := range legends {
		style := lipgloss.NewStyle().Foreground(le.color)
		entry := style.Render("■") + " " + le.label
		if le.kind == domain.SegmentProject {
			projLegend = append(projLegend, entry)
		} else {
			workoutLegend = append(workoutLegend, entry)
		}
	}
	if len(projLegend) > 0 {
		b.WriteString("  " + strings.Join(projLegend, "  "))
	}
	if len(workoutLegend) > 0 {
		if len(projLegend) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + strings.Join(workoutLegend, "  "))
	}
	b.WriteString("\n")

	return b.String()
}

// renderChartCompact renders a table fallback for narrow terminals.
func renderChartCompact(breakdown []domain.WeeklyBreakdown) string {
	headers := []string{"WEEK", "CATEGORY", "DURATION"}
	var rows [][]string
	for _, wk := range breakdown {
		if len(wk.Segments) == 0 {
			rows = append(rows, []string{wk.WeekLabel, Dim("—"), Dim("0m")})
			continue
		}
		for i, seg := range wk.Segments {
			weekCol := ""
			if i == 0 {
				weekCol = wk.WeekLabel
			}
			rows = append(rows, []string{weekCol, seg.Label, FormatMinutes(seg.Minutes)})
		}
	}
	return RenderBox("Time Breakdown", RenderTable(headers, rows))
}

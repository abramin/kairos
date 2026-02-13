package formatter

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// RenderBox wraps content in a rounded-border box with an optional title.
func RenderBox(title string, content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDim).
		PaddingLeft(2).
		PaddingRight(2).
		PaddingTop(1).
		PaddingBottom(1)

	if title != "" {
		titleRendered := StyleHeader.Render(strings.ToUpper(title))
		inner := titleRendered + "\n\n" + content
		return boxStyle.Render(inner)
	}

	return boxStyle.Render(content)
}

// RelativeDate returns a human-friendly relative date string.
func RelativeDate(t time.Time) string {
	return RelativeDateFrom(t, time.Now())
}

// RelativeDateFrom returns a human-friendly relative date string from a reference time.
func RelativeDateFrom(t time.Time, now time.Time) string {
	diff := t.Sub(now)
	days := int(math.Round(diff.Hours() / 24))

	switch {
	case days == 0:
		return "Today"
	case days == 1:
		return "Tomorrow"
	case days == -1:
		return "Yesterday"
	case days > 0 && days < 14:
		return fmt.Sprintf("In %dd", days)
	case days > 0 && days < 60:
		return fmt.Sprintf("In %dw", days/7)
	case days > 0:
		return fmt.Sprintf("In %dmo", days/30)
	case days < 0 && days > -14:
		return fmt.Sprintf("%dd ago", -days)
	case days < 0 && days > -60:
		return fmt.Sprintf("%dw ago", -days/7)
	default:
		return fmt.Sprintf("%dmo ago", -days/30)
	}
}

// RelativeDateStyled returns RelativeDate with urgency coloring applied.
func RelativeDateStyled(t time.Time) string {
	text := RelativeDate(t)
	days := int(math.Round(time.Until(t).Hours() / 24))

	if days >= 0 && days <= 2 {
		return StyleRed.Render(text)
	}
	if days > 2 && days <= 7 {
		return StyleYellow.Render(text)
	}
	if days < 0 {
		return StyleRed.Render(text)
	}
	return StyleFg.Render(text)
}

// HumanDate returns a human-friendly absolute date string.
func HumanDate(t time.Time) string {
	now := time.Now()
	y1, m1, d1 := now.Date()
	y2, m2, d2 := t.Date()

	if y1 == y2 && m1 == m2 && d1 == d2 {
		return "Today"
	}
	yesterday := now.AddDate(0, 0, -1)
	y3, m3, d3 := yesterday.Date()
	if y2 == y3 && m2 == m3 && d2 == d3 {
		return "Yesterday"
	}
	return t.Format("Jan 2, 2006")
}

// HumanTimestamp returns a human-friendly relative timestamp string.
func HumanTimestamp(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < 0:
		return HumanDate(t)
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return HumanDate(t)
	}
}

// StatusPill returns a colored status indicator for project status.
func StatusPill(status domain.ProjectStatus) string {
	switch status {
	case domain.ProjectActive:
		return StyleGreen.Render("● Active")
	case domain.ProjectPaused:
		return StyleYellow.Render("○ Paused")
	case domain.ProjectDone:
		return StyleDim.Render("✔ Done")
	case domain.ProjectArchived:
		return StyleDim.Render("✖ Archived")
	default:
		return StyleDim.Render(string(status))
	}
}

// ModeBadge returns a styled mode indicator with description.
func ModeBadge(mode domain.PlanMode) string {
	if mode == domain.ModeCritical {
		return StyleRed.Render("▲ CRITICAL MODE") + Dim(" — focus on critical work only")
	}
	return StyleGreen.Render("● BALANCED") + Dim(" — all projects available")
}

// WorkItemStatusPill returns a colored status indicator for work item status.
func WorkItemStatusPill(status domain.WorkItemStatus) string {
	switch status {
	case domain.WorkItemTodo:
		return StyleBlue.Render("○ Todo")
	case domain.WorkItemInProgress:
		return StyleGreen.Render("● In Progress")
	case domain.WorkItemDone:
		return StyleDim.Render("✔ Done")
	case domain.WorkItemSkipped:
		return StyleDim.Render("⊘ Skipped")
	case domain.WorkItemArchived:
		return StyleDim.Render("✖ Archived")
	default:
		return StyleDim.Render(string(status))
	}
}

// DomainBadge returns a capitalized, purple-styled domain label.
func DomainBadge(d string) string {
	if d == "" {
		return StyleDim.Render("--")
	}
	label := strings.ToUpper(d[:1]) + d[1:]
	return StylePurple.Render(label)
}

// TruncID returns the first 8 characters of an ID, dimmed.
func TruncID(id string) string {
	if len(id) > 8 {
		id = id[:8]
	}
	return StyleDim.Render(id)
}

// FormatMinutes converts raw minutes into human-friendly format.
func FormatMinutes(min int) string {
	if min <= 0 {
		return "0m"
	}
	h := min / 60
	m := min % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
)

// FormatWhatNow formats a WhatNowResponse into a styled CLI dashboard string.
func FormatWhatNow(resp *contract.WhatNowResponse) string {
	return FormatWhatNowWithProjectIDs(resp, nil, nil)
}

// FormatWhatNowWithProjectIDs formats WhatNow output, replacing internal project IDs with
// project names when available, and rendering per-item natural language summaries instead of
// REASON lines when itemSummaries is provided.
func FormatWhatNowWithProjectIDs(resp *contract.WhatNowResponse, projectNames map[string]string, itemSummaries map[string]string) string {
	var b strings.Builder

	// Mode indicator.
	modeLabel := string(resp.Mode)
	b.WriteString(StylePurple.Render(fmt.Sprintf("MODE: %s", strings.ToUpper(modeLabel))))
	b.WriteString("\n\n")

	// Session header.
	headerText := fmt.Sprintf("Suggested Session (%s available)", FormatMinutes(resp.RequestedMin))
	b.WriteString(Header(headerText))
	b.WriteString("\n\n")

	// Recommendations.
	if len(resp.Recommendations) == 0 {
		b.WriteString(Dim("No recommendations available."))
		b.WriteString("\n")
	} else {
		now := time.Now()
		for i, rec := range resp.Recommendations {
			num := fmt.Sprintf("%d.", i+1)

			// Title line differs for habits vs work items.
			var titleLine string
			if rec.IsHabit {
				titleLine = fmt.Sprintf(
					"%s %s  %s  %s",
					Bold(num),
					StyleFg.Render(rec.Title),
					StyleBlue.Render(fmt.Sprintf("(%s)", FormatMinutes(rec.AllocatedMin))),
					StylePurple.Render("HABIT"),
				)
			} else {
				riskBadge := itemRiskBadge(rec.RiskLevel, rec.DueDate, now)
				seqLabel := ""
				if rec.WorkItemSeq > 0 {
					seqLabel = StyleDim.Render(fmt.Sprintf("#%d ", rec.WorkItemSeq))
				}
				titleLine = fmt.Sprintf(
					"%s %s%s  %s  %s",
					Bold(num),
					seqLabel,
					StyleFg.Render(rec.Title),
					StyleBlue.Render(fmt.Sprintf("(%s)", FormatMinutes(rec.AllocatedMin))),
					riskBadge,
				)
			}
			b.WriteString(titleLine + "\n")

			if rec.IsHabit {
				// Habit-specific info: cadence and last-done status.
				cadence := formatHabitCadence(rec.CadenceDays)
				status := formatHabitDueSince(rec.DaysSinceLog, rec.CadenceDays)
				b.WriteString(fmt.Sprintf("   %s %s\n", Dim(cadence+" •"), status))
			} else {
				// Project info with name when available.
				if rec.ProjectID != "" {
					b.WriteString(fmt.Sprintf("   %s %s\n", Dim("Project:"), renderProjectID(rec.ProjectID, projectNames)))
				}

				// Due date with relative styling.
				if rec.DueDate != nil {
					if parsed, ok := parseDueDate(*rec.DueDate); ok {
						b.WriteString(fmt.Sprintf("   %s %s\n", Dim("Due:"), RelativeDateStyled(parsed)))
					} else {
						b.WriteString(fmt.Sprintf("   %s\n", Dim(fmt.Sprintf("Due: %s", *rec.DueDate))))
					}
				}
			}

			// Natural language summary when available, otherwise fall back to REASON lines.
			if itemSummaries != nil {
				if summary := itemSummaries[rec.WorkItemID]; summary != "" {
					b.WriteString(fmt.Sprintf("   %s\n", Dim(summary)))
				}
			} else {
				for _, reason := range rec.Reasons {
					b.WriteString(fmt.Sprintf("   %s %s\n",
						StyleYellow.Render("REASON:"),
						Dim(reason.Message),
					))
				}
			}

			// Blank line between recommendations.
			if i < len(resp.Recommendations)-1 {
				b.WriteString("\n")
			}
		}
	}

	// Summary line.
	b.WriteString("\n")
	summaryLine := fmt.Sprintf(
		"%s  %s  %s",
		StyleGreen.Render(fmt.Sprintf("Allocated: %s", FormatMinutes(resp.AllocatedMin))),
		StyleDim.Render("|"),
		StyleDim.Render(fmt.Sprintf("Unallocated: %s", FormatMinutes(resp.UnallocatedMin))),
	)
	b.WriteString(summaryLine + "\n")

	// Policy messages.
	if len(resp.PolicyMessages) > 0 {
		b.WriteString("\n")
		for _, msg := range resp.PolicyMessages {
			b.WriteString(Dim(fmt.Sprintf("  %s", msg)) + "\n")
		}
	}

	// Warnings.
	if len(resp.Warnings) > 0 {
		b.WriteString("\n")
		for _, w := range resp.Warnings {
			b.WriteString(StyleYellow.Render(fmt.Sprintf("  WARNING: %s", w)) + "\n")
		}
	}

	return RenderBox("Session Plan", b.String())
}

func formatHabitCadence(cadenceDays int) string {
	switch cadenceDays {
	case 1:
		return "Daily"
	case 7:
		return "Weekly"
	default:
		return fmt.Sprintf("Every %dd", cadenceDays)
	}
}

func formatHabitDueSince(daysSinceLog, cadenceDays int) string {
	if daysSinceLog >= 9999 {
		return StyleYellow.Render("never done")
	}
	overdue := daysSinceLog - cadenceDays
	switch {
	case overdue > 0:
		return StyleYellow.Render(fmt.Sprintf("overdue %dd", overdue))
	case overdue == 0:
		return StyleYellow.Render("due today")
	default:
		return StyleGreen.Render(fmt.Sprintf("%dd ago", daysSinceLog))
	}
}

// parseDueDate tries RFC3339 then date-only format, returning zero time on failure.
func parseDueDate(raw string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// itemRiskBadge returns the risk badge for a work item, upgrading to AT RISK
// when the project is ON TRACK but the item's own due date is past due.
func itemRiskBadge(risk domain.RiskLevel, dueDate *string, now time.Time) string {
	if risk == domain.RiskOnTrack && dueDate != nil {
		if parsed, ok := parseDueDate(*dueDate); ok {
			if parsed.Before(now.Truncate(24 * time.Hour)) {
				return StyleYellow.Render("● AT RISK")
			}
		}
	}
	return RiskIndicator(risk)
}

func renderProjectID(projectID string, projectIDs map[string]string) string {
	if projectIDs != nil {
		if displayID := strings.TrimSpace(projectIDs[projectID]); displayID != "" {
			return StyleDim.Render(displayID)
		}
	}
	return TruncID(projectID)
}

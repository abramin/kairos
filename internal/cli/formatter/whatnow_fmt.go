package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
)

// FormatWhatNow formats a WhatNowResponse into a styled CLI dashboard string.
func FormatWhatNow(resp *contract.WhatNowResponse) string {
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
		for i, rec := range resp.Recommendations {
			num := fmt.Sprintf("%d.", i+1)
			riskBadge := RiskIndicator(rec.RiskLevel)

			// Title line: "1. Title  (25m)  ● ON TRACK"
			titleLine := fmt.Sprintf(
				"%s %s  %s  %s",
				Bold(num),
				StyleFg.Render(rec.Title),
				StyleBlue.Render(fmt.Sprintf("(%s)", FormatMinutes(rec.AllocatedMin))),
				riskBadge,
			)
			b.WriteString(titleLine + "\n")

			// Project info with truncated ID.
			if rec.ProjectID != "" {
				b.WriteString(fmt.Sprintf("   %s\n", Dim(fmt.Sprintf("Project: %s", TruncID(rec.ProjectID)))))
			}

			// Due date with relative styling.
			if rec.DueDate != nil {
				if parsed, err := time.Parse(time.RFC3339, *rec.DueDate); err == nil {
					b.WriteString(fmt.Sprintf("   %s %s\n", Dim("Due:"), RelativeDateStyled(parsed)))
				} else if parsed, err := time.Parse("2006-01-02", *rec.DueDate); err == nil {
					b.WriteString(fmt.Sprintf("   %s %s\n", Dim("Due:"), RelativeDateStyled(parsed)))
				} else {
					b.WriteString(fmt.Sprintf("   %s\n", Dim(fmt.Sprintf("Due: %s", *rec.DueDate))))
				}
			}

			// Reason lines.
			for _, reason := range rec.Reasons {
				b.WriteString(fmt.Sprintf("   %s %s\n",
					StyleYellow.Render("REASON:"),
					Dim(reason.Message),
				))
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

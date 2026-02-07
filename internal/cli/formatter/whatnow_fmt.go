package formatter

import (
	"fmt"
	"strings"

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
	headerText := fmt.Sprintf("SUGGESTED SESSION (%d Minutes Available)", resp.RequestedMin)
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

			// Title line: "1. Title  (25 min)  ● ON TRACK"
			titleLine := fmt.Sprintf(
				"%s %s  %s  %s",
				Bold(num),
				StyleFg.Render(rec.Title),
				StyleBlue.Render(fmt.Sprintf("(%d min)", rec.AllocatedMin)),
				riskBadge,
			)
			b.WriteString(titleLine + "\n")

			// Project info.
			if rec.ProjectID != "" {
				b.WriteString(fmt.Sprintf("   %s\n", Dim(fmt.Sprintf("Project: %s", rec.ProjectID))))
			}

			// Due date if present.
			if rec.DueDate != nil {
				b.WriteString(fmt.Sprintf("   %s\n", Dim(fmt.Sprintf("Due: %s", *rec.DueDate))))
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
		StyleGreen.Render(fmt.Sprintf("Allocated: %d min", resp.AllocatedMin)),
		StyleDim.Render("|"),
		StyleDim.Render(fmt.Sprintf("Unallocated: %d min", resp.UnallocatedMin)),
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

	return b.String()
}

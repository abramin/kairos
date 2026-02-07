package formatter

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
)

const statusProgressBarWidth = 10

// FormatStatus formats a StatusResponse into a styled CLI dashboard string.
func FormatStatus(resp *contract.StatusResponse) string {
	var b strings.Builder

	// Header.
	b.WriteString(Header("Projects Overview"))
	b.WriteString("\n\n")

	// Build the table.
	headers := []string{"NAME", "DOMAIN", "PROGRESS", "STATUS", "DUE"}
	rows := make([][]string, 0, len(resp.Projects))

	for _, p := range resp.Projects {
		// Progress bar.
		progress := RenderProgress(p.ProgressTimePct, statusProgressBarWidth)

		// Risk indicator as status.
		status := RiskIndicator(p.RiskLevel)

		// Due date.
		due := Dim("--")
		if p.DueDate != nil {
			dueStr := *p.DueDate
			if p.DaysLeft != nil {
				dueStr = fmt.Sprintf("%s (%dd)", *p.DueDate, *p.DaysLeft)
			}
			due = StyleFg.Render(dueStr)
		}

		// Domain display (capitalize first letter).
		domainStr := Dim("--")
		if p.ProjectName != "" {
			// Use project status as fallback label; domain is not on ProjectStatusView,
			// so we show the project status.
			domainStr = StylePurple.Render(string(p.Status))
		}

		rows = append(rows, []string{
			Bold(p.ProjectName),
			domainStr,
			progress,
			status,
			due,
		})
	}

	b.WriteString(RenderTable(headers, rows))

	// Summary line.
	summary := resp.Summary
	b.WriteString("\n")

	criticalPart := StyleRed.Render(fmt.Sprintf("%d Critical", summary.CountsCritical))
	atRiskPart := StyleYellow.Render(fmt.Sprintf("%d At Risk", summary.CountsAtRisk))
	onTrackPart := StyleGreen.Render(fmt.Sprintf("%d On Track", summary.CountsOnTrack))

	summaryLine := fmt.Sprintf("%s, %s, %s", criticalPart, atRiskPart, onTrackPart)
	b.WriteString(summaryLine + "\n")

	// Policy message.
	if summary.PolicyMessage != "" {
		b.WriteString("\n")
		b.WriteString(Dim(summary.PolicyMessage) + "\n")
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

// countByRisk counts projects by risk level. This is a utility in case
// the summary counts are not pre-computed.
func countByRisk(projects []contract.ProjectStatusView) (critical, atRisk, onTrack int) {
	for _, p := range projects {
		switch p.RiskLevel {
		case domain.RiskCritical:
			critical++
		case domain.RiskAtRisk:
			atRisk++
		case domain.RiskOnTrack:
			onTrack++
		}
	}
	return
}

// formatDomainLabel formats a domain string for display. Currently a
// simple pass-through that capitalizes the first letter.
func formatDomainLabel(d string) string {
	if d == "" {
		return "--"
	}
	return strings.ToUpper(d[:1]) + d[1:]
}

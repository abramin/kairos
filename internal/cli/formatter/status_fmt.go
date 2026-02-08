package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
)

const statusProgressBarWidth = 10

// FormatStatus formats a StatusResponse into a styled CLI dashboard string.
func FormatStatus(resp *contract.StatusResponse) string {
	var b strings.Builder

	// Build the table.
	headers := []string{"NAME", "STATUS", "PROGRESS", "RISK", "DUE"}
	rows := make([][]string, 0, len(resp.Projects))

	for _, p := range resp.Projects {
		// Progress bar.
		progress := RenderProgress(p.ProgressTimePct, statusProgressBarWidth)

		// Risk indicator.
		risk := RiskIndicator(p.RiskLevel)

		// Status pill.
		status := StatusPill(p.Status)

		// Due date with relative styling.
		due := Dim("--")
		if p.DueDate != nil {
			if parsed, err := time.Parse("2006-01-02", *p.DueDate); err == nil {
				due = RelativeDateStyled(parsed)
			} else {
				due = StyleFg.Render(*p.DueDate)
			}
		}

		rows = append(rows, []string{
			Bold(p.ProjectName),
			status,
			progress,
			risk,
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

	return RenderBox("Status", b.String())
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

package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
)

// FormatTemplateList renders a styled template list inside a bordered box.
func FormatTemplateList(templates []domain.Template) string {
	headers := []string{"NAME", "DOMAIN", "VERSION"}
	rows := make([][]string, 0, len(templates))

	for _, t := range templates {
		rows = append(rows, []string{
			Bold(t.Name),
			DomainBadge(t.Domain),
			Dim(t.Version),
		})
	}

	table := RenderTable(headers, rows)
	return RenderBox("Templates", table)
}

// FormatTemplateShow renders a styled template detail card.
func FormatTemplateShow(t *domain.Template) string {
	var b strings.Builder

	titleLine := fmt.Sprintf("%s  %s", StyleBold.Render(t.Name), DomainBadge(t.Domain))
	b.WriteString(titleLine + "\n\n")

	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("VERSION"), Dim(t.Version)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", StyleDim.Render("ID     "), Dim(t.ID)))

	b.WriteString("\n")
	b.WriteString(Header("Configuration"))
	b.WriteString("\n")

	// Pretty-print the JSON config if valid, otherwise show raw.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(t.ConfigJSON), "  ", "  "); err == nil {
		b.WriteString("  " + pretty.String() + "\n")
	} else {
		b.WriteString("  " + t.ConfigJSON + "\n")
	}

	return RenderBox("", b.String())
}

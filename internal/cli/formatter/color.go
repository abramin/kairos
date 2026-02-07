package formatter

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// Gruvbox-inspired color palette.
var (
	ColorGreen  = lipgloss.Color("#8ec07c")
	ColorYellow = lipgloss.Color("#fabd2f")
	ColorRed    = lipgloss.Color("#fb4934")
	ColorBlue   = lipgloss.Color("#83a598")
	ColorPurple = lipgloss.Color("#d3869b")
	ColorDim    = lipgloss.Color("#928374")
	ColorFg     = lipgloss.Color("#ebdbb2")
	ColorHeader = lipgloss.Color("#fe8019")
)

// Predefined lipgloss styles.
var (
	StyleGreen  = lipgloss.NewStyle().Foreground(ColorGreen)
	StyleYellow = lipgloss.NewStyle().Foreground(ColorYellow)
	StyleRed    = lipgloss.NewStyle().Foreground(ColorRed)
	StyleBlue   = lipgloss.NewStyle().Foreground(ColorBlue)
	StylePurple = lipgloss.NewStyle().Foreground(ColorPurple)
	StyleDim    = lipgloss.NewStyle().Foreground(ColorDim)
	StyleFg     = lipgloss.NewStyle().Foreground(ColorFg)
	StyleHeader = lipgloss.NewStyle().Foreground(ColorHeader).Bold(true)
	StyleBold   = lipgloss.NewStyle().Foreground(ColorFg).Bold(true)
)

// RiskColor returns the lipgloss style corresponding to the given risk level.
func RiskColor(risk domain.RiskLevel) lipgloss.Style {
	switch risk {
	case domain.RiskCritical:
		return StyleRed
	case domain.RiskAtRisk:
		return StyleYellow
	case domain.RiskOnTrack:
		return StyleGreen
	default:
		return StyleDim
	}
}

// RiskIndicator returns a colored risk indicator string such as "● CRITICAL".
func RiskIndicator(risk domain.RiskLevel) string {
	switch risk {
	case domain.RiskCritical:
		return StyleRed.Render("● CRITICAL")
	case domain.RiskAtRisk:
		return StyleYellow.Render("● AT RISK")
	case domain.RiskOnTrack:
		return StyleGreen.Render("● ON TRACK")
	default:
		return StyleDim.Render("● UNKNOWN")
	}
}

// Header renders a section header with the orange header style and an underline.
func Header(text string) string {
	upper := strings.ToUpper(text)
	line := strings.Repeat("─", len(upper))
	return fmt.Sprintf("%s\n%s", StyleHeader.Render(upper), StyleDim.Render(line))
}

// Dim renders text in the muted/dim color.
func Dim(text string) string {
	return StyleDim.Render(text)
}

// Bold renders text in bold with the foreground color.
func Bold(text string) string {
	return StyleBold.Render(text)
}

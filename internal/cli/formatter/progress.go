package formatter

import (
	"fmt"
	"strings"
)

const (
	filledBlock = "█"
	emptyBlock  = "░"
)

// RenderProgress renders a progress bar like [████░░░░] 45%.
// The bar is colored based on percentage: green >66%, yellow 33-66%, red <33%.
func RenderProgress(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	if width < 2 {
		width = 2
	}

	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := strings.Repeat(filledBlock, filled) + strings.Repeat(emptyBlock, empty)

	var style = StyleGreen
	if pct < 0.33 {
		style = StyleRed
	} else if pct < 0.66 {
		style = StyleYellow
	}

	pctStr := fmt.Sprintf("%3.0f%%", pct*100)
	return fmt.Sprintf("[%s] %s", style.Render(bar), pctStr)
}

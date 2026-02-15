package formatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderCompactBar(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		width int
		dim  bool
	}{
		{"0% normal", 0.0, 10, false},
		{"50% normal", 0.5, 10, false},
		{"100% normal", 1.0, 10, false},
		{"50% dimmed", 0.5, 10, true},
		{"over 100% clamps", 1.5, 10, false},
		{"negative clamps", -0.5, 10, false},
		{"tiny width clamps to 2", 0.5, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderCompactBar(tt.pct, tt.width, tt.dim)
			assert.NotEmpty(t, got)
			// Compact bar must not contain brackets or percentage text.
			assert.NotContains(t, got, "[")
			assert.NotContains(t, got, "]")
			assert.NotContains(t, got, "%")
		})
	}
}

func TestRenderCompactBarBlocks(t *testing.T) {
	// With dim=true we can strip ANSI and check block content.
	// 0% should be all empty blocks.
	bar0 := RenderCompactBar(0.0, 4, true)
	assert.Contains(t, bar0, emptyBlock)

	// 100% should be all filled blocks.
	bar100 := RenderCompactBar(1.0, 4, true)
	assert.Contains(t, bar100, filledBlock)
}

package formatter

import (
	"strings"
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRenderChart_EmptyData(t *testing.T) {
	result := RenderChart(nil, 100)
	assert.Contains(t, result, "No data")
}

func TestRenderChart_CompactFallback(t *testing.T) {
	breakdown := []domain.WeeklyBreakdown{
		{
			ISOWeek:   "2026-W08",
			WeekLabel: "Feb 16–22",
			Segments: []domain.CategorySegment{
				{Label: "Alpha", Minutes: 60, Kind: domain.SegmentProject},
			},
			TotalMin: 60,
		},
	}
	// Width < 60 should trigger compact table.
	result := RenderChart(breakdown, 50)
	assert.Contains(t, result, "Alpha")
	assert.Contains(t, result, "Feb 16–22")
	// Compact uses RenderBox which uppercases title.
	assert.Contains(t, result, "TIME BREAKDOWN")
}

func TestRenderChart_NormalRendering(t *testing.T) {
	breakdown := []domain.WeeklyBreakdown{
		{
			ISOWeek:   "2026-W08",
			WeekLabel: "Feb 16–22",
			Segments: []domain.CategorySegment{
				{Label: "Kairos", Minutes: 90, Kind: domain.SegmentProject},
				{Label: "Qigong", Minutes: 20, Kind: domain.SegmentWorkout},
			},
			TotalMin: 110,
		},
		{
			ISOWeek:   "2026-W07",
			WeekLabel: "Feb 9–15",
			Segments: []domain.CategorySegment{
				{Label: "Kairos", Minutes: 60, Kind: domain.SegmentProject},
			},
			TotalMin: 60,
		},
	}

	result := RenderChart(breakdown, 100)
	// Header() uppercases text.
	assert.Contains(t, result, "TIME BREAKDOWN (2 WEEKS)")
	assert.Contains(t, result, "Feb 16–22")
	assert.Contains(t, result, "Feb 9–15")
	// Should contain block characters.
	assert.True(t, strings.Contains(result, filledBlock), "should contain filled blocks")
	// Legend should have both project and workout entries.
	assert.Contains(t, result, "Kairos")
	assert.Contains(t, result, "Qigong")
}

func TestRenderChart_EmptyWeek(t *testing.T) {
	breakdown := []domain.WeeklyBreakdown{
		{
			ISOWeek:   "2026-W08",
			WeekLabel: "Feb 16–22",
			Segments:  nil,
			TotalMin:  0,
		},
	}

	result := RenderChart(breakdown, 100)
	assert.Contains(t, result, "Feb 16–22")
	// Empty week should still render (with empty blocks).
	assert.True(t, strings.Contains(result, "░"), "should contain empty blocks for zero week")
}

func TestProjectColorFromName_Deterministic(t *testing.T) {
	c1 := projectColorFromName("Kairos")
	c2 := projectColorFromName("Kairos")
	assert.Equal(t, c1, c2, "same name should produce same color")

	c3 := projectColorFromName("Other Project")
	// Different names may or may not have different colors, but the function shouldn't panic.
	_ = c3
}

func TestSegmentColor_WorkoutCategory(t *testing.T) {
	seg := domain.CategorySegment{Label: "Qigong", Kind: domain.SegmentWorkout}
	color := segmentColor(seg)
	assert.NotEmpty(t, string(color))
}

func TestSegmentColor_ProjectFallback(t *testing.T) {
	seg := domain.CategorySegment{Label: "MyProject", Kind: domain.SegmentProject}
	color := segmentColor(seg)
	assert.NotEmpty(t, string(color))
}

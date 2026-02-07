package formatter

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRelativeDateFrom(t *testing.T) {
	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{"today", now, "Today"},
		{"tomorrow", now.Add(24 * time.Hour), "Tomorrow"},
		{"yesterday", now.Add(-24 * time.Hour), "Yesterday"},
		{"3 days future", now.Add(3 * 24 * time.Hour), "In 3d"},
		{"3 days past", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"10 days future", now.Add(10 * 24 * time.Hour), "In 10d"},
		{"3 weeks future", now.Add(21 * 24 * time.Hour), "In 3w"},
		{"3 months future", now.Add(90 * 24 * time.Hour), "In 3mo"},
		{"2 weeks past", now.Add(-14 * 24 * time.Hour), "2w ago"},
		{"3 months past", now.Add(-90 * 24 * time.Hour), "3mo ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeDateFrom(tt.input, now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHumanDate(t *testing.T) {
	// Test that a past date returns formatted date
	past := time.Date(2022, 9, 30, 0, 0, 0, 0, time.UTC)
	got := HumanDate(past)
	assert.Equal(t, "Sep 30, 2022", got)

	// Test today
	today := time.Now()
	got = HumanDate(today)
	assert.Equal(t, "Today", got)

	// Test yesterday
	yesterday := time.Now().AddDate(0, 0, -1)
	got = HumanDate(yesterday)
	assert.Equal(t, "Yesterday", got)
}

func TestHumanTimestamp(t *testing.T) {
	now := time.Now()

	got := HumanTimestamp(now)
	assert.Equal(t, "Just now", got)

	got = HumanTimestamp(now.Add(-5 * time.Minute))
	assert.Equal(t, "5m ago", got)

	got = HumanTimestamp(now.Add(-2 * time.Hour))
	assert.Equal(t, "2h ago", got)

	// More than 24h falls back to HumanDate
	got = HumanTimestamp(now.Add(-48 * time.Hour))
	assert.NotEmpty(t, got)
}

func TestStatusPill(t *testing.T) {
	tests := []struct {
		status   domain.ProjectStatus
		contains string
	}{
		{domain.ProjectActive, "Active"},
		{domain.ProjectPaused, "Paused"},
		{domain.ProjectDone, "Done"},
		{domain.ProjectArchived, "Archived"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := StatusPill(tt.status)
			assert.Contains(t, got, tt.contains)
		})
	}
}

func TestWorkItemStatusPill(t *testing.T) {
	tests := []struct {
		status   domain.WorkItemStatus
		contains string
	}{
		{domain.WorkItemTodo, "Todo"},
		{domain.WorkItemInProgress, "In Progress"},
		{domain.WorkItemDone, "Done"},
		{domain.WorkItemSkipped, "Skipped"},
		{domain.WorkItemArchived, "Archived"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := WorkItemStatusPill(tt.status)
			assert.Contains(t, got, tt.contains)
		})
	}
}

func TestDomainBadge(t *testing.T) {
	got := DomainBadge("education")
	assert.Contains(t, got, "Education")

	got = DomainBadge("fitness")
	assert.Contains(t, got, "Fitness")

	got = DomainBadge("")
	assert.Contains(t, got, "--")
}

func TestTruncID(t *testing.T) {
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	got := TruncID(id)
	assert.Contains(t, got, "a1b2c3d4")
	assert.NotContains(t, got, "e5f6")

	// Short IDs should be returned as-is (dimmed)
	got = TruncID("short")
	assert.Contains(t, got, "short")
}

func TestFormatMinutes(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0m"},
		{-5, "0m"},
		{45, "45m"},
		{60, "1h"},
		{120, "2h"},
		{150, "2h 30m"},
		{61, "1h 1m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatMinutes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderBox(t *testing.T) {
	result := RenderBox("TEST", "content here")
	assert.Contains(t, result, "TEST")
	assert.Contains(t, result, "content here")
	// Should contain rounded border characters
	assert.Contains(t, result, "╭")
	assert.Contains(t, result, "╰")
}

func TestRenderBoxWithoutTitle(t *testing.T) {
	result := RenderBox("", "just content")
	assert.Contains(t, result, "just content")
	assert.Contains(t, result, "╭")
}

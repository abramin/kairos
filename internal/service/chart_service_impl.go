package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
)

type chartService struct {
	sessions repository.SessionRepo
	workouts repository.WorkoutLogRepo
}

// NewChartService creates a new ChartService.
func NewChartService(sessions repository.SessionRepo, workouts repository.WorkoutLogRepo) ChartService {
	return &chartService{sessions: sessions, workouts: workouts}
}

func (s *chartService) WeeklyBreakdown(ctx context.Context, numWeeks int) ([]domain.WeeklyBreakdown, error) {
	if numWeeks <= 0 {
		numWeeks = 6
	}

	now := time.Now().UTC()
	from, to := weekRange(now, numWeeks)

	// Fetch project session minutes grouped by project+week.
	projRows, err := s.sessions.ListSessionMinutesByWeek(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("chart: loading session data: %w", err)
	}

	// Fetch workout logs in the date range and aggregate in Go.
	workoutLogs, err := s.workouts.ListByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("chart: loading workout data: %w", err)
	}

	// Build a map of isoWeek → segments.
	weekMap := make(map[string][]domain.CategorySegment)

	for _, pw := range projRows {
		weekMap[pw.ISOWeek] = append(weekMap[pw.ISOWeek], domain.CategorySegment{
			Label:   pw.ProjectName,
			Minutes: pw.TotalMin,
			Kind:    domain.SegmentProject,
		})
	}

	// Aggregate workouts by category + ISO week.
	type catWeekKey struct {
		week     string
		category domain.WorkoutCategory
	}
	workoutAgg := make(map[catWeekKey]int)
	for _, wl := range workoutLogs {
		key := catWeekKey{week: isoWeek(wl.PerformedAt), category: wl.Category}
		workoutAgg[key] += wl.Minutes
	}
	for key, mins := range workoutAgg {
		label := domain.WorkoutCategoryLabel[key.category]
		if label == "" {
			label = string(key.category)
		}
		weekMap[key.week] = append(weekMap[key.week], domain.CategorySegment{
			Label:   label,
			Minutes: mins,
			Kind:    domain.SegmentWorkout,
		})
	}

	// Build ordered list of all ISO weeks in range.
	allWeeks := enumerateWeeks(from, to)

	result := make([]domain.WeeklyBreakdown, 0, len(allWeeks))
	for _, wk := range allWeeks {
		segments := weekMap[wk]
		// Sort segments by minutes descending.
		sort.Slice(segments, func(i, j int) bool {
			return segments[i].Minutes > segments[j].Minutes
		})
		total := 0
		for _, seg := range segments {
			total += seg.Minutes
		}
		result = append(result, domain.WeeklyBreakdown{
			ISOWeek:   wk,
			WeekLabel: weekLabel(wk),
			Segments:  segments,
			TotalMin:  total,
		})
	}

	return result, nil
}

// weekRange returns the from (inclusive) and to (exclusive) times spanning numWeeks
// ending at the current ISO week.
func weekRange(now time.Time, numWeeks int) (time.Time, time.Time) {
	// Find the Monday of the current ISO week.
	y, w := now.ISOWeek()
	// time.Date with ISOWeek: go back to Jan 1 then adjust.
	// Simpler: find the current weekday offset from Monday.
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)

	// "to" is start of next week (exclusive).
	to := monday.AddDate(0, 0, 7)
	// "from" is numWeeks before the current Monday.
	from := monday.AddDate(0, 0, -7*(numWeeks-1))

	_ = y
	_ = w
	return from, to
}

// isoWeek returns the ISO week string for a given time (e.g. "2026-W08").
func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// enumerateWeeks returns all ISO week strings from the week containing 'from'
// through the week containing 'to' (exclusive), most recent first.
func enumerateWeeks(from, to time.Time) []string {
	var weeks []string
	seen := make(map[string]bool)
	// Walk day by day from 'from' to 'to', collecting unique weeks.
	for d := from; d.Before(to); d = d.AddDate(0, 0, 7) {
		wk := isoWeek(d)
		if !seen[wk] {
			seen[wk] = true
			weeks = append(weeks, wk)
		}
	}
	// Also ensure the 'to' boundary week is not missed (if to falls on a Monday).
	// Reverse: most recent first.
	for i, j := 0, len(weeks)-1; i < j; i, j = i+1, j-1 {
		weeks[i], weeks[j] = weeks[j], weeks[i]
	}
	return weeks
}

// weekLabel converts an ISO week string like "2026-W08" into a human label
// like "Feb 16–22".
func weekLabel(isoWk string) string {
	var y, w int
	if _, err := fmt.Sscanf(isoWk, "%d-W%d", &y, &w); err != nil {
		return isoWk
	}
	// Find the Monday of this ISO week.
	// Jan 4 is always in ISO week 1.
	jan4 := time.Date(y, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := jan4.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	isoWeek1Monday := jan4.AddDate(0, 0, -int(weekday-time.Monday))
	monday := isoWeek1Monday.AddDate(0, 0, 7*(w-1))
	sunday := monday.AddDate(0, 0, 6)

	return fmt.Sprintf("%s %d–%d", monday.Month().String()[:3], monday.Day(), sunday.Day())
}

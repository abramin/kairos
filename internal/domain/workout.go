package domain

import "time"

// WorkoutCategory classifies a workout log entry.
type WorkoutCategory string

const (
	WorkoutQigong       WorkoutCategory = "qigong"
	WorkoutCalisthenics WorkoutCategory = "calisthenics"
	WorkoutRunning      WorkoutCategory = "running"
	WorkoutKettlebell   WorkoutCategory = "kettlebell"
	WorkoutGMB          WorkoutCategory = "gmb"
	WorkoutStretching   WorkoutCategory = "stretching"
	WorkoutOther        WorkoutCategory = "other"
)

// ValidWorkoutCategories is the canonical set of accepted workout category strings.
var ValidWorkoutCategories = map[string]bool{
	"qigong": true, "calisthenics": true, "running": true,
	"kettlebell": true, "gmb": true, "stretching": true, "other": true,
}

// WorkoutCategoryLabel maps each category to a human-friendly display label.
var WorkoutCategoryLabel = map[WorkoutCategory]string{
	WorkoutQigong:       "Qigong",
	WorkoutCalisthenics: "Calisthenics",
	WorkoutRunning:      "Running",
	WorkoutKettlebell:   "Kettlebell",
	WorkoutGMB:          "GMB Movement",
	WorkoutStretching:   "Stretching",
	WorkoutOther:        "Other",
}

// WorkoutLog represents a single physical training session.
type WorkoutLog struct {
	ID          string
	Category    WorkoutCategory
	Minutes     int
	PerformedAt time.Time
	Notes       *string
	CreatedAt   time.Time
}

// SegmentKind distinguishes project session segments from workout segments in charts.
type SegmentKind string

const (
	SegmentProject SegmentKind = "project"
	SegmentWorkout SegmentKind = "workout"
)

// CategorySegment is one slice of a weekly bar chart — a labeled time amount.
type CategorySegment struct {
	Label   string
	Minutes int
	Kind    SegmentKind
}

// WeeklyBreakdown aggregates all time entries for a single ISO week.
type WeeklyBreakdown struct {
	ISOWeek   string            // e.g. "2026-W08"
	WeekLabel string            // e.g. "Feb 16–22"
	Segments  []CategorySegment // ordered by minutes desc
	TotalMin  int
}

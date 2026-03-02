package formatter

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
)

// WorkoutCategoryBadge returns a colored category label.
func WorkoutCategoryBadge(cat domain.WorkoutCategory) string {
	switch cat {
	case domain.WorkoutQigong:
		return StyleRed.Render("qigong")
	case domain.WorkoutCalisthenics:
		return StyleHeader.Render("calisthenics") // orange
	case domain.WorkoutRunning:
		return StyleYellow.Render("running")
	case domain.WorkoutKettlebell:
		return StyleYellowBold.Render("kettlebell")
	case domain.WorkoutGMB:
		return StyleRed.Render("gmb")
	case domain.WorkoutStretching:
		return StyleGreen.Render("stretching")
	default:
		return StyleDim.Render(string(cat))
	}
}

// FormatWorkoutList renders a table of workout logs.
func FormatWorkoutList(logs []domain.WorkoutLog) string {
	headers := []string{"DATE", "CATEGORY", "DURATION", "NOTES", "ID"}
	rows := make([][]string, 0, len(logs))

	totalMin := 0
	for _, w := range logs {
		totalMin += w.Minutes

		notePreview := ""
		if w.Notes != nil {
			notePreview = *w.Notes
			if len(notePreview) > 30 {
				notePreview = notePreview[:27] + "..."
			}
		}
		rows = append(rows, []string{
			HumanDate(w.PerformedAt),
			WorkoutCategoryBadge(w.Category),
			FormatMinutes(w.Minutes),
			Dim(notePreview),
			TruncID(w.ID),
		})
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%d workouts", len(logs)))
	parts = append(parts, Bold(FormatMinutes(totalMin))+" total")
	summary := Dim(strings.Join(parts, " · "))

	return RenderBox("Workouts", RenderTable(headers, rows)+"\n"+summary)
}

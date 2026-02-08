package scheduler

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeCandidate(projectName, workItemID string, risk domain.RiskLevel, dueDate *time.Time, score float64) ScoredCandidate {
	return ScoredCandidate{
		Input: ScoringInput{
			WorkItemID:  workItemID,
			ProjectName: projectName,
			ProjectRisk: risk,
			DueDate:     dueDate,
		},
		Score: score,
	}
}

func TestCanonicalSort_RiskPriority(t *testing.T) {
	due := time.Now().Add(7 * 24 * time.Hour)
	candidates := []ScoredCandidate{
		makeCandidate("On Track", "wi-1", domain.RiskOnTrack, &due, 50),
		makeCandidate("Critical", "wi-2", domain.RiskCritical, &due, 50),
		makeCandidate("At Risk", "wi-3", domain.RiskAtRisk, &due, 50),
	}

	CanonicalSort(candidates)

	assert.Equal(t, domain.RiskCritical, candidates[0].Input.ProjectRisk, "critical should be first")
	assert.Equal(t, domain.RiskAtRisk, candidates[1].Input.ProjectRisk, "at_risk should be second")
	assert.Equal(t, domain.RiskOnTrack, candidates[2].Input.ProjectRisk, "on_track should be last")
}

func TestCanonicalSort_DueDateTiebreak(t *testing.T) {
	earlyDue := time.Now().Add(3 * 24 * time.Hour)
	lateDue := time.Now().Add(10 * 24 * time.Hour)

	candidates := []ScoredCandidate{
		makeCandidate("Late", "wi-1", domain.RiskOnTrack, &lateDue, 50),
		makeCandidate("Early", "wi-2", domain.RiskOnTrack, &earlyDue, 50),
	}

	CanonicalSort(candidates)

	assert.Equal(t, "Early", candidates[0].Input.ProjectName, "earlier due date should sort first")
	assert.Equal(t, "Late", candidates[1].Input.ProjectName)
}

func TestCanonicalSort_NilDueDateLast(t *testing.T) {
	due := time.Now().Add(7 * 24 * time.Hour)

	candidates := []ScoredCandidate{
		makeCandidate("No Due", "wi-1", domain.RiskOnTrack, nil, 50),
		makeCandidate("Has Due", "wi-2", domain.RiskOnTrack, &due, 50),
	}

	CanonicalSort(candidates)

	assert.Equal(t, "Has Due", candidates[0].Input.ProjectName, "non-nil due date should sort before nil")
	assert.Equal(t, "No Due", candidates[1].Input.ProjectName)
}

func TestCanonicalSort_ScoreTiebreak(t *testing.T) {
	due := time.Now().Add(7 * 24 * time.Hour)

	candidates := []ScoredCandidate{
		makeCandidate("Same", "wi-1", domain.RiskOnTrack, &due, 30),
		makeCandidate("Same", "wi-2", domain.RiskOnTrack, &due, 80),
	}

	CanonicalSort(candidates)

	assert.Equal(t, 80.0, candidates[0].Score, "higher score should sort first")
	assert.Equal(t, 30.0, candidates[1].Score)
}

func TestCanonicalSort_StableTiebreak_NameThenID(t *testing.T) {
	due := time.Now().Add(7 * 24 * time.Hour)

	candidates := []ScoredCandidate{
		makeCandidate("Bravo", "wi-2", domain.RiskOnTrack, &due, 50),
		makeCandidate("Alpha", "wi-1", domain.RiskOnTrack, &due, 50),
		makeCandidate("Alpha", "wi-3", domain.RiskOnTrack, &due, 50),
	}

	CanonicalSort(candidates)

	require.Len(t, candidates, 3)
	assert.Equal(t, "Alpha", candidates[0].Input.ProjectName, "alphabetically earlier name first")
	assert.Equal(t, "wi-1", candidates[0].Input.WorkItemID, "within same project, lower ID first")
	assert.Equal(t, "Alpha", candidates[1].Input.ProjectName)
	assert.Equal(t, "wi-3", candidates[1].Input.WorkItemID)
	assert.Equal(t, "Bravo", candidates[2].Input.ProjectName)
}

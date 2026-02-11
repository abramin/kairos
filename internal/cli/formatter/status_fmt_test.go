package formatter

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/contract"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatStatus_IncludesPolicyWarningsAndFallbackDueDate(t *testing.T) {
	badDueDate := "not-a-date"
	resp := &contract.StatusResponse{
		Summary: contract.GlobalStatusSummary{
			CountsCritical: 1,
			CountsAtRisk:   0,
			CountsOnTrack:  0,
			PolicyMessage:  "Critical work requires attention",
		},
		Projects: []contract.ProjectStatusView{
			{
				ProjectName:     "Chemistry Prep",
				Status:          domain.ProjectActive,
				RiskLevel:       domain.RiskCritical,
				DueDate:         &badDueDate,
				ProgressTimePct: 35.0,
			},
		},
		Warnings: []string{"Projected overload this week"},
	}

	out := FormatStatus(resp)
	assert.Contains(t, out, "Chemistry Prep")
	assert.Contains(t, out, "not-a-date")
	assert.Contains(t, out, "Critical work requires attention")
	assert.Contains(t, out, "Projected overload this week")
}


package formatter

import (
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatProjectList_UsesShortIDWhenPresent(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "12345678-aaaa-bbbb-cccc-1234567890ab",
			ShortID:   "PSY01",
			Name:      "Psychology OU - Module 1",
			Domain:    "Education",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "PSY01")
	assert.NotContains(t, out, "12345678")
}

func TestFormatProjectList_FallsBackToUUIDPrefixWhenShortIDMissing(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "abcdef12-3456-7890-abcd-ef1234567890",
			ShortID:   "",
			Name:      "Psychology OU - Module 1",
			Domain:    "Education",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "abcdef12")
}

func TestFormatProjectList_UsesPlaceholderWhenIDAndShortIDMissing(t *testing.T) {
	now := time.Now().UTC()
	projects := []*domain.Project{
		{
			ID:        "",
			ShortID:   "",
			Name:      "Untitled",
			Domain:    "General",
			Status:    domain.ProjectActive,
			StartDate: now,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	out := FormatProjectList(projects)

	assert.Contains(t, out, "--")
}

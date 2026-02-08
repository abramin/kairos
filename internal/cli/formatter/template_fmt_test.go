package formatter

import (
	"testing"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatTemplateList(t *testing.T) {
	out := FormatTemplateList([]domain.Template{
		{NumericID: 1, Name: "OU Weekly", Domain: "education", Version: "1.0.0"},
		{NumericID: 2, Name: "Marathon", Domain: "fitness", Version: "2.1.0"},
	})

	assert.Contains(t, out, "TEMPLATES")
	assert.Contains(t, out, "OU Weekly")
	assert.Contains(t, out, "Marathon")
	assert.Contains(t, out, "1.0.0")
	assert.Contains(t, out, "2.1.0")
}

func TestFormatTemplateShow_PrettyAndRawJSON(t *testing.T) {
	pretty := FormatTemplateShow(&domain.Template{
		NumericID:  7,
		ID:         "ou_weekly",
		Name:       "OU Weekly",
		Domain:     "education",
		Version:    "1.0.0",
		ConfigJSON: `{"nodes":[{"title":"Week 1"}]}`,
	})
	assert.Contains(t, pretty, "OU Weekly")
	assert.Contains(t, pretty, "NUM ID")
	assert.Contains(t, pretty, "\"nodes\"")
	assert.Contains(t, pretty, "\"title\": \"Week 1\"")

	raw := FormatTemplateShow(&domain.Template{
		NumericID:  8,
		ID:         "broken",
		Name:       "Broken",
		Domain:     "general",
		Version:    "1.0.0",
		ConfigJSON: "{invalid-json",
	})
	assert.Contains(t, raw, "{invalid-json")
}

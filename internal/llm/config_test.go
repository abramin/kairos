package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig_ParseTimeoutMatchesGlobalDefault(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 10000, cfg.Tasks[TaskParse].TimeoutMs)
}

func TestLoadConfig_TaskTimeoutOverrides(t *testing.T) {
	t.Setenv("KAIROS_LLM_TIMEOUT_MS", "9000")
	t.Setenv("KAIROS_LLM_PARSE_TIMEOUT_MS", "15000")
	t.Setenv("KAIROS_LLM_EXPLAIN_TIMEOUT_MS", "7000")

	cfg := LoadConfig()

	assert.Equal(t, 9000, cfg.TimeoutMs)
	assert.Equal(t, 15000, cfg.TaskTimeout(TaskParse))
	assert.Equal(t, 7000, cfg.TaskTimeout(TaskExplain))
	assert.Equal(t, 8000, cfg.TaskTimeout(TaskTemplateDraft))
}

func TestLoadConfig_InvalidTaskTimeoutOverrideIgnored(t *testing.T) {
	t.Setenv("KAIROS_LLM_PARSE_TIMEOUT_MS", "not-a-number")

	cfg := LoadConfig()

	assert.Equal(t, 10000, cfg.TaskTimeout(TaskParse))
}

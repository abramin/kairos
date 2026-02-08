package llm

import (
	"os"
	"strconv"
)

// TaskType identifies the kind of LLM task being performed.
type TaskType string

const (
	TaskParse         TaskType = "parse"
	TaskExplain       TaskType = "explain"
	TaskTemplateDraft TaskType = "template_draft"
	TaskProjectDraft  TaskType = "project_draft"
	TaskHelp          TaskType = "help"
)

// TaskConfig holds per-task LLM parameters.
type TaskConfig struct {
	Temperature float64
	MaxTokens   int
	TimeoutMs   int // overrides global if > 0
}

// LLMConfig holds all configuration for the LLM subsystem.
type LLMConfig struct {
	Enabled             bool
	LogCalls            bool
	Endpoint            string
	Model               string
	TimeoutMs           int
	MaxRetries          int
	ConfidenceThreshold float64
	Tasks               map[TaskType]TaskConfig
}

// DefaultConfig returns an LLMConfig with sensible defaults.
// LLM is disabled by default.
func DefaultConfig() LLMConfig {
	return LLMConfig{
		Enabled:             false,
		LogCalls:            false,
		Endpoint:            "http://localhost:11434",
		Model:               "llama3.2",
		TimeoutMs:           10000,
		MaxRetries:          1,
		ConfidenceThreshold: 0.85,
		Tasks: map[TaskType]TaskConfig{
			TaskParse:         {Temperature: 0.1, MaxTokens: 512, TimeoutMs: 10000},
			TaskExplain:       {Temperature: 0.3, MaxTokens: 1024, TimeoutMs: 6000},
			TaskTemplateDraft: {Temperature: 0.2, MaxTokens: 2048, TimeoutMs: 8000},
			TaskProjectDraft:  {Temperature: 0.3, MaxTokens: 4096, TimeoutMs: 30000},
			TaskHelp:          {Temperature: 0.2, MaxTokens: 1024, TimeoutMs: 8000},
		},
	}
}

// LoadConfig reads LLM configuration from environment variables,
// falling back to defaults for any unset values.
func LoadConfig() LLMConfig {
	cfg := DefaultConfig()

	if v := os.Getenv("KAIROS_LLM_ENABLED"); v != "" {
		cfg.Enabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("KAIROS_LLM_LOG_CALLS"); v != "" {
		cfg.LogCalls, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("KAIROS_LLM_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("KAIROS_LLM_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("KAIROS_LLM_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TimeoutMs = n
		}
	}
	if v := os.Getenv("KAIROS_LLM_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv("KAIROS_LLM_CONFIDENCE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.ConfidenceThreshold = f
		}
	}

	applyTaskTimeoutEnv(&cfg, TaskParse, "KAIROS_LLM_PARSE_TIMEOUT_MS")
	applyTaskTimeoutEnv(&cfg, TaskExplain, "KAIROS_LLM_EXPLAIN_TIMEOUT_MS")
	applyTaskTimeoutEnv(&cfg, TaskTemplateDraft, "KAIROS_LLM_TEMPLATE_DRAFT_TIMEOUT_MS")
	applyTaskTimeoutEnv(&cfg, TaskProjectDraft, "KAIROS_LLM_PROJECT_DRAFT_TIMEOUT_MS")
	applyTaskTimeoutEnv(&cfg, TaskHelp, "KAIROS_LLM_HELP_TIMEOUT_MS")

	return cfg
}

// TaskTimeout returns the effective timeout for a given task type.
// Uses the task-specific timeout if set, otherwise the global timeout.
func (c LLMConfig) TaskTimeout(task TaskType) int {
	if tc, ok := c.Tasks[task]; ok && tc.TimeoutMs > 0 {
		return tc.TimeoutMs
	}
	return c.TimeoutMs
}

func applyTaskTimeoutEnv(cfg *LLMConfig, task TaskType, envName string) {
	v := os.Getenv(envName)
	if v == "" {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return
	}
	tc := cfg.Tasks[task]
	tc.TimeoutMs = n
	cfg.Tasks[task] = tc
}

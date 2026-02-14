package intelligence

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntentService_Parse_Timeout_GracefulError verifies that when the LLM
// server is slow to respond (exceeds timeout), the service:
// 1. Returns an error (doesn't hang indefinitely)
// 2. Respects the configured timeout
// 3. Provides a helpful error message
//
// This prevents CLI hangs when using slow/unresponsive LLM servers.
func TestIntentService_Parse_Timeout_GracefulError(t *testing.T) {
	// Create slow httptest server that responds with delay
	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay, but check context to avoid blocking server.Close()
		select {
		case <-time.After(10 * time.Second):
		case <-r.Context().Done():
			return // Context cancelled, exit handler
		}
	}))
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0 // don't retry on timeout

	// Override Parse task timeout to 1 second
	parseTask := cfg.Tasks[llm.TaskParse]
	parseTask.TimeoutMs = 1000
	cfg.Tasks[llm.TaskParse] = parseTask

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))

	start := time.Now()
	_, err := svc.Parse(context.Background(), "What should I work on?")
	elapsed := time.Since(start)

	// Verify timeout enforced (doesn't hang)
	require.Error(t, err, "should return error on timeout")
	assert.Contains(t, err.Error(), "timed out",
		"error message should mention timeout")
	assert.Less(t, elapsed, 3*time.Second,
		"should timeout within configured limit + small overhead")
}

// TestExplainService_ExplainNow_Timeout_DeterministicFallback verifies that
// ExplainService gracefully falls back to deterministic explanations when
// the LLM times out. This ensures users always get an explanation, even if
// the LLM is slow/unavailable.
func TestExplainService_ExplainNow_Timeout_DeterministicFallback(t *testing.T) {
	// Slow server that respects context cancellation
	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(10 * time.Second):
		case <-r.Context().Done():
			return
		}
	}))
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.MaxRetries = 0

	// Override Explain task timeout to 500ms
	explainTask := cfg.Tasks[llm.TaskExplain]
	explainTask.TimeoutMs = 500
	cfg.Tasks[llm.TaskExplain] = explainTask

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewExplainService(client, llm.NoopObserver{})

	// Create minimal trace for deterministic fallback
	trace := RecommendationTrace{
		Mode:         "balanced",
		RequestedMin: 60,
		AllocatedMin: 60,
		Recommendations: []RecommendationTraceItem{
			{
				WorkItemID:   "wi-123",
				Title:        "Test Task",
				ProjectID:    "proj-1",
				AllocatedMin: 30,
				Score:        100.0,
				RiskLevel:    "on_track",
				Reasons: []ReasonTraceItem{
					{Code: "DEADLINE_PRESSURE", Message: "Due soon"},
				},
			},
		},
		Blockers:       []BlockerTraceItem{},
		RiskProjects:   []RiskTraceItem{},
		PolicyMessages: []string{},
	}

	start := time.Now()
	explanation, err := svc.ExplainNow(context.Background(), trace)
	elapsed := time.Since(start)

	// Verify fallback happened quickly (didn't wait full 10 seconds)
	assert.Less(t, elapsed, 2*time.Second,
		"should fall back to deterministic explanation quickly on timeout")

	// ExplainNow should return deterministic fallback, not error
	require.NoError(t, err, "ExplainNow should fall back gracefully")
	assert.NotEmpty(t, explanation.SummaryShort,
		"should provide deterministic explanation on LLM timeout")
	assert.True(t, explanation.SummaryShort != "" || explanation.SummaryDetailed != "",
		"fallback explanation should have content")
}

// TestLLMClient_Timeout_ContextCancellation verifies that the LLM client
// respects context cancellation in addition to configured timeout. This ensures
// users can interrupt long-running LLM calls (e.g., Ctrl+C in CLI).
func TestLLMClient_Timeout_ContextCancellation(t *testing.T) {
	// Server that respects context cancellation
	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
			return
		}
	}))
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.TimeoutMs = 10000 // long timeout (10s), but context will cancel first
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})

	// Create context that cancels after 500ms
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Generate(ctx, llm.GenerateRequest{
		Task:       llm.TaskParse,
		UserPrompt: "test",
	})
	elapsed := time.Since(start)

	// Verify context cancellation honored (doesn't wait full 5 seconds)
	require.Error(t, err, "should return error on context cancellation")
	assert.Less(t, elapsed, 2*time.Second,
		"should cancel within context timeout + small overhead")
}

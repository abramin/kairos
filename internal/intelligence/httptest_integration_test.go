package intelligence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHTTPTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("skipping HTTP integration test: local listener unavailable (%v)", r)
			}
		}()
		srv = httptest.NewServer(handler)
	}()
	return srv
}

// TestIntentService_Parse_WithHTTPTestServer exercises the full HTTP serialization
// path: httptest server → OllamaClient → IntentService.Parse → write safety
// enforcement → confirmation policy. This validates that no mock-drift exists
// between the Ollama HTTP response format and the intelligence layer's parsing.
func TestIntentService_Parse_WithHTTPTestServer(t *testing.T) {
	intent := ParsedIntent{
		Intent:               IntentWhatNow,
		Risk:                 RiskReadOnly,
		Arguments:            map[string]interface{}{"available_min": float64(45)},
		Confidence:           0.92,
		RequiresConfirmation: false,
		ClarificationOptions: []string{},
	}
	intentJSON, err := json.Marshal(intent)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(intentJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))

	res, err := svc.Parse(context.Background(), "What should I work on for 45 minutes?")
	require.NoError(t, err)

	assert.Equal(t, StateExecuted, res.ExecutionState,
		"high-confidence read-only intent should auto-execute")
	assert.Equal(t, IntentWhatNow, res.ParsedIntent.Intent)
	assert.Equal(t, RiskReadOnly, res.ParsedIntent.Risk)
	assert.InDelta(t, 0.92, res.ParsedIntent.Confidence, 0.01)
}

// TestIntentService_Parse_WriteSafety_WithHTTPTestServer verifies that even when
// the LLM (via real HTTP) classifies a write intent as "safe", the write safety
// enforcement still requires confirmation.
func TestIntentService_Parse_WriteSafety_WithHTTPTestServer(t *testing.T) {
	intent := ParsedIntent{
		Intent:               IntentProjectRemove,
		Risk:                 RiskReadOnly, // LLM incorrectly says read-only
		Arguments:            map[string]interface{}{"project_id": "abc-123"},
		Confidence:           0.99,
		RequiresConfirmation: false, // LLM says no confirmation needed
		ClarificationOptions: []string{},
	}
	intentJSON, err := json.Marshal(intent)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(intentJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewIntentService(client, llm.NoopObserver{}, DefaultConfirmationPolicy(0.85))

	res, err := svc.Parse(context.Background(), "delete my physics project")
	require.NoError(t, err)

	// EnforceWriteSafety should override the LLM's risk classification.
	assert.Equal(t, RiskWrite, res.ParsedIntent.Risk,
		"write safety enforcement should override LLM risk for known write intents")
	assert.Equal(t, StateNeedsConfirmation, res.ExecutionState,
		"write intent should require confirmation regardless of LLM classification")
}

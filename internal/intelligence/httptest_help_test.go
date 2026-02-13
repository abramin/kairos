package intelligence

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/alexanderramin/kairos/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelpService_Ask_WithHTTPTestServer exercises the full HTTP serialization
// path for HelpService: httptest server → OllamaClient → HelpService.Ask →
// grounding validation. This validates no mock-drift between the Ollama HTTP
// response format and the help layer's parsing + grounding filter.
func TestHelpService_Ask_WithHTTPTestServer(t *testing.T) {
	helpResp := HelpAnswer{
		Answer: "Use `kairos status` to view project health.",
		Examples: []ShellExample{
			{Command: "kairos status", Description: "Show project status"},
		},
		NextCommands: []string{"kairos what-now"},
		Confidence:   0.91,
	}
	respJSON, err := json.Marshal(helpResp)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(respJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return // skipped in newHTTPTestServer
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewHelpService(client, llm.NoopObserver{})

	answer, err := svc.Ask(context.Background(), "how do I check status?", testHelpCommandSpec)
	require.NoError(t, err)

	assert.Equal(t, "llm", answer.Source)
	assert.Contains(t, answer.Answer, "status")
	require.NotEmpty(t, answer.Examples)
	assert.Equal(t, "kairos status", answer.Examples[0].Command)
}

// TestHelpService_Ask_HallucinatedCommand_WithHTTPTestServer verifies that
// grounding validation filters out hallucinated commands even when received
// via real HTTP transport.
func TestHelpService_Ask_HallucinatedCommand_WithHTTPTestServer(t *testing.T) {
	helpResp := HelpAnswer{
		Answer: "Try the deploy command.",
		Examples: []ShellExample{
			{Command: "kairos deploy --production", Description: "Deploy project"},
		},
		NextCommands: []string{"kairos rollback"},
		Confidence:   0.85,
	}
	respJSON, err := json.Marshal(helpResp)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(respJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewHelpService(client, llm.NoopObserver{})

	answer, err := svc.Ask(context.Background(), "how do I deploy?", testHelpCommandSpec)
	require.NoError(t, err)

	// Grounding validation should reject the hallucinated "deploy" command
	// and fall back to deterministic help.
	assert.Equal(t, "deterministic", answer.Source,
		"hallucinated commands should trigger fallback to deterministic help")
}

// TestHelpService_Chat_WithHTTPTestServer verifies multi-turn chat works
// through the full HTTP boundary.
func TestHelpService_Chat_WithHTTPTestServer(t *testing.T) {
	helpResp := HelpAnswer{
		Answer: "Use `kairos what-now --minutes 45` to get recommendations.",
		Examples: []ShellExample{
			{Command: "kairos what-now --minutes 45", Description: "Get recommendations"},
		},
		NextCommands: []string{"kairos status"},
		Confidence:   0.90,
	}
	respJSON, err := json.Marshal(helpResp)
	require.NoError(t, err)

	srv := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"model":    "test-model",
			"response": string(respJSON),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	if srv == nil {
		return
	}
	defer srv.Close()

	cfg := llm.DefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.Model = "test-model"
	cfg.MaxRetries = 0

	client := llm.NewOllamaClient(cfg, llm.NoopObserver{})
	svc := NewHelpService(client, llm.NoopObserver{})

	conv, first, err := svc.StartChat(context.Background(), "what should I do?", testHelpCommandSpec)
	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, "llm", first.Source)
	assert.NotEmpty(t, first.Answer)

	// Second turn.
	second, err := svc.NextTurn(context.Background(), conv, "and how do I check status?")
	require.NoError(t, err)
	assert.NotEmpty(t, second.Answer)
}

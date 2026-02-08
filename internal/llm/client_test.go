package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(endpoint string) LLMConfig {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = endpoint
	return cfg
}

func TestOllamaClient_Generate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req ollamaRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "llama3.2", req.Model)
		assert.False(t, req.Stream)
		assert.Equal(t, "system prompt", req.System)
		assert.Equal(t, "user prompt", req.Prompt)

		resp := ollamaResponse{
			Model:    "llama3.2",
			Response: `{"intent":"what_now"}`,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewOllamaClient(testConfig(srv.URL), NoopObserver{})
	resp, err := client.Generate(context.Background(), GenerateRequest{
		Task:         TaskParse,
		SystemPrompt: "system prompt",
		UserPrompt:   "user prompt",
	})

	require.NoError(t, err)
	assert.Equal(t, `{"intent":"what_now"}`, resp.Text)
	assert.Equal(t, "llama3.2", resp.Model)
	assert.GreaterOrEqual(t, resp.LatencyMs, int64(0))
}

func TestOllamaClient_Generate_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.Tasks = map[TaskType]TaskConfig{
		TaskParse: {Temperature: 0.1, MaxTokens: 512, TimeoutMs: 50},
	}

	client := NewOllamaClient(cfg, NoopObserver{})
	_, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	assert.ErrorIs(t, err, ErrTimeout)
}

func TestOllamaClient_Generate_Unavailable(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1") // nothing listening
	cfg.MaxRetries = 0
	cfg.Tasks = map[TaskType]TaskConfig{
		TaskParse: {Temperature: 0.1, MaxTokens: 512, TimeoutMs: 1000},
	}

	client := NewOllamaClient(cfg, NoopObserver{})
	_, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	assert.ErrorIs(t, err, ErrOllamaUnavailable)
}

func TestOllamaClient_Generate_RetryOnTransientError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		resp := ollamaResponse{Model: "llama3.2", Response: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 1

	client := NewOllamaClient(cfg, NoopObserver{})
	resp, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Text)
	assert.Equal(t, 2, attempts)
}

func TestOllamaClient_Generate_RetryAfterTimeout(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			time.Sleep(120 * time.Millisecond)
		}
		resp := ollamaResponse{Model: "llama3.2", Response: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 1
	cfg.Tasks = map[TaskType]TaskConfig{
		TaskParse: {Temperature: 0.1, MaxTokens: 512, TimeoutMs: 50},
	}

	client := NewOllamaClient(cfg, NoopObserver{})
	resp, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Text)
	assert.Equal(t, int32(2), attempts.Load())
}

func TestOllamaClient_Generate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 0

	client := NewOllamaClient(cfg, NoopObserver{})
	_, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrRetryExhausted)
}

func TestOllamaClient_Available_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewOllamaClient(testConfig(srv.URL), NoopObserver{})
	assert.True(t, client.Available(context.Background()))
}

func TestOllamaClient_Available_False(t *testing.T) {
	client := NewOllamaClient(testConfig("http://127.0.0.1:1"), NoopObserver{})
	assert.False(t, client.Available(context.Background()))
}

func TestOllamaClient_ObserverCalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{Model: "llama3.2", Response: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var captured LLMCallEvent
	obs := &captureObserver{fn: func(e LLMCallEvent) { captured = e }}

	client := NewOllamaClient(testConfig(srv.URL), obs)
	_, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	require.NoError(t, err)
	assert.Equal(t, TaskParse, captured.Task)
	assert.Equal(t, "llama3.2", captured.Model)
	assert.True(t, captured.Success)
	assert.GreaterOrEqual(t, captured.LatencyMs, int64(0))
}

func TestOllamaClient_ObserverTimeoutErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 0
	cfg.Tasks = map[TaskType]TaskConfig{
		TaskParse: {Temperature: 0.1, MaxTokens: 512, TimeoutMs: 50},
	}

	var captured LLMCallEvent
	obs := &captureObserver{fn: func(e LLMCallEvent) { captured = e }}
	client := NewOllamaClient(cfg, obs)

	_, err := client.Generate(context.Background(), GenerateRequest{
		Task:       TaskParse,
		UserPrompt: "test",
	})

	assert.ErrorIs(t, err, ErrTimeout)
	assert.False(t, captured.Success)
	assert.Equal(t, "TIMEOUT", captured.ErrorCode)
}

type captureObserver struct {
	fn func(LLMCallEvent)
}

func (o *captureObserver) OnCallComplete(e LLMCallEvent) { o.fn(e) }

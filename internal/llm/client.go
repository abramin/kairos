package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// GenerateRequest holds the parameters for an LLM generation call.
type GenerateRequest struct {
	Task         TaskType
	SystemPrompt string
	UserPrompt   string
	Temperature  *float64 // nil uses task default
	MaxTokens    *int     // nil uses task default
}

// GenerateResponse holds the result of an LLM generation call.
type GenerateResponse struct {
	Text      string
	Model     string
	LatencyMs int64
}

// LLMClient provides access to a language model for text generation.
type LLMClient interface {
	// Generate sends a prompt and returns the raw text response.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// Available checks whether the Ollama server is reachable.
	Available(ctx context.Context) bool
}

// ollamaClient implements LLMClient using the Ollama HTTP API.
type ollamaClient struct {
	cfg    LLMConfig
	http   *http.Client
	observer Observer
}

// NewOllamaClient creates an LLMClient that talks to a local Ollama instance.
func NewOllamaClient(cfg LLMConfig, observer Observer) LLMClient {
	if observer == nil {
		observer = NoopObserver{}
	}
	return &ollamaClient{
		cfg: cfg,
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
			},
		},
		observer: observer,
	}
}

// ollamaRequest is the JSON body sent to POST /api/generate.
type ollamaRequest struct {
	Model   string         `json:"model"`
	System  string         `json:"system,omitempty"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaResponse is the JSON body returned by POST /api/generate (non-streaming).
type ollamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
}

func (c *ollamaClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	start := time.Now()

	taskCfg := c.cfg.Tasks[req.Task]
	temp := taskCfg.Temperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	maxTok := taskCfg.MaxTokens
	if req.MaxTokens != nil {
		maxTok = *req.MaxTokens
	}

	timeoutMs := c.cfg.TaskTimeout(req.Task)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	body := ollamaRequest{
		Model:  c.cfg.Model,
		System: req.SystemPrompt,
		Prompt: req.UserPrompt,
		Stream: false,
		Options: ollamaOptions{
			Temperature: temp,
			NumPredict:  maxTok,
		},
	}

	var lastErr error
	attempts := 1 + c.cfg.MaxRetries

	for i := 0; i < attempts; i++ {
		resp, err := c.doRequest(ctx, body)
		if err == nil {
			latency := time.Since(start).Milliseconds()
			c.observer.OnCallComplete(LLMCallEvent{
				Task:      req.Task,
				Model:     c.cfg.Model,
				LatencyMs: latency,
				Success:   true,
			})
			return &GenerateResponse{
				Text:      resp.Response,
				Model:     resp.Model,
				LatencyMs: latency,
			}, nil
		}
		lastErr = err

		// Don't retry on context cancellation/timeout
		if ctx.Err() != nil {
			break
		}
	}

	latency := time.Since(start).Milliseconds()
	errCode := errorCode(lastErr)
	c.observer.OnCallComplete(LLMCallEvent{
		Task:      req.Task,
		Model:     c.cfg.Model,
		LatencyMs: latency,
		Success:   false,
		ErrorCode: errCode,
	})

	if ctx.Err() != nil {
		return nil, ErrTimeout
	}
	if isConnectionError(lastErr) {
		return nil, ErrOllamaUnavailable
	}
	return nil, fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
}

func (c *ollamaClient) doRequest(ctx context.Context, body ollamaRequest) (*ollamaResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := c.cfg.Endpoint + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp ollamaResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &resp, nil
}

func (c *ollamaClient) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	url := c.cfg.Endpoint + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

func errorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrTimeout):
		return "TIMEOUT"
	case errors.Is(err, ErrOllamaUnavailable):
		return "UNAVAILABLE"
	case errors.Is(err, ErrInvalidOutput):
		return "INVALID_OUTPUT"
	default:
		return "UNKNOWN"
	}
}

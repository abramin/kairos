package llm

import (
	"fmt"
	"io"
	"time"
)

// LLMCallEvent records metadata about a single LLM invocation.
type LLMCallEvent struct {
	Task      TaskType
	Model     string
	LatencyMs int64
	Success   bool
	ErrorCode string
}

// Observer receives events about LLM calls for logging and metrics.
type Observer interface {
	OnCallComplete(event LLMCallEvent)
}

// LogObserver writes LLM call events to an io.Writer.
type LogObserver struct {
	w io.Writer
}

// NewLogObserver creates an Observer that logs events to w.
func NewLogObserver(w io.Writer) *LogObserver {
	return &LogObserver{w: w}
}

func (o *LogObserver) OnCallComplete(event LLMCallEvent) {
	ts := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	if !event.Success {
		status = "err:" + event.ErrorCode
	}
	fmt.Fprintf(o.w, "[%s] llm_call task=%s model=%s latency_ms=%d status=%s\n",
		ts, event.Task, event.Model, event.LatencyMs, status)
}

// NoopObserver discards all events. Useful for tests.
type NoopObserver struct{}

func (NoopObserver) OnCallComplete(LLMCallEvent) {}

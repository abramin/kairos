package llm

import "errors"

var (
	// ErrOllamaUnavailable indicates the Ollama server is unreachable.
	ErrOllamaUnavailable = errors.New("ollama server unavailable")

	// ErrTimeout indicates the LLM request exceeded the configured timeout.
	ErrTimeout = errors.New("llm request timed out")

	// ErrInvalidOutput indicates the LLM response could not be parsed
	// into the expected structured format.
	ErrInvalidOutput = errors.New("invalid llm output format")

	// ErrRetryExhausted indicates all retry attempts have been exhausted.
	ErrRetryExhausted = errors.New("llm retry attempts exhausted")
)

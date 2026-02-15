package service

import (
	"context"
	"io"
	"log/slog"
	"time"
)

// UseCaseEvent captures lightweight execution telemetry for a service use case.
type UseCaseEvent struct {
	Name      string
	Duration  time.Duration
	Success   bool
	Err       error
	Fields    map[string]any
	StartedAt time.Time
}

// UseCaseObserver receives use-case execution events.
type UseCaseObserver interface {
	ObserveUseCase(ctx context.Context, event UseCaseEvent)
}

// NoopUseCaseObserver ignores all events.
type NoopUseCaseObserver struct{}

func (NoopUseCaseObserver) ObserveUseCase(context.Context, UseCaseEvent) {}

type logUseCaseObserver struct {
	logger *slog.Logger
}

// NewLogUseCaseObserver writes service use-case events to the provided writer.
func NewLogUseCaseObserver(w io.Writer) UseCaseObserver {
	if w == nil {
		return NoopUseCaseObserver{}
	}
	return &logUseCaseObserver{
		logger: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
}

func (o *logUseCaseObserver) ObserveUseCase(ctx context.Context, event UseCaseEvent) {
	attrs := make([]any, 0, 8+len(event.Fields)*2)
	attrs = append(attrs,
		"use_case", event.Name,
		"duration_ms", event.Duration.Milliseconds(),
		"success", event.Success,
	)
	for k, v := range event.Fields {
		attrs = append(attrs, k, v)
	}
	if event.Err != nil {
		attrs = append(attrs, "error", event.Err.Error())
		o.logger.ErrorContext(ctx, "service_use_case", attrs...)
		return
	}
	o.logger.InfoContext(ctx, "service_use_case", attrs...)
}

func useCaseObserverOrNoop(observers []UseCaseObserver) UseCaseObserver {
	for _, obs := range observers {
		if obs != nil {
			return obs
		}
	}
	return NoopUseCaseObserver{}
}

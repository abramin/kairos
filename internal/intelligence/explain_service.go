package intelligence

import (
	"context"
	"encoding/json"

	"github.com/alexanderramin/kairos/internal/llm"
)

// ExplainService generates faithful narrative explanations from engine traces.
type ExplainService interface {
	// ExplainNow generates an explanation for a what-now recommendation.
	ExplainNow(ctx context.Context, trace RecommendationTrace) (*LLMExplanation, error)

	// ExplainWhyNot explains why a specific candidate was not recommended.
	ExplainWhyNot(ctx context.Context, trace RecommendationTrace, candidateID string) (*LLMExplanation, error)

	// WeeklyReview generates a summary of the past week.
	WeeklyReview(ctx context.Context, trace WeeklyReviewTrace) (*LLMExplanation, error)
}

type explainService struct {
	client   llm.LLMClient
	observer llm.Observer
}

// NewExplainService creates an ExplainService backed by an LLM client.
func NewExplainService(client llm.LLMClient, observer llm.Observer) ExplainService {
	return &explainService{client: client, observer: observer}
}

func (s *explainService) ExplainNow(ctx context.Context, trace RecommendationTrace) (*LLMExplanation, error) {
	return s.generateExplanation(
		ctx, explainNowSystemPrompt, trace, trace.TraceKeys(),
		func() *LLMExplanation { return DeterministicExplainNow(trace) },
	)
}

func (s *explainService) ExplainWhyNot(ctx context.Context, trace RecommendationTrace, candidateID string) (*LLMExplanation, error) {
	prompt := struct {
		Trace       RecommendationTrace `json:"trace"`
		CandidateID string              `json:"candidate_id"`
	}{
		Trace:       trace,
		CandidateID: candidateID,
	}
	return s.generateExplanation(
		ctx, explainWhyNotSystemPrompt, prompt, trace.TraceKeys(),
		func() *LLMExplanation { return DeterministicWhyNot(trace, candidateID) },
	)
}

func (s *explainService) WeeklyReview(ctx context.Context, trace WeeklyReviewTrace) (*LLMExplanation, error) {
	return s.generateExplanation(
		ctx, weeklyReviewSystemPrompt, trace, trace.WeeklyTraceKeys(),
		func() *LLMExplanation { return DeterministicWeeklyReview(trace) },
	)
}

// generateExplanation is the shared pipeline: marshal → LLM call → extract JSON → validate evidence.
// On any failure, it falls back to the deterministic function.
func (s *explainService) generateExplanation(
	ctx context.Context,
	systemPrompt string,
	data any,
	validKeys map[string]bool,
	fallback func() *LLMExplanation,
) (*LLMExplanation, error) {
	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fallback(), nil
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: systemPrompt,
		UserPrompt:   string(dataJSON),
	})
	if err != nil {
		return fallback(), nil
	}

	explanation, err := llm.ExtractJSON[LLMExplanation](resp.Text, nil)
	if err != nil {
		return fallback(), nil
	}

	if valErr := ValidateEvidenceBindings(explanation.Factors, validKeys); valErr != nil {
		return fallback(), nil
	}

	return &explanation, nil
}

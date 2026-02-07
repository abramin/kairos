package intelligence

import (
	"context"
	"encoding/json"
	"fmt"

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
	traceJSON, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return DeterministicExplainNow(trace), nil
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: explainNowSystemPrompt,
		UserPrompt:   "Here is the recommendation trace:\n\n" + string(traceJSON),
	})
	if err != nil {
		return DeterministicExplainNow(trace), nil
	}

	explanation, err := llm.ExtractJSON[LLMExplanation](resp.Text, nil)
	if err != nil {
		return DeterministicExplainNow(trace), nil
	}

	// Validate evidence bindings against the trace.
	if valErr := ValidateEvidenceBindings(explanation.Factors, trace.TraceKeys()); valErr != nil {
		return DeterministicExplainNow(trace), nil
	}

	return &explanation, nil
}

func (s *explainService) ExplainWhyNot(ctx context.Context, trace RecommendationTrace, candidateID string) (*LLMExplanation, error) {
	prompt := struct {
		Trace       RecommendationTrace `json:"trace"`
		CandidateID string              `json:"candidate_id"`
	}{
		Trace:       trace,
		CandidateID: candidateID,
	}

	promptJSON, err := json.MarshalIndent(prompt, "", "  ")
	if err != nil {
		return DeterministicWhyNot(trace, candidateID), nil
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: explainWhyNotSystemPrompt,
		UserPrompt:   string(promptJSON),
	})
	if err != nil {
		return DeterministicWhyNot(trace, candidateID), nil
	}

	explanation, err := llm.ExtractJSON[LLMExplanation](resp.Text, nil)
	if err != nil {
		return DeterministicWhyNot(trace, candidateID), nil
	}

	// Build extended keys: trace keys + blocker keys.
	keys := trace.TraceKeys()
	if valErr := ValidateEvidenceBindings(explanation.Factors, keys); valErr != nil {
		return DeterministicWhyNot(trace, candidateID), nil
	}

	return &explanation, nil
}

func (s *explainService) WeeklyReview(ctx context.Context, trace WeeklyReviewTrace) (*LLMExplanation, error) {
	traceJSON, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return DeterministicWeeklyReview(trace), nil
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: weeklyReviewSystemPrompt,
		UserPrompt:   fmt.Sprintf("Here is the weekly review data:\n\n%s", string(traceJSON)),
	})
	if err != nil {
		return DeterministicWeeklyReview(trace), nil
	}

	explanation, err := llm.ExtractJSON[LLMExplanation](resp.Text, nil)
	if err != nil {
		return DeterministicWeeklyReview(trace), nil
	}

	if valErr := ValidateEvidenceBindings(explanation.Factors, trace.WeeklyTraceKeys()); valErr != nil {
		return DeterministicWeeklyReview(trace), nil
	}

	return &explanation, nil
}

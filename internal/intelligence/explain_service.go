package intelligence

import (
	"context"
	"encoding/json"

	"github.com/alexanderramin/kairos/internal/llm"
)

// itemSummariesResponse is the expected JSON shape from the LLM for SummarizeItems.
type itemSummariesResponse struct {
	Summaries map[string]string `json:"summaries"`
}

// summarizeItemPayload is the per-item data sent to the LLM.
type summarizeItemPayload struct {
	ID          string            `json:"work_item_id"`
	Title       string            `json:"title"`
	ProjectName string            `json:"project,omitempty"`
	DueDate     *string           `json:"due_date,omitempty"`
	RiskLevel   string            `json:"risk_level"`
	Reasons     []ReasonTraceItem `json:"reasons"`
}

// ExplainService generates faithful narrative explanations from engine traces.
type ExplainService interface {
	// ExplainNow generates an explanation for a what-now recommendation.
	ExplainNow(ctx context.Context, trace RecommendationTrace) (*LLMExplanation, error)

	// ExplainWhyNot explains why a specific candidate was not recommended.
	ExplainWhyNot(ctx context.Context, trace RecommendationTrace, candidateID string) (*LLMExplanation, error)

	// WeeklyReview generates a summary of the past week.
	WeeklyReview(ctx context.Context, trace WeeklyReviewTrace) (*LLMExplanation, error)

	// SummarizeItems generates a short natural-language summary for each recommended item.
	// Returns a map of work_item_id → 1-2 sentence explanation.
	SummarizeItems(ctx context.Context, trace RecommendationTrace, projectNames map[string]string) (map[string]string, error)
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

func (s *explainService) SummarizeItems(ctx context.Context, trace RecommendationTrace, projectNames map[string]string) (map[string]string, error) {
	// Build the per-item payload, filtering out generic reasons.
	items := make([]summarizeItemPayload, 0, len(trace.Recommendations))
	for _, rec := range trace.Recommendations {
		payload := summarizeItemPayload{
			ID:          rec.WorkItemID,
			Title:       rec.Title,
			ProjectName: projectNames[rec.ProjectID],
			DueDate:     rec.DueDate,
			RiskLevel:   rec.RiskLevel,
		}
		for _, r := range rec.Reasons {
			if !genericReasonCode[r.Code] {
				payload.Reasons = append(payload.Reasons, r)
			}
		}
		items = append(items, payload)
	}

	dataJSON, err := json.MarshalIndent(map[string]any{"items": items}, "", "  ")
	if err != nil {
		return DeterministicSummarizeItems(trace, projectNames), nil
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: summarizeItemsSystemPrompt,
		UserPrompt:   string(dataJSON),
	})
	if err != nil {
		return DeterministicSummarizeItems(trace, projectNames), nil
	}

	parsed, err := llm.ExtractJSON[itemSummariesResponse](resp.Text, nil)
	if err != nil {
		return DeterministicSummarizeItems(trace, projectNames), nil
	}

	// Validate: all returned keys must be real trace item IDs.
	validIDs := make(map[string]bool, len(trace.Recommendations))
	for _, rec := range trace.Recommendations {
		validIDs[rec.WorkItemID] = true
	}
	for id := range parsed.Summaries {
		if !validIDs[id] {
			return DeterministicSummarizeItems(trace, projectNames), nil
		}
	}

	return parsed.Summaries, nil
}

// genericReasonCode lists reason codes that add no unique per-item signal.
var genericReasonCode = map[string]bool{
	"VARIATION_BONUS":        true,
	"DOMAIN_VARIATION_BONUS": true,
	"ON_TRACK_SAFE_MIX":      true,
	"BOUNDS_APPLIED":         true,
	"SPACING_OK":             true,
	"SPACING_BLOCKED":        true,
}

// generateExplanation is the shared pipeline: marshal → LLM call → extract JSON → validate evidence.
// On any failure, it falls back to the deterministic function with Source="deterministic".
func (s *explainService) generateExplanation(
	ctx context.Context,
	systemPrompt string,
	data any,
	validKeys map[string]bool,
	fallback func() *LLMExplanation,
) (*LLMExplanation, error) {
	useFallback := func() (*LLMExplanation, error) {
		result := fallback()
		result.Source = "deterministic"
		return result, nil
	}

	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return useFallback()
	}

	resp, err := s.client.Generate(ctx, llm.GenerateRequest{
		Task:         llm.TaskExplain,
		SystemPrompt: systemPrompt,
		UserPrompt:   string(dataJSON),
	})
	if err != nil {
		return useFallback()
	}

	explanation, err := llm.ExtractJSON[LLMExplanation](resp.Text, nil)
	if err != nil {
		return useFallback()
	}

	if valErr := ValidateEvidenceBindings(explanation.Factors, validKeys); valErr != nil {
		return useFallback()
	}

	explanation.Source = "llm"
	return &explanation, nil
}

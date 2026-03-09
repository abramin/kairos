package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/intelligence"
)

// FormatExplanation renders an LLMExplanation for terminal output.
func FormatExplanation(e *intelligence.LLMExplanation) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s\n\n", StyleBold.Render(e.SummaryShort)))

	if e.SummaryDetailed != "" && e.SummaryDetailed != e.SummaryShort {
		b.WriteString(fmt.Sprintf("  %s\n\n", e.SummaryDetailed))
	}

	if len(e.Factors) > 0 {
		b.WriteString(Header("Factors"))
		b.WriteString("\n")
		for _, f := range e.Factors {
			icon := "+"
			style := StyleGreen
			if f.Direction == "push_against" {
				icon = "-"
				style = StyleRed
			}
			impact := Dim(fmt.Sprintf("[%s]", f.Impact))
			b.WriteString(fmt.Sprintf("  %s %s %s\n", style.Render(icon), f.Name, impact))
			b.WriteString(fmt.Sprintf("    %s\n", Dim(f.Summary)))
		}
		b.WriteString("\n")
	}

	if len(e.Counterfactuals) > 0 {
		b.WriteString(Header("What If"))
		b.WriteString("\n")
		for _, c := range e.Counterfactuals {
			b.WriteString(fmt.Sprintf("  %s %s\n", StyleYellow.Render(c.Label+":"), c.PredictedEffect))
		}
		b.WriteString("\n")
	}

	b.WriteString(Dim(fmt.Sprintf("  Confidence: %.0f%%\n", e.Confidence*100)))
	return RenderBox("Explanation", b.String())
}

// intentLabels maps intent names to human-readable descriptions used as fallback
// when the LLM does not populate the Rationale field.
var intentLabels = map[intelligence.IntentName]string{
	intelligence.IntentWhatNow:             "show current recommendations",
	intelligence.IntentStatus:              "show project status",
	intelligence.IntentReplan:              "rebalance and reorder your project items",
	intelligence.IntentProjectAdd:          "add a new project",
	intelligence.IntentProjectImport:       "import a project from a file",
	intelligence.IntentProjectUpdate:       "update a project",
	intelligence.IntentProjectArchive:      "archive a project",
	intelligence.IntentProjectRemove:       "remove a project",
	intelligence.IntentNodeAdd:             "add a node to a project",
	intelligence.IntentNodeUpdate:          "update a project node",
	intelligence.IntentNodeRemove:          "remove a project node",
	intelligence.IntentWorkAdd:             "add a work item",
	intelligence.IntentWorkUpdate:          "update a work item",
	intelligence.IntentWorkDone:            "mark a work item as done",
	intelligence.IntentWorkRemove:          "remove a work item",
	intelligence.IntentSessionLog:          "log a work session",
	intelligence.IntentSessionRemove:       "remove a session log entry",
	intelligence.IntentTemplateList:        "list available templates",
	intelligence.IntentTemplateShow:        "show a template",
	intelligence.IntentTemplateDraft:       "draft a new template",
	intelligence.IntentTemplateValidate:    "validate a template",
	intelligence.IntentProjectInitFromTmpl: "create a project from a template",
	intelligence.IntentExplainNow:          "explain the current recommendations",
	intelligence.IntentExplainWhyNot:       "explain why an item wasn't recommended",
	intelligence.IntentReviewWeekly:        "review your weekly progress",
	intelligence.IntentSimulate:            "simulate a scenario",
}

// FormatAskResolution renders the result of an `ask` command in natural language.
func FormatAskResolution(r *intelligence.AskResolution) string {
	var b strings.Builder
	intent := r.ParsedIntent

	switch r.ExecutionState {
	case intelligence.StateNeedsClarification:
		b.WriteString(fmt.Sprintf("  %s\n", StyleYellow.Render("I wasn't sure what you meant. Did you mean:")))
		for i, opt := range intent.ClarificationOptions {
			b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, opt))
		}
	case intelligence.StateRejected:
		b.WriteString(fmt.Sprintf("  %s\n", StyleRed.Render(fmt.Sprintf("I couldn't do that: %s", r.ExecutionMessage))))
	default:
		understood := intent.Rationale
		if understood == "" {
			if label, ok := intentLabels[intent.Intent]; ok {
				understood = label
			} else {
				understood = string(intent.Intent)
			}
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", StyleBold.Render("I understood:"), understood))

		if r.Explanation != "" {
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  %s\n", r.Explanation))
		}

		if intent.Confidence < 0.80 {
			b.WriteString(Dim(fmt.Sprintf("  Low confidence: %.0f%%\n", intent.Confidence*100)))
		}

		if r.CommandHint != "" && r.ExecutionState == intelligence.StateNeedsConfirmation {
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  %s\n", StyleGreen.Render("Run: "+r.CommandHint)))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  %s\n", StyleYellow.Render("This will modify your data. Run the command above to confirm.")))
		}
	}

	return RenderBox("Ask", b.String())
}

// FormatTemplateDraft renders a template draft result.
func FormatTemplateDraft(d *intelligence.TemplateDraft) string {
	var b strings.Builder

	if d.Validation.IsValid {
		b.WriteString(StyleGreen.Render("  Validation: PASSED"))
	} else {
		b.WriteString(StyleRed.Render("  Validation: FAILED"))
	}
	b.WriteString("\n\n")

	if len(d.Validation.Errors) > 0 {
		b.WriteString(StyleRed.Render("  Errors:"))
		b.WriteString("\n")
		for _, e := range d.Validation.Errors {
			b.WriteString(fmt.Sprintf("    - %s\n", e))
		}
		b.WriteString("\n")
	}

	if len(d.Validation.Warnings) > 0 {
		b.WriteString(StyleYellow.Render("  Warnings:"))
		b.WriteString("\n")
		for _, w := range d.Validation.Warnings {
			b.WriteString(fmt.Sprintf("    - %s\n", w))
		}
		b.WriteString("\n")
	}

	if len(d.RepairSuggestions) > 0 {
		b.WriteString(Dim("  Suggestions:"))
		b.WriteString("\n")
		for _, s := range d.RepairSuggestions {
			b.WriteString(fmt.Sprintf("    - %s\n", s))
		}
		b.WriteString("\n")
	}

	b.WriteString(Header("Preview"))
	b.WriteString("\n")
	preview, _ := json.MarshalIndent(d.TemplateJSON, "  ", "  ")
	b.WriteString(fmt.Sprintf("  %s\n", string(preview)))
	b.WriteString("\n")
	b.WriteString(Dim(fmt.Sprintf("  Confidence: %.0f%%\n", d.Confidence*100)))

	return RenderBox("Template Draft", b.String())
}


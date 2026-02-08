package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/charmbracelet/lipgloss"
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

// FormatAskResolution renders the result of an `ask` command.
func FormatAskResolution(r *intelligence.AskResolution) string {
	var b strings.Builder

	intent := r.ParsedIntent
	b.WriteString(fmt.Sprintf("  Intent:     %s\n", StyleBold.Render(string(intent.Intent))))
	b.WriteString(fmt.Sprintf("  Risk:       %s\n", riskStyle(intent.Risk).Render(string(intent.Risk))))
	b.WriteString(fmt.Sprintf("  Confidence: %.0f%%\n", intent.Confidence*100))

	if len(intent.Arguments) > 0 {
		b.WriteString(fmt.Sprintf("  Arguments:  %s\n", formatArgs(intent.Arguments)))
	}

	b.WriteString("\n")

	switch r.ExecutionState {
	case intelligence.StateExecuted:
		b.WriteString(StyleGreen.Render("  Auto-executing (read-only, high confidence)"))
	case intelligence.StateNeedsConfirmation:
		b.WriteString(StyleYellow.Render("  Requires confirmation. Proceed? [Y/n]"))
	case intelligence.StateNeedsClarification:
		b.WriteString(StyleYellow.Render("  Low confidence. Did you mean:"))
		b.WriteString("\n")
		for i, opt := range intent.ClarificationOptions {
			b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, opt))
		}
	case intelligence.StateRejected:
		b.WriteString(StyleRed.Render(fmt.Sprintf("  Rejected: %s", r.ExecutionMessage)))
	}

	b.WriteString("\n")
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

func riskStyle(risk intelligence.IntentRisk) lipgloss.Style {
	if risk == intelligence.RiskWrite {
		return StyleYellow
	}
	return StyleGreen
}

func formatArgs(args map[string]interface{}) string {
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return Dim(string(data))
}

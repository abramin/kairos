package formatter

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/intelligence"
)

const helpTextWrapWidth = 88

// FormatHelpAnswer renders a HelpAnswer for terminal output.
func FormatHelpAnswer(answer *intelligence.HelpAnswer) string {
	var b strings.Builder

	b.WriteString(indentWrapped(answer.Answer, 2, helpTextWrapWidth))
	b.WriteString("\n")

	if len(answer.Examples) > 0 {
		b.WriteString(Header("Examples"))
		b.WriteString("\n")
		for _, ex := range answer.Examples {
			b.WriteString(fmt.Sprintf("  %s\n", StyleGreen.Render("$ "+ex.Command)))
			if ex.Description != "" {
				for _, line := range strings.Split(wrapText(ex.Description, helpTextWrapWidth-4), "\n") {
					if strings.TrimSpace(line) == "" {
						continue
					}
					b.WriteString(fmt.Sprintf("    %s\n", Dim(line)))
				}
			}
		}
	}

	if len(answer.NextCommands) > 0 {
		b.WriteString("\n")
		b.WriteString(Header("Try Next"))
		b.WriteString("\n")
		for _, cmd := range answer.NextCommands {
			b.WriteString(fmt.Sprintf("  %s\n", StyleBlue.Render(cmd)))
		}
	}

	sourceLabel := "LLM"
	if answer.Source == "deterministic" {
		sourceLabel = "Local"
	}
	b.WriteString(fmt.Sprintf("\n  %s\n", Dim(fmt.Sprintf("[%s | Confidence: %.0f%%]", sourceLabel, answer.Confidence*100))))

	return RenderBox("Help", b.String())
}

func indentWrapped(text string, indent, width int) string {
	prefix := strings.Repeat(" ", indent)
	lines := strings.Split(wrapText(text, width), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
	}
	return b.String()
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return strings.TrimSpace(text)
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}

		words := strings.Fields(trimmed)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			if len(current)+1+len(word) <= width {
				current += " " + word
				continue
			}
			out = append(out, current)
			current = word
		}
		out = append(out, current)
	}

	return strings.Join(out, "\n")
}

// FormatHelpChatWelcome renders the welcome banner for interactive help chat.
func FormatHelpChatWelcome() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(StylePurple.Render("  kairos") + StyleDim.Render(" help agent"))
	b.WriteString("\n")
	b.WriteString(StyleDim.Render("  ─────────────────────────────") + "\n\n")
	b.WriteString(StyleDim.Render("  Ask anything about using Kairos.") + "\n")
	b.WriteString(StyleDim.Render("  Type /quit to exit, /commands to list all commands.") + "\n\n")
	return b.String()
}

// FormatCommandList renders a compact list of all commands from the spec.
func FormatCommandList(commands []intelligence.HelpCommandInfo) string {
	var b strings.Builder
	for _, cmd := range commands {
		b.WriteString(fmt.Sprintf("  %s  %s\n", StyleGreen.Render(cmd.FullPath), Dim(cmd.Short)))
	}
	return RenderBox("Commands", b.String())
}

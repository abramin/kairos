package intelligence

import (
	"fmt"
	"sort"
	"strings"
)

// HelpGlossary maps domain terms to their definitions.
// Embedded in the help agent system prompt so the LLM can reference
// precise definitions when answering user questions.
var HelpGlossary = map[string]string{
	"project":        "A top-level planning container with a name, domain, start date, optional target date, and status (active/paused/done/archived).",
	"plan_node":      "A hierarchical subdivision of a project (week, module, section, stage). Nodes form a tree under a project. Also called 'node'.",
	"work_item":      "A concrete task under a plan node with planned minutes, duration mode, and session policy. Also called 'work'.",
	"session":        "A logged time block against a work item. Records actual minutes spent and optional units completed.",
	"what-now":       "The recommendation engine. Given available minutes, scores and allocates work items across projects respecting deadlines, risk, spacing, and variation.",
	"status":         "An overview of project health: progress percentages, risk levels, on-track indicators.",
	"replan":         "Recalculates estimates and schedules based on current progress. Applies smoothing: new_planned = 0.7*old + 0.3*implied.",
	"risk_level":     "Computed per-project: critical (behind schedule, deadline imminent), at_risk (falling behind), on_track.",
	"template":       "A JSON schema that scaffolds project structure (nodes + work items) with variables and expressions.",
	"short_id":       "A human-friendly project identifier (e.g., PHYS01). 3-6 uppercase letters + 2-4 digits.",
	"duration_mode":  "How a work item's time is measured: 'fixed' (exact), 'estimate' (adjustable), 'derived' (from units).",
	"session_policy": "Min/max/default session minutes and splittable flag. Controls how work items are allocated into time blocks.",
	"scoring":        "6 weighted factors for ranking work items: deadline pressure, risk level, recency spacing, variation, completion momentum, and unit progress.",
	"allocation":     "Two-pass process: first enforce cross-project variation, then fill remaining time with top-scored items.",
	"import":         "Create a project from a JSON file (ImportSchema) with project metadata, nodes, work items, and dependencies.",
	"archive":        "Soft-delete via archived_at timestamp. Archived entities are excluded from recommendations but not destroyed.",
	"dependency":     "A predecessor-successor relationship between work items. Successors are blocked until predecessors are complete.",
	"domain":         "A category label for a project (e.g., education, fitness, software, personal).",
	"explain":        "LLM-powered narrative explanations of recommendations. Falls back to deterministic output when LLM is unavailable.",
}

// FormatGlossary serializes the glossary as a readable string for embedding
// in LLM system prompts.
func FormatGlossary() string {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(HelpGlossary))
	for k := range HelpGlossary {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, HelpGlossary[k]))
	}
	return b.String()
}

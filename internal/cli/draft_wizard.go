package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/importer"
)

// wizardGroup represents one group of repeating nodes (e.g., "A2 Module" x 9).
type wizardGroup struct {
	Label   string
	Count   int
	Kind    string
	DaysPer int // 0 = spread evenly or no due dates
}

// wizardWorkItem represents a work item template applied to every node.
type wizardWorkItem struct {
	Title      string
	Type       string
	PlannedMin int
}

// wizardSpecialNode represents a one-off node like an exam or milestone.
type wizardSpecialNode struct {
	Title     string
	Kind      string
	DueDate   string // YYYY-MM-DD, empty = use deadline
	WorkItems []wizardWorkItem
}

// wizardResult holds all data collected by the structure wizard.
type wizardResult struct {
	Description  string
	StartDate    string
	Deadline     string
	Groups       []wizardGroup
	WorkItems    []wizardWorkItem
	SpecialNodes []wizardSpecialNode
}

// validNodeKinds and validWorkItemTypes alias the canonical domain sets.
var (
	validNodeKinds     = domain.ValidNodeKinds
	validWorkItemTypes = domain.ValidWorkItemTypes
)

// collectWorkItem prompts for a single work item (title, type, minutes).
// Returns nil when the user enters an empty title (signals "done").
func collectWorkItem(in io.Reader, indent string) (*wizardWorkItem, error) {
	fmt.Printf("%sTitle (Enter when done): ", indent)
	title, err := readDraftLine(in)
	if err != nil {
		return nil, err
	}
	if title == "" {
		return nil, nil
	}

	wi := wizardWorkItem{Title: title, Type: "task"}

	fmt.Printf("%s  Type [reading/practice/review/assignment/task/quiz/study]: ", indent)
	wiType, err := readDraftLine(in)
	if err != nil {
		return nil, err
	}
	if wiType != "" {
		wiType = strings.ToLower(wiType)
		if !validWorkItemTypes[wiType] {
			fmt.Fprintf(os.Stderr, "%s  Invalid type, using task.\n", indent)
		} else {
			wi.Type = wiType
		}
	}

	fmt.Printf("%s  Estimated minutes: ", indent)
	minStr, err := readDraftLine(in)
	if err != nil {
		return nil, err
	}
	if minStr != "" {
		mins, err := strconv.Atoi(minStr)
		if err != nil || mins < 1 {
			fmt.Fprintf(os.Stderr, "%s  Invalid number, using 30.\n", indent)
			mins = 30
		}
		wi.PlannedMin = mins
	}

	return &wi, nil
}

// runStructureWizard collects project structure interactively.
func runStructureWizard(in io.Reader) (*wizardResult, error) {
	result := &wizardResult{}

	// --- Groups ---
	fmt.Print("\n  How many groups of work? (e.g., phases, levels — Enter for 1)\n  > ")
	groupCountStr, err := readDraftLine(in)
	if err != nil {
		return nil, err
	}
	groupCount := 1
	if groupCountStr != "" {
		n, err := strconv.Atoi(groupCountStr)
		if err != nil || n < 1 {
			fmt.Fprintf(os.Stderr, "  Invalid number, using 1.\n")
		} else {
			groupCount = n
		}
	}

	for g := 0; g < groupCount; g++ {
		group := wizardGroup{Kind: "module"}

		if groupCount > 1 {
			fmt.Printf("\n  --- Group %d ---\n", g+1)
			fmt.Print("  Label (e.g., \"Chapter\", \"Week\", \"A2 Module\"): ")
			label, err := readDraftLine(in)
			if err != nil {
				return nil, err
			}
			group.Label = label
		} else {
			fmt.Print("\n  Node label (e.g., \"Chapter\", \"Week\", \"Module\"): ")
			label, err := readDraftLine(in)
			if err != nil {
				return nil, err
			}
			group.Label = label
		}
		if group.Label == "" {
			group.Label = "Module"
		}

		fmt.Print("  How many? ")
		countStr, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		count, err := strconv.Atoi(countStr)
		if err != nil || count < 1 {
			fmt.Fprintf(os.Stderr, "  Invalid number, using 1.\n")
			count = 1
		}
		group.Count = count

		fmt.Print("  Node kind [module/week/section/stage/assessment/generic] (Enter for module): ")
		kind, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		if kind != "" {
			kind = strings.ToLower(kind)
			if !validNodeKinds[kind] {
				fmt.Fprintf(os.Stderr, "  Invalid kind, using module.\n")
			} else {
				group.Kind = kind
			}
		}

		fmt.Print("  Days per node (Enter to spread evenly): ")
		daysStr, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		if daysStr != "" {
			days, err := strconv.Atoi(daysStr)
			if err != nil || days < 1 {
				fmt.Fprintf(os.Stderr, "  Invalid number, skipping.\n")
			} else {
				group.DaysPer = days
			}
		}

		result.Groups = append(result.Groups, group)
	}

	// --- Work Items ---
	fmt.Print("\n  --- Work Items (applied to every node) ---\n")
	for {
		wi, err := collectWorkItem(in, "  ")
		if err != nil {
			return nil, err
		}
		if wi == nil {
			break
		}
		result.WorkItems = append(result.WorkItems, *wi)
	}

	// --- Special Nodes ---
	fmt.Print("\n  --- Special Nodes (exams, milestones — Enter to skip) ---\n")
	for {
		fmt.Print("  Title (Enter to skip): ")
		title, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		if title == "" {
			break
		}

		sn := wizardSpecialNode{Title: title, Kind: "assessment"}

		fmt.Print("    Kind [assessment/generic] (Enter for assessment): ")
		kind, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		if kind != "" {
			kind = strings.ToLower(kind)
			if kind != "assessment" && kind != "generic" {
				fmt.Fprintf(os.Stderr, "    Invalid kind, using assessment.\n")
			} else {
				sn.Kind = kind
			}
		}

		fmt.Print("    Due date (YYYY-MM-DD, Enter for deadline): ")
		dueStr, err := readDraftLine(in)
		if err != nil {
			return nil, err
		}
		if dueStr != "" {
			if _, err := time.Parse("2006-01-02", dueStr); err != nil {
				fmt.Fprintf(os.Stderr, "    Invalid date, skipping.\n")
			} else {
				sn.DueDate = dueStr
			}
		}

		// Collect work items for the special node.
		for {
			wi, err := collectWorkItem(in, "    ")
			if err != nil {
				return nil, err
			}
			if wi == nil {
				break
			}
			sn.WorkItems = append(sn.WorkItems, *wi)
		}

		result.SpecialNodes = append(result.SpecialNodes, sn)
	}

	return result, nil
}

// generateShortID creates a short ID from the project description.
func generateShortID(description string) string {
	upper := strings.ToUpper(description)
	var letters []byte
	for i := 0; i < len(upper) && len(letters) < 4; i++ {
		if upper[i] >= 'A' && upper[i] <= 'Z' {
			letters = append(letters, upper[i])
		}
	}
	for len(letters) < 3 {
		letters = append(letters, 'X')
	}
	return fmt.Sprintf("%s01", string(letters))
}

// buildSchemaFromWizard creates a valid ImportSchema from wizard-collected data.
func buildSchemaFromWizard(result *wizardResult) *importer.ImportSchema {
	schema := &importer.ImportSchema{
		Project: importer.ProjectImport{
			ShortID:   generateShortID(result.Description),
			Name:      result.Description,
			Domain:    "education",
			StartDate: result.StartDate,
		},
		Defaults: &importer.DefaultsImport{
			DurationMode: "estimate",
			SessionPolicy: &importer.SessionPolicyImport{
				MinSessionMin:     intPtr(15),
				MaxSessionMin:     intPtr(60),
				DefaultSessionMin: intPtr(30),
				Splittable:        boolPtr(true),
			},
		},
	}

	if result.Deadline != "" {
		schema.Project.TargetDate = &result.Deadline
	}

	startDate, _ := time.Parse("2006-01-02", result.StartDate)
	var deadlineDate time.Time
	hasDeadline := false
	if result.Deadline != "" {
		deadlineDate, _ = time.Parse("2006-01-02", result.Deadline)
		hasDeadline = true
	}

	nodeIdx := 0
	wiIdx := 0
	cursor := startDate

	// Calculate total nodes for even spreading.
	totalNodes := 0
	for _, g := range result.Groups {
		totalNodes += g.Count
	}

	for _, group := range result.Groups {
		for i := 0; i < group.Count; i++ {
			nodeIdx++
			nodeRef := fmt.Sprintf("n%d", nodeIdx)

			title := fmt.Sprintf("%s %d", group.Label, i+1)

			node := importer.NodeImport{
				Ref:   nodeRef,
				Title: title,
				Kind:  group.Kind,
				Order: nodeIdx,
			}

			// Calculate due date.
			if group.DaysPer > 0 {
				cursor = cursor.AddDate(0, 0, group.DaysPer)
				dueStr := cursor.Format("2006-01-02")
				node.DueDate = &dueStr
			} else if hasDeadline && totalNodes > 0 {
				// Spread evenly from start to deadline.
				totalDays := deadlineDate.Sub(startDate).Hours() / 24
				dayOffset := int(totalDays * float64(nodeIdx) / float64(totalNodes))
				dueDate := startDate.AddDate(0, 0, dayOffset)
				dueStr := dueDate.Format("2006-01-02")
				node.DueDate = &dueStr
			}

			// Calculate budget from work items.
			budget := 0
			for _, wi := range result.WorkItems {
				budget += wi.PlannedMin
			}
			if budget > 0 {
				node.PlannedMinBudget = &budget
			}

			schema.Nodes = append(schema.Nodes, node)

			// Stamp work items onto this node.
			for _, wiTemplate := range result.WorkItems {
				wiIdx++
				wiRef := fmt.Sprintf("w%d", wiIdx)

				wi := importer.WorkItemImport{
					Ref:     wiRef,
					NodeRef: nodeRef,
					Title:   wiTemplate.Title,
					Type:    wiTemplate.Type,
				}
				if wiTemplate.PlannedMin > 0 {
					pm := wiTemplate.PlannedMin
					wi.PlannedMin = &pm
				}

				schema.WorkItems = append(schema.WorkItems, wi)
			}
		}
	}

	// Append special nodes.
	for _, sn := range result.SpecialNodes {
		nodeIdx++
		nodeRef := fmt.Sprintf("n%d", nodeIdx)

		node := importer.NodeImport{
			Ref:   nodeRef,
			Title: sn.Title,
			Kind:  sn.Kind,
			Order: nodeIdx,
		}

		if sn.DueDate != "" {
			node.DueDate = &sn.DueDate
		} else if hasDeadline {
			node.DueDate = &result.Deadline
		}

		budget := 0
		for _, wi := range sn.WorkItems {
			budget += wi.PlannedMin
		}
		if budget > 0 {
			node.PlannedMinBudget = &budget
		}

		schema.Nodes = append(schema.Nodes, node)

		for _, wiTemplate := range sn.WorkItems {
			wiIdx++
			wiRef := fmt.Sprintf("w%d", wiIdx)

			wi := importer.WorkItemImport{
				Ref:     wiRef,
				NodeRef: nodeRef,
				Title:   wiTemplate.Title,
				Type:    wiTemplate.Type,
			}
			if wiTemplate.PlannedMin > 0 {
				pm := wiTemplate.PlannedMin
				wi.PlannedMin = &pm
			}

			schema.WorkItems = append(schema.WorkItems, wi)
		}
	}

	return schema
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }

// buildLLMDescription converts wizard results into a natural language description
// for seeding the LLM draft conversation.
func buildLLMDescription(wizard *wizardResult) string {
	var b strings.Builder
	b.WriteString(wizard.Description)
	b.WriteString("\nStart date: ")
	b.WriteString(wizard.StartDate)
	if wizard.Deadline != "" {
		b.WriteString("\nDeadline: ")
		b.WriteString(wizard.Deadline)
	}
	b.WriteString("\nStructure: ")
	for i, g := range wizard.Groups {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%d %ss", g.Count, g.Label))
		if g.DaysPer > 0 {
			b.WriteString(fmt.Sprintf(" (%d days each)", g.DaysPer))
		}
	}
	if len(wizard.WorkItems) > 0 {
		b.WriteString(". Each node has: ")
		for i, wi := range wizard.WorkItems {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(wi.Title)
			if wi.PlannedMin > 0 {
				b.WriteString(fmt.Sprintf(" (%dmin)", wi.PlannedMin))
			}
		}
	}
	return b.String()
}

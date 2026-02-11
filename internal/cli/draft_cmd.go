package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/spf13/cobra"
)

func newProjectDraftCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft [description]",
		Short: "Interactively create a project with a guided wizard",
		Long: `Create a project through a step-by-step structure wizard.

Run without arguments to enter interactive mode, or pass a description
to use LLM-powered conversational drafting (requires KAIROS_LLM_ENABLED=true).

Interactive wizard collects:
  1. Project description and dates
  2. Node groups (phases, levels, modules)
  3. Work items per node
  4. Special nodes (exams, milestones)

After the wizard, you can accept the draft or refine it with AI.

Examples:
  project draft
  project draft "A 12-week physics study plan for my final exam in June"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Wrap stdin in a bufio.Reader so readPromptLine can consume
			// \r\n sequences without leaking a stale \n into the next read.
			reader := bufio.NewReader(os.Stdin)

			if len(args) > 0 {
				// Direct description: use LLM conversational flow.
				if app.ProjectDraft == nil {
					return fmt.Errorf("LLM features are disabled. Run without arguments for the guided wizard.\n" +
						"Or use explicit commands:\n" +
						"  project add --name ... --domain ... --start ...\n" +
						"  project import file.json\n\n" +
						"Enable LLM with: KAIROS_LLM_ENABLED=true")
				}
				return runProjectDraftLoop(app, reader, args[0], nil)
			}

			// Interactive wizard flow.
			info, err := gatherProjectInfo(reader)
			if err != nil {
				return err
			}

			wizard, err := runStructureWizard(reader)
			if err != nil {
				return err
			}
			wizard.Description = info.description
			wizard.StartDate = info.startDate
			wizard.Deadline = info.deadline

			schema := buildSchemaFromWizard(wizard)
			return runWizardReview(app, reader, schema, wizard)
		},
	}
	return cmd
}

// projectInfo holds the basic project metadata from the first three questions.
type projectInfo struct {
	description string
	startDate   string
	deadline    string
}

func gatherProjectInfo(in io.Reader) (*projectInfo, error) {
	fmt.Print(formatter.FormatDraftWelcome())

	// Question 1: Project description (required).
	fmt.Print("  Describe your project:\n  > ")
	description, err := readDraftLine(in)
	if err != nil {
		return nil, fmt.Errorf("input cancelled")
	}
	if description == "" {
		return nil, fmt.Errorf("project description is required")
	}

	// Question 2: Start date (optional, defaults to today).
	fmt.Print("\n  When do you want to start? (YYYY-MM-DD, or Enter for today)\n  > ")
	startDate := time.Now().Format("2006-01-02")
	if input, err := readDraftLine(in); err == nil {
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Fprintf(os.Stderr, "  Invalid date format, using today.\n")
			} else {
				startDate = input
			}
		}
	}

	// Question 3: Deadline (optional).
	fmt.Print("\n  When is the deadline? (YYYY-MM-DD, or Enter to skip)\n  > ")
	var deadline string
	if input, err := readDraftLine(in); err == nil {
		if input != "" {
			if _, err := time.Parse("2006-01-02", input); err != nil {
				fmt.Fprintf(os.Stderr, "  Invalid date format, skipping deadline.\n")
			} else {
				deadline = input
			}
		}
	}

	return &projectInfo{
		description: description,
		startDate:   startDate,
		deadline:    deadline,
	}, nil
}

// runWizardReview shows the wizard-generated draft and offers accept/refine/cancel.
func runWizardReview(app *App, in io.Reader, schema *importer.ImportSchema, wizard *wizardResult) error {
	ctx := context.Background()

	// Wrap schema in a DraftConversation for preview formatting.
	conv := &intelligence.DraftConversation{
		Draft:  schema,
		Status: intelligence.DraftStatusReady,
	}

	fmt.Print(formatter.FormatDraftPreview(conv))

	for {
		if app.ProjectDraft != nil {
			fmt.Print("\n[a]ccept  [r]efine with AI  [c]ancel: ")
		} else {
			fmt.Print("\n[a]ccept  [c]ancel: ")
		}

		input, err := readDraftLine(in)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch strings.ToLower(input) {
		case "a", "accept":
			return acceptWizardDraft(app, ctx, schema)
		case "c", "cancel":
			fmt.Println("Draft cancelled.")
			return nil
		case "r", "refine":
			if app.ProjectDraft == nil {
				fmt.Println("LLM features are disabled. Accept the draft or cancel.")
				continue
			}
			// Build a description string for the LLM to have context.
			desc := buildLLMDescription(wizard)
			return runProjectDraftLoop(app, in, desc, schema)
		default:
			fmt.Println("Invalid option.")
		}
	}
}

// buildLLMDescription creates a rich description string from wizard data for LLM context.
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

// runProjectDraftLoop enters the LLM-powered conversational draft loop.
// If preDraft is provided, the conversation is seeded with it.
func runProjectDraftLoop(app *App, in io.Reader, description string, preDraft *importer.ImportSchema) error {
	ctx := context.Background()

	var conv *intelligence.DraftConversation
	var err error

	if preDraft != nil {
		// Seed conversation with the wizard-built draft.
		stopSpinner := formatter.StartSpinner("Preparing for refinement...")
		conv, err = app.ProjectDraft.StartWithDraft(ctx, description, preDraft)
		stopSpinner()
	} else {
		stopSpinner := formatter.StartSpinner("Building your project draft...")
		conv, err = app.ProjectDraft.Start(ctx, description)
		stopSpinner()
	}
	if err != nil {
		return fmt.Errorf("failed to start project draft: %w", err)
	}

	fmt.Print(formatter.FormatDraftTurn(conv))

	for {
		// If status is "ready", show review and prompt for action.
		if conv.Status == intelligence.DraftStatusReady {
			fmt.Print(formatter.FormatDraftReview(conv))
			fmt.Print("\n[a]ccept  [e]dit  [c]ancel: ")

			input, err := readDraftLine(in)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			switch strings.ToLower(input) {
			case "a", "accept":
				return acceptDraft(app, ctx, conv)
			case "c", "cancel":
				fmt.Println("Draft cancelled.")
				return nil
			case "e", "edit":
				conv.Status = intelligence.DraftStatusGathering
				fmt.Print("What would you like to change?\n> ")
				editMsg, err := readDraftLine(in)
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				if editMsg == "" {
					continue
				}
				stopSpinner := formatter.StartSpinner("Thinking...")
				conv, err = app.ProjectDraft.NextTurn(ctx, conv, editMsg)
				stopSpinner()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}
				fmt.Print(formatter.FormatDraftTurn(conv))
				continue
			default:
				// Treat as an edit instruction.
				stopSpinner := formatter.StartSpinner("Thinking...")
				conv, err = app.ProjectDraft.NextTurn(ctx, conv, input)
				stopSpinner()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}
				fmt.Print(formatter.FormatDraftTurn(conv))
				continue
			}
		}

		// Read user input.
		fmt.Print("> ")
		input, err := readDraftLine(in)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if input == "" {
			continue
		}

		// Handle special commands.
		switch strings.ToLower(input) {
		case "/quit", "/cancel", "/q":
			fmt.Println("Draft cancelled.")
			return nil
		case "/show", "/draft":
			fmt.Print(formatter.FormatDraftPreview(conv))
			continue
		case "/accept":
			if conv.Draft != nil {
				return acceptDraft(app, ctx, conv)
			}
			fmt.Println("No draft to accept yet.")
			continue
		}

		// Send to LLM.
		stopSpinner := formatter.StartSpinner("Thinking...")
		conv, err = app.ProjectDraft.NextTurn(ctx, conv, input)
		stopSpinner()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Print(formatter.FormatDraftTurn(conv))
	}
}

func readDraftLine(in io.Reader) (string, error) {
	line, err := readPromptLine(in)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func acceptDraft(app *App, ctx context.Context, conv *intelligence.DraftConversation) error {
	return acceptSchemaImport(app, ctx, conv.Draft)
}

func acceptWizardDraft(app *App, ctx context.Context, schema *importer.ImportSchema) error {
	return acceptSchemaImport(app, ctx, schema)
}

func acceptSchemaImport(app *App, ctx context.Context, schema *importer.ImportSchema) error {
	// Validate the draft.
	errs := importer.ValidateImportSchema(schema)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors.")
		return nil
	}

	// Import via ImportService.
	result, err := app.Import.ImportProjectFromSchema(ctx, schema)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Print(formatter.FormatDraftAccepted(result))
	return nil
}

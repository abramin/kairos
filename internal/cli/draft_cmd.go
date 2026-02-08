package cli

import (
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
		Short: "Interactively create a project from a natural language description",
		Long: `Use LLM to iteratively build a project through conversation.

Run without arguments to enter interactive mode, or pass a description directly.

Commands during conversation:
  /show    Show current draft as a tree
  /accept  Accept the current draft
  /quit    Cancel and exit

Examples:
  kairos project draft
  kairos project draft "A 12-week physics study plan for my final exam in June"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.ProjectDraft == nil {
				return fmt.Errorf("LLM features are disabled. Use explicit commands:\n" +
					"  kairos project add --name ... --domain ... --start ...\n" +
					"  kairos project import file.json\n\n" +
					"Enable with: KAIROS_LLM_ENABLED=true")
			}

			var description string
			if len(args) > 0 {
				description = args[0]
			} else {
				var err error
				description, err = gatherProjectInfo(os.Stdin)
				if err != nil {
					return err
				}
			}

			return runProjectDraftLoop(app, os.Stdin, description)
		},
	}
	return cmd
}

func gatherProjectInfo(in io.Reader) (string, error) {
	fmt.Print(formatter.FormatDraftWelcome())

	// Question 1: Project description (required).
	fmt.Print("  Describe your project:\n  > ")
	description, err := readDraftLine(in)
	if err != nil {
		return "", fmt.Errorf("input cancelled")
	}
	if description == "" {
		return "", fmt.Errorf("project description is required")
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

	// Question 4: Structure hint (optional).
	fmt.Print("\n  How is the work organized? (e.g., \"5 chapters with reading + exercises each\")\n  > ")
	var structure string
	if input, err := readDraftLine(in); err == nil {
		structure = input
	}

	// Bundle into a rich description string.
	var b strings.Builder
	b.WriteString(description)
	b.WriteString("\nStart date: ")
	b.WriteString(startDate)
	if deadline != "" {
		b.WriteString("\nDeadline: ")
		b.WriteString(deadline)
	}
	if structure != "" {
		b.WriteString("\nStructure: ")
		b.WriteString(structure)
	}

	fmt.Printf("\n  %s\n\n", formatter.Dim("Building your project draft..."))

	return b.String(), nil
}

func runProjectDraftLoop(app *App, in io.Reader, description string) error {
	ctx := context.Background()

	// Start the conversation.
	conv, err := app.ProjectDraft.Start(ctx, description)
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
				conv, err = app.ProjectDraft.NextTurn(ctx, conv, editMsg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}
				fmt.Print(formatter.FormatDraftTurn(conv))
				continue
			default:
				// Treat as an edit instruction.
				conv, err = app.ProjectDraft.NextTurn(ctx, conv, input)
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
		conv, err = app.ProjectDraft.NextTurn(ctx, conv, input)
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
	// Validate the draft.
	errs := importer.ValidateImportSchema(conv.Draft)
	if len(errs) > 0 {
		fmt.Print(formatter.FormatDraftValidationErrors(errs))
		fmt.Println("Draft has validation errors. Continue editing to fix them.")
		conv.Status = intelligence.DraftStatusGathering
		return nil
	}

	// Import via ImportService.
	result, err := app.Import.ImportProjectFromSchema(ctx, conv.Draft)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Print(formatter.FormatDraftAccepted(result))
	return nil
}

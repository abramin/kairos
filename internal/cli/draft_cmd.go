package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/importer"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/spf13/cobra"
)

func newProjectDraftCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   `draft "<description>"`,
		Short: "Interactively create a project from a natural language description",
		Long: `Use LLM to iteratively build a project through conversation.

Describe your project, answer questions, review the draft, then accept.

Commands during conversation:
  /show    Show current draft as a tree
  /accept  Accept the current draft
  /quit    Cancel and exit

Example:
  kairos project draft "A 12-week physics study plan for my final exam in June"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.ProjectDraft == nil {
				return fmt.Errorf("LLM features are disabled. Use explicit commands:\n" +
					"  kairos project add --name ... --domain ... --start ...\n" +
					"  kairos project import file.json\n\n" +
					"Enable with: KAIROS_LLM_ENABLED=true")
			}
			return runProjectDraftLoop(app, args[0])
		},
	}
	return cmd
}

func runProjectDraftLoop(app *App, description string) error {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)

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

			if !scanner.Scan() {
				return nil
			}
			input := strings.TrimSpace(scanner.Text())

			switch strings.ToLower(input) {
			case "a", "accept":
				return acceptDraft(app, ctx, conv)
			case "c", "cancel":
				fmt.Println("Draft cancelled.")
				return nil
			case "e", "edit":
				conv.Status = intelligence.DraftStatusGathering
				fmt.Print("What would you like to change?\n> ")
				if !scanner.Scan() {
					return nil
				}
				editMsg := strings.TrimSpace(scanner.Text())
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
		if !scanner.Scan() {
			return nil
		}
		input := strings.TrimSpace(scanner.Text())
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

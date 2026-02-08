package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/spf13/cobra"
)

func newTemplateCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Browse project templates",
	}

	cmd.AddCommand(
		newTemplateListCmd(app),
		newTemplateShowCmd(app),
		newTemplateDraftCmd(app),
	)

	return cmd
}

func newTemplateListCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			templates, err := app.Templates.List(context.Background())
			if err != nil {
				return err
			}

			if len(templates) == 0 {
				fmt.Println("No templates found.")
				return nil
			}

			fmt.Print(formatter.FormatTemplateList(templates))
			return nil
		},
	}
}

func newTemplateShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show REF",
		Short: "Show template details by ID, name, or file stem",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.Templates.Get(context.Background(), args[0])
			if err != nil {
				return err
			}

			fmt.Print(formatter.FormatTemplateShow(t))
			return nil
		},
	}
}

func newTemplateDraftCmd(app *App) *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   `draft "<description>"`,
		Short: "Generate a template from a natural language description",
		Long:  "Use Ollama to draft a project template JSON from a description.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.TemplateDraft == nil {
				return fmt.Errorf("LLM not available for template drafting.\n" +
					"Create templates manually using templates/*.json as reference.\n\n" +
					"Enable with: KAIROS_LLM_ENABLED=true")
			}

			draft, err := app.TemplateDraft.Draft(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("template draft failed: %w", err)
			}

			fmt.Print(formatter.FormatTemplateDraft(draft))

			if !draft.Validation.IsValid {
				fmt.Println("Fix validation errors before saving.")
				return nil
			}

			fmt.Print("Save this template? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(strings.ToLower(text))
			if text != "y" && text != "yes" {
				fmt.Println("Discarded.")
				return nil
			}

			// Determine output path.
			id, _ := draft.TemplateJSON["id"].(string)
			if id == "" {
				id = "draft"
			}
			filename := id + ".json"
			outPath := filepath.Join(outputDir, filename)

			data, err := json.MarshalIndent(draft.TemplateJSON, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling template: %w", err)
			}

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}

			if err := os.WriteFile(outPath, data, 0o644); err != nil {
				return fmt.Errorf("writing template: %w", err)
			}

			fmt.Printf("Template saved to %s\n", outPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "templates", "Directory to save the template")
	return cmd
}

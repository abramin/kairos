package cli

import (
	"context"
	"fmt"

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

			headers := []string{"Name", "Domain", "Version"}
			rows := make([][]string, 0, len(templates))
			for _, t := range templates {
				rows = append(rows, []string{
					t.Name,
					t.Domain,
					t.Version,
				})
			}

			fmt.Print(formatter.RenderTable(headers, rows))
			return nil
		},
	}
}

func newTemplateShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show template details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.Templates.Get(context.Background(), args[0])
			if err != nil {
				return err
			}

			fmt.Println(formatter.Header("Template"))
			fmt.Printf("  Name:    %s\n", formatter.Bold(t.Name))
			fmt.Printf("  Domain:  %s\n", t.Domain)
			fmt.Printf("  Version: %s\n", t.Version)
			fmt.Println()
			fmt.Println(formatter.Header("Configuration"))
			fmt.Println(t.ConfigJSON)

			return nil
		},
	}
}

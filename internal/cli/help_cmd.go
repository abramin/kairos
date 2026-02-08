package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/intelligence"
	"github.com/spf13/cobra"
)

// newHelpCmd creates a custom help command that preserves Cobra's default
// behavior for `help` and `help <command>` while adding `help chat` for
// interactive LLM-powered help.
func newHelpCmd(app *App, root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command, or start an interactive help session",
		Long: `Display help for any command, or use 'help chat' for interactive
LLM-powered help that understands natural language questions.

Examples:
  kairos help
  kairos help project
  kairos help chat
  kairos help chat "How do I log a 30 minute session?"`,
		// DisableFlagsInUseLine prevents --help showing in this command's usage.
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			// Find and show help for the specified subcommand.
			target, _, err := root.Find(args)
			if err != nil || target == nil {
				return fmt.Errorf("unknown command %q — run 'kairos help' for available commands", strings.Join(args, " "))
			}
			return target.Help()
		},
	}

	cmd.AddCommand(newHelpChatCmd(app, root))
	return cmd
}

// newHelpChatCmd creates the `help chat` subcommand for LLM-powered help.
func newHelpChatCmd(app *App, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "chat [question]",
		Short: "Interactive LLM-powered help (or ask a single question)",
		Long: `Start an interactive help session, or pass a question directly
for a one-shot answer.

Commands during chat:
  /quit      Exit the help session
  /commands  List all available commands

Examples:
  kairos help chat
  kairos help chat "How do I log a 30 minute session?"
  kairos help chat "What is a plan node?"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := app.getCommandSpec(root)
			specJSON := SerializeCommandSpec(spec)
			cmdInfos := buildHelpCommandInfos(spec)

			if len(args) == 1 {
				return runHelpOneShot(app, args[0], specJSON, cmdInfos)
			}
			return runHelpChat(app, specJSON, cmdInfos)
		},
	}
}

func runHelpOneShot(app *App, question, specJSON string, cmdInfos []intelligence.HelpCommandInfo) error {
	answer := resolveHelpAnswer(app, question, specJSON, cmdInfos)
	fmt.Print(formatter.FormatHelpAnswer(answer))
	return nil
}

func runHelpChat(app *App, specJSON string, cmdInfos []intelligence.HelpCommandInfo) error {
	fmt.Print(formatter.FormatHelpChatWelcome())

	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	var conv *intelligence.HelpConversation

	for {
		fmt.Print("help> ")
		line, err := readPromptLine(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Handle special commands.
		switch strings.ToLower(input) {
		case "/quit", "/exit", "/q":
			return nil
		case "/commands":
			fmt.Print(formatter.FormatCommandList(cmdInfos))
			continue
		}

		// Use LLM service if available.
		if app.Help != nil {
			var answer *intelligence.HelpAnswer
			var err error

			stopSpinner := formatter.StartSpinner("Thinking...")
			if conv == nil {
				conv, answer, err = app.Help.StartChat(ctx, input, specJSON)
			} else {
				answer, err = app.Help.NextTurn(ctx, conv, input)
			}
			stopSpinner()

			if err != nil {
				// Defensive fallback; HelpService should already handle this.
				answer = intelligence.DeterministicHelp(input, cmdInfos)
			}

			fmt.Print(formatter.FormatHelpAnswer(answer))
		} else {
			// LLM disabled — use deterministic help.
			answer := intelligence.DeterministicHelp(input, cmdInfos)
			fmt.Print(formatter.FormatHelpAnswer(answer))
		}
	}
}

// resolveHelpAnswer gets a help answer using LLM with fallback to deterministic.
func resolveHelpAnswer(app *App, question, specJSON string, cmdInfos []intelligence.HelpCommandInfo) *intelligence.HelpAnswer {
	if app.Help == nil {
		return intelligence.DeterministicHelp(question, cmdInfos)
	}

	ctx := context.Background()
	stopSpinner := formatter.StartSpinner("Thinking...")
	answer, err := app.Help.Ask(ctx, question, specJSON)
	stopSpinner()
	if err != nil {
		return intelligence.DeterministicHelp(question, cmdInfos)
	}

	return answer
}

// buildHelpCommandInfos converts CommandSpec entries into HelpCommandInfo
// for use by the intelligence layer (avoids import cycle).
func buildHelpCommandInfos(spec *CommandSpec) []intelligence.HelpCommandInfo {
	infos := make([]intelligence.HelpCommandInfo, len(spec.Commands))
	for i, cmd := range spec.Commands {
		infos[i] = intelligence.HelpCommandInfo{
			FullPath: cmd.FullPath,
			Short:    cmd.Short,
		}
	}
	return infos
}

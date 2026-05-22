// Package cli implements the `2m chat` interactive REPL command.
//
// Usage: 2m chat <team>
//
// Opens an interactive session where the user can type messages and the
// agent team responds collaboratively. The session persists until the user
// types 'exit', 'quit', or presses Ctrl+C.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/2mcode/2mcode/internal/bridge"
	"github.com/2mcode/2mcode/internal/bus"
	"github.com/2mcode/2mcode/internal/orchestrator"
	"github.com/2mcode/2mcode/internal/team"
)

var chatCmd = &cobra.Command{
	Use:   "chat <team>",
	Short: "Start an interactive REPL with an agent team",
	Long: `Open an interactive chat session with a configured agent team.
Type your messages and the team will collaborate on responses.

Type 'exit' or 'quit' to end the session. Press Ctrl+C to cancel.

Example:
  2m chat fullstack
  2m chat code-review`,
	Args: cobra.ExactArgs(1),
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

// runChat is the handler for `2m chat <team>`.
func runChat(cmd *cobra.Command, args []string) error {
	teamName := args[0]
	renderer := NewRenderer()

	// Print welcome banner
	renderer.PrintWelcome()

	// Load team configuration
	t, err := team.LoadTeam(teamName)
	if err != nil {
		renderer.PrintError(err.Error())
		return err
	}

	// Validate API keys
	missingKeys := team.ValidateProviderKeys(t)
	if len(missingKeys) > 0 {
		for _, provider := range missingKeys {
			renderer.PrintError(fmt.Sprintf("Missing API key for provider '%s'", provider))
		}
		return fmt.Errorf("set missing API keys before chatting")
	}

	// Show team info
	renderer.PrintTeamInfo(t)

	// Create the session database
	sessDir, err := team.SessionsPath(teamName)
	if err != nil {
		return fmt.Errorf("cannot determine sessions path: %w", err)
	}

	sessionID := uuid.New().String()
	dbPath := filepath.Join(sessDir, sessionID+".db")

	db, err := bus.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("cannot initialize session database: %w", err)
	}
	defer db.Close()

	eventBus := bus.New(db)

	// Create the bridge to the Python agent engine
	br := bridge.DefaultBridge()

	// Verify the agent engine is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := br.HealthCheck(ctx); err != nil {
		renderer.PrintError("Agent engine is not running. Start it or run via the main 2m binary.")
		return err
	}

	// Create the orchestrator
	orch := orchestrator.New(eventBus, br, renderer)

	// Create the session
	if err := eventBus.CreateSession(sessionID, teamName); err != nil {
		return fmt.Errorf("cannot create session: %w", err)
	}

	// Interactive REPL
	scanner := bufio.NewScanner(os.Stdin)
	renderer.PrintInfo("Chat started. Type 'exit' or 'quit' to end.\n")

	for {
		// Print prompt
		fmt.Print("you > ")

		if !scanner.Scan() {
			break // EOF or Ctrl+C
		}

		input := strings.TrimSpace(scanner.Text())

		// Check for exit commands
		switch strings.ToLower(input) {
		case "exit", "quit", "/exit", "/quit":
			renderer.PrintInfo("Session ended. Goodbye!")
			return nil
		case "":
			continue // Skip empty input
		case "/help":
			printChatHelp(renderer)
			continue
		case "/info":
			renderer.PrintTeamInfo(t)
			continue
		}

		// Run the agent team on this message
		ctx := context.Background()
		if err := orch.RunChatTurn(ctx, t, sessionID, input); err != nil {
			renderer.PrintError(fmt.Sprintf("Chat turn failed: %s", err))
			// Don't exit on error — let the user try again
		}

		fmt.Println() // Blank line between turns
	}

	return nil
}

// printChatHelp shows available REPL commands.
func printChatHelp(renderer *TerminalRenderer) {
	renderer.PrintInfo("Available commands:")
	renderer.PrintInfo("  /help  — Show this help")
	renderer.PrintInfo("  /info  — Show team configuration")
	renderer.PrintInfo("  /exit  — End the session")
	renderer.PrintInfo("  /quit  — End the session")
	fmt.Println()
}

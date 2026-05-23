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
	"github.com/2mcode/2mcode/internal/memory"
	"github.com/2mcode/2mcode/internal/orchestrator"
	"github.com/2mcode/2mcode/internal/team"
)

// chatState holds the runtime state for the chat REPL.
type chatState struct {
	orch      *orchestrator.Orchestrator
	t         *team.Team
	sessionID string
	eventBus  *bus.Bus
	renderer  *TerminalRenderer
}

var chatCmd = &cobra.Command{
	Use:   "chat <team>",
	Short: "Start an interactive REPL with an agent team",
	Long: `Open an interactive chat session with a configured agent team.
Type your messages and the team will collaborate on responses.

Type 'exit' or 'quit' to end the session. Press Ctrl+C to cancel.

Example:
  2m chat fullstack
  2m chat code-review`,
	Args: cobra.MinimumNArgs(1),
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

// runChat is the handler for `2m chat <team>`.
// The team name may contain spaces (e.g. '2m code test team') so all
// positional args are joined before lookup.
func runChat(cmd *cobra.Command, args []string) error {
	teamName := strings.Join(args, " ")

	renderer := NewRenderer()

	// Print welcome banner
	renderer.PrintWelcome()

	// Load team configuration
	t, err := team.LoadTeam(teamName)
	if err != nil {
		renderer.PrintError(err.Error())
		return err
	}

	// Check API keys — warn but don't block
	for _, provider := range team.ValidateProviderKeys(t) {
		renderer.PrintInfo(fmt.Sprintf("Note: %s API key not set — will try other providers if available", provider))
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

	// Attach persistent memory if available
	if memDir, err := memoryDir(); err == nil {
		if memStore, err := memory.NewFileStore(memDir); err == nil {
			orch.WithMemory(memory.NewSummarizer(br, memStore))
		}
	}

	// Create the session
	if err := eventBus.CreateSession(sessionID, teamName); err != nil {
		return fmt.Errorf("cannot create session: %w", err)
	}

	state := &chatState{
		orch:      orch,
		t:         t,
		sessionID: sessionID,
		eventBus:  eventBus,
		renderer:  renderer,
	}

	// Interactive REPL
	scanner := bufio.NewScanner(os.Stdin)
	renderer.PrintInfo("Chat started. Type /help for commands.\n")

	for {
		fmt.Print("you > ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if state.handleCommand(input) {
				return nil // exit signal
			}
			continue
		}

		// Handle exit aliases
		switch strings.ToLower(input) {
		case "exit", "quit":
			renderer.PrintInfo("Session ended. Goodbye!")
			return nil
		}

		// Run the agent team on this message
		ctx := context.Background()
		if err := orch.RunChatTurn(ctx, t, sessionID, input); err != nil {
			renderer.PrintError(fmt.Sprintf("Chat turn failed: %s", err))
		}

		fmt.Println()
	}

	return nil
}

// handleCommand processes a slash command. Returns true if the REPL should exit.
func (s *chatState) handleCommand(cmd string) bool {
	parts := strings.Fields(strings.ToLower(cmd))
	if len(parts) == 0 {
		return false
	}

	switch parts[0] {
	case "/help":
		printChatHelp(s.renderer)

	case "/info":
		s.renderer.PrintTeamInfo(s.t)

	case "/exit", "/quit":
		s.renderer.PrintInfo("Session ended. Goodbye!")
		return true
	}

	return false
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

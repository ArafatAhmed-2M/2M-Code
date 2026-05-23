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

	// Attach persistent memory if available
	if memDir, err := memoryDir(); err == nil {
		if memStore, err := memory.NewFileStore(memDir); err == nil {
			memSummarizer := memory.NewSummarizer(br, memStore)
			if memSummarizer.Enabled() {
				orch.WithMemory(memSummarizer)
			}
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

	case "/session":
		s.showSessionInfo()

	case "/clear":
		fmt.Print("\033[2J\033[H") // ANSI clear screen

	case "/models":
		s.renderer.PrintInfo("Run '2m models' in your terminal to list available models.")

	case "/export":
		s.exportSession()

	case "/compact":
		s.compactSession()

	case "/new", "/clear-session":
		s.renderer.PrintInfo("Start a new session with: 2m chat " + s.t.Name)

	case "/exit", "/quit":
		s.renderer.PrintInfo("Session ended. Goodbye!")
		return true
	}

	return false
}

// showSessionInfo prints the current session details.
func (s *chatState) showSessionInfo() {
	count, err := s.eventBus.MessageCount(s.sessionID)
	if err != nil {
		s.renderer.PrintError(fmt.Sprintf("Cannot get message count: %s", err))
		return
	}

	s.renderer.PrintInfo(fmt.Sprintf("Team: %s", s.t.Name))
	s.renderer.PrintInfo(fmt.Sprintf("Session: %s", s.sessionID[:8]))
	s.renderer.PrintInfo(fmt.Sprintf("Messages: %d", count))
	s.renderer.PrintInfo(fmt.Sprintf("Agents: %d", len(s.t.Agents)))
	fmt.Println()
}

// exportSession writes the current session transcript to a markdown file.
func (s *chatState) exportSession() {
	messages, err := s.eventBus.GetAllMessages(s.sessionID)
	if err != nil {
		s.renderer.PrintError(fmt.Sprintf("Cannot read session: %s", err))
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 2M Code Session: %s\n\n", s.t.Name))
	b.WriteString(fmt.Sprintf("**Date:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("---\n\n")

	for _, msg := range messages {
		speaker := msg.AgentName
		if speaker == "" {
			speaker = msg.Role
		}
		b.WriteString(fmt.Sprintf("### %s\n\n", speaker))
		b.WriteString(fmt.Sprintf("%s\n\n", msg.Content))
	}

	filename := fmt.Sprintf("2m-session-%s.md", time.Now().Format("20060102-150405"))
	if err := os.WriteFile(filename, []byte(b.String()), 0644); err != nil {
		s.renderer.PrintError(fmt.Sprintf("Cannot write export: %s", err))
		return
	}

	s.renderer.PrintInfo(fmt.Sprintf("Exported to: %s (%d messages)", filename, len(messages)))
}

// compactSession triggers a memory summarization of the current session.
func (s *chatState) compactSession() {
	messages, err := s.eventBus.GetAllMessages(s.sessionID)
	if err != nil {
		s.renderer.PrintError(fmt.Sprintf("Cannot read session: %s", err))
		return
	}

	var b strings.Builder
	for _, msg := range messages {
		speaker := msg.AgentName
		if speaker == "" {
			speaker = msg.Role
		}
		b.WriteString(fmt.Sprintf("[%s]: %s\n", speaker, msg.Content))
	}

	s.renderer.PrintInfo(fmt.Sprintf("Session has %d messages. Summarizing...", len(messages)))
	ctx := context.Background()
	s.orch.SaveMemory(ctx, s.t, s.sessionID, "Chat session", b.String())
}

// printChatHelp shows available REPL commands.
func printChatHelp(renderer *TerminalRenderer) {
	renderer.PrintInfo("Available commands:")
	renderer.PrintInfo("  /help       — Show this help")
	renderer.PrintInfo("  /info       — Show team configuration")
	renderer.PrintInfo("  /session    — Show current session info (ID, message count)")
	renderer.PrintInfo("  /export     — Export session transcript to markdown file")
	renderer.PrintInfo("  /compact    — Summarize and save session to memory now")
	renderer.PrintInfo("  /clear      — Clear the terminal screen")
	renderer.PrintInfo("  /new        — Start a fresh session")
	renderer.PrintInfo("  /exit, quit — End the session")
	fmt.Println()
	renderer.PrintInfo("Type any message to chat with your agent team.")
}

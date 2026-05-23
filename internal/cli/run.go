// Package cli implements the `2m run` command for one-shot task execution.
//
// Usage: 2m run <team> "<task>"
//
// This command loads a team configuration, validates API keys, creates a
// session, and runs the orchestrator to completion.
package cli

import (
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

// memoryDir returns the path to the persistent memory directory.
func memoryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".2mcode", "memory"), nil
}

var runCmd = &cobra.Command{
	Use:   "run <team> \"<task>\"",
	Short: "Run a one-shot task with an agent team",
	Long: `Execute a task with a configured agent team. The team's agents will
collaborate — planning, implementing, and reviewing — then exit.

Quote the team name and/or the task when they contain spaces:
  2m run "full-stack" "Build a REST API for user authentication"
  2m run "code-review" "Review the auth middleware"

Example:
  2m run fullstack "Build a REST API for user authentication with JWT"
  2m run code-review "Review the auth middleware in internal/auth/"
  2m run data-science "Analyze the sales CSV and suggest ML models"`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTask,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

// runTask is the handler for `2m run <team> "<task>"`.
// The team name may contain spaces so all positional args are joined and then
// split by trying progressively shorter prefixes as team name candidates.
func runTask(cmd *cobra.Command, args []string) error {
	// Try every possible split, consuming fewer words from the right until a
	// matching team file is found (first attempt = "everything but last arg is
	// team, last arg is task").
	var teamName, task string
	found := false
	for i := len(args) - 1; i >= 1; i-- {
		teamName = strings.Join(args[:i], " ")
		task = strings.Join(args[i:], " ")
		if _, err := team.LoadTeam(teamName); err == nil {
			found = true
			break
		}
	}
	if !found {
		// Nothing matched — treat the last token as the team name.
		teamName = args[len(args)-1]
		task = strings.Join(args[:len(args)-1], " ")
	}
	renderer := NewRenderer()

	// Load team configuration
	t, err := team.LoadTeam(teamName)
	if err != nil {
		renderer.PrintError(err.Error())
		return err
	}

	// Validate API keys for all providers used in this team
	missingKeys := team.ValidateProviderKeys(t)
	if len(missingKeys) > 0 {
		for _, provider := range missingKeys {
			renderer.PrintError(fmt.Sprintf("Missing API key for provider '%s'", provider))
		}
		return fmt.Errorf("set missing API keys before running — see 2m config help")
	}

	// Show team info
	renderer.PrintTeamInfo(t)
	renderer.PrintInfo(fmt.Sprintf("Task: %s", task))
	fmt.Println()

	// Create the session database
	sessDir, err := team.SessionsPath(teamName)
	if err != nil {
		return fmt.Errorf("cannot determine sessions path: %w", err)
	}

	sessionID := generateSessionID()
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

	// Create the orchestrator and run the task
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

	ctx = context.Background()
	return orch.RunTask(ctx, t, sessionID, task)
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	return uuid.New().String()
}

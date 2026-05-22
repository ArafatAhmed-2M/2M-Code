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
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/2mcode/2mcode/internal/bridge"
	"github.com/2mcode/2mcode/internal/bus"
	"github.com/2mcode/2mcode/internal/orchestrator"
	"github.com/2mcode/2mcode/internal/team"
)

var runCmd = &cobra.Command{
	Use:   "run <team> \"<task>\"",
	Short: "Run a one-shot task with an agent team",
	Long: `Execute a task with a configured agent team. The team's agents will
collaborate — planning, implementing, and reviewing — then exit.

Example:
  2m run fullstack "Build a REST API for user authentication with JWT"
  2m run code-review "Review the auth middleware in internal/auth/"
  2m run data-science "Analyze the sales CSV and suggest ML models"`,
	Args: cobra.ExactArgs(2),
	RunE: runTask,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

// runTask is the handler for `2m run <team> "<task>"`.
func runTask(cmd *cobra.Command, args []string) error {
	teamName := args[0]
	task := args[1]

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

	ctx = context.Background()
	return orch.RunTask(ctx, t, sessionID, task)
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	return uuid.New().String()
}

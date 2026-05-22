// Package cli implements the `2m team` subcommands for managing team configs.
//
// Subcommands:
//   2m team list           — List all available teams
//   2m team show <name>    — Show team configuration details
//   2m history <team>      — Show last session's team channel log
//   2m config set <key>    — Set global config values
package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/2mcode/2mcode/internal/team"
)

// teamCmd is the parent command for team management.
var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage agent team configurations",
	Long: `View and manage your agent team configurations.

Subcommands:
  list  — List all available teams
  show  — Show detailed configuration for a specific team`,
}

// teamListCmd lists all available teams.
var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available teams",
	Long:  "Show all team configurations found in project-local, global, and bundled locations.",
	RunE:  listTeams,
}

// teamShowCmd shows details for a specific team.
var teamShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show team configuration details",
	Long:  "Display the full configuration for a specific team, including all agents and workflow settings.",
	Args:  cobra.ExactArgs(1),
	RunE:  showTeam,
}

// historyCmd shows the last session's log.
var historyCmd = &cobra.Command{
	Use:   "history <team>",
	Short: "Show last session's team channel log",
	Long:  "Display the conversation history from the most recent session with a team.",
	Args:  cobra.ExactArgs(1),
	RunE:  showHistory,
}

// configCmd manages global configuration.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global 2M Code configuration",
}

// configSetCmd sets a global config value.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a global configuration value",
	Long: `Set a global configuration value.

Available keys:
  default_team     — Team to use when none is specified
  default_provider — Provider to use when creating new teams
  verbose          — Enable verbose output (true/false)`,
	Args: cobra.ExactArgs(2),
	RunE: setConfig,
}

func init() {
	// Register subcommands
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamShowCmd)
	rootCmd.AddCommand(teamCmd)
	rootCmd.AddCommand(historyCmd)

	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

// listTeams handles `2m team list`.
func listTeams(cmd *cobra.Command, args []string) error {
	teams, err := team.ListTeams()
	if err != nil {
		return fmt.Errorf("cannot list teams: %w", err)
	}

	if len(teams) == 0 {
		fmt.Println("No teams found. Run '2m new-team' to create one.")
		return nil
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	fmt.Println(headerStyle.Render("Available teams:"))
	fmt.Println()

	for name, path := range teams {
		// Load the team to show description
		t, err := team.LoadTeam(name)
		desc := ""
		if err == nil && t.Description != "" {
			desc = " — " + t.Description
		}

		fmt.Printf("  %s%s\n", headerStyle.Render(name), desc)
		fmt.Printf("  %s\n\n", pathStyle.Render(path))
	}

	return nil
}

// showTeam handles `2m team show <name>`.
func showTeam(cmd *cobra.Command, args []string) error {
	teamName := args[0]
	renderer := NewRenderer()

	t, err := team.LoadTeam(teamName)
	if err != nil {
		renderer.PrintError(err.Error())
		return err
	}

	renderer.PrintTeamInfo(t)

	// Show agent details
	for _, agent := range t.Agents {
		detailStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

		fmt.Printf("  %s\n", detailStyle.Render("System prompt (first 200 chars):"))
		prompt := agent.SystemPrompt
		if len(prompt) > 200 {
			prompt = prompt[:200] + "..."
		}
		fmt.Printf("  %s\n\n", detailStyle.Render(prompt))
	}

	// Show API key status
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).Render("API Key Status:"))
	seen := make(map[string]bool)
	for _, agent := range t.Agents {
		if seen[agent.Provider] {
			continue
		}
		seen[agent.Provider] = true

		_, err := team.GetProviderAPIKey(agent.Provider)
		if err != nil {
			fmt.Printf("  %s: %s\n", agent.Provider, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗ not set"))
		} else {
			fmt.Printf("  %s: %s\n", agent.Provider, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ configured"))
		}
	}
	fmt.Println()

	return nil
}

// showHistory handles `2m history <team>`.
func showHistory(cmd *cobra.Command, args []string) error {
	teamName := args[0]
	renderer := NewRenderer()

	renderer.PrintInfo(fmt.Sprintf("History for team '%s':", teamName))
	renderer.PrintInfo("(Session history display — coming in next iteration)")

	// TODO: Implement full history display by loading the latest session DB
	// and printing all messages with agent badges

	return nil
}

// setConfig handles `2m config set <key> <value>`.
func setConfig(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfg, err := team.LoadConfig()
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}

	switch key {
	case "default_team":
		cfg.DefaultTeam = value
	case "default_provider":
		cfg.DefaultProvider = value
	case "verbose":
		cfg.Verbose = value == "true"
	default:
		return fmt.Errorf("unknown config key '%s' — use: default_team, default_provider, verbose", key)
	}

	if err := team.SaveConfig(cfg); err != nil {
		return fmt.Errorf("cannot save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

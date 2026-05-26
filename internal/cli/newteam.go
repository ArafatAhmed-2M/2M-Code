// Package cli implements the `2m new-team` interactive wizard.
//
// The wizard walks the user through creating a team configuration YAML by
// prompting for team name, agents (name, role, provider, model), and
// workflow settings. The resulting YAML is saved to the global teams directory.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/2mcode/2mcode/internal/team"
)

var newTeamCmd = &cobra.Command{
	Use:   "new-team",
	Short: "Create a new agent team interactively",
	Long: `Launch an interactive wizard to create a new team configuration.

The wizard will guide you through:
  1. Team name and description
  2. Adding agents (name, role, provider, model)
  3. Workflow configuration (orchestration mode, turns)

The resulting YAML will be saved to ~/.2mcode/teams/<name>.yaml`,
	RunE: runNewTeam,
}

func init() {
	rootCmd.AddCommand(newTeamCmd)
}

// runNewTeam is the handler for `2m new-team`.
func runNewTeam(cmd *cobra.Command, args []string) error {
	renderer := NewRenderer()
	scanner := bufio.NewScanner(os.Stdin)

	renderer.PrintWelcome()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))

	fmt.Println(headerStyle.Render("🧙 Team Creation Wizard"))
	fmt.Println()

	// 1. Team basics
	fmt.Println(headerStyle.Render("Step 1: Team Basics"))
	teamName := prompt(scanner, promptStyle.Render("Team name: "))
	if teamName == "" {
		return fmt.Errorf("team name is required")
	}

	teamDesc := prompt(scanner, promptStyle.Render("Description (optional): "))

	// 2. Agents
	fmt.Println()
	fmt.Println(headerStyle.Render("Step 2: Add Agents"))
	fmt.Println("  Add at least 2 agents. Type 'done' when finished.")
	fmt.Println()

	var agents []team.Agent
	colors := []string{"cyan", "green", "yellow", "magenta", "blue", "red"}
	agentNum := 0

	for {
		agentNum++
		fmt.Printf("  %s\n", headerStyle.Render(fmt.Sprintf("Agent %d:", agentNum)))

		name := prompt(scanner, promptStyle.Render("  Name (or 'done'): "))
		if strings.ToLower(name) == "done" {
			if len(agents) < 1 {
				fmt.Println("  Need at least 1 agent. Try again.")
				agentNum--
				continue
			}
			break
		}

		role := prompt(scanner, promptStyle.Render("  Role (e.g., Tech Lead): "))
		provider := promptWithOptions(scanner, promptStyle.Render("  Provider"), []string{"anthropic", "google", "openai", "openai_compatible", "mistral", "cohere", "groq", "ollama", "openrouter"})
		if provider == "openai_compatible" {
			fmt.Println("    Note: openai_compatible covers DeepSeek, Together, xAI, Perplexity, Fireworks, GitHub Models, and any OpenAI-compatible API.")
		}
		model := prompt(scanner, promptStyle.Render("  Model (e.g., claude-opus-4-5): "))

		// Default system prompt
		defaultPrompt := fmt.Sprintf(
			"You are %s, the %s on this team. You collaborate with your teammates through a shared conversation channel. "+
				"Every message you see is from a team member. Communicate clearly, focus on your role, and build on what others have said. "+
				"When you disagree, explain your reasoning. When you agree, add value rather than repeating.",
			name, role,
		)

		systemPrompt := prompt(scanner, promptStyle.Render("  System prompt (press Enter for default): "))
		if systemPrompt == "" {
			systemPrompt = defaultPrompt
		}

		// Base URL (only for openai_compatible)
		var baseURL string
		if provider == "openai_compatible" {
			baseURL = prompt(scanner, promptStyle.Render("  Base URL (e.g. https://api.deepseek.com; Enter for env var): "))
		}

		// Tools
		fmt.Println("  Available tools: bash, read_file, write_file, web_fetch")
		toolsStr := prompt(scanner, promptStyle.Render("  Tools (comma-separated, or Enter for all): "))
		var tools []string
		if toolsStr == "" {
			tools = []string{"bash", "read_file", "write_file"}
		} else {
			for _, t := range strings.Split(toolsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tools = append(tools, t)
				}
			}
		}

		agents = append(agents, team.Agent{
			Name:         name,
			Role:         role,
			Provider:     provider,
			Model:        model,
			SystemPrompt: systemPrompt,
			MaxContext:   20,
			Color:        colors[(agentNum-1)%len(colors)],
			Tools:        tools,
			BaseURL:      baseURL,
		})

		fmt.Printf("  ✓ Added %s (%s)\n\n", name, role)
	}

	// 3. Workflow
	fmt.Println()
	fmt.Println(headerStyle.Render("Step 3: Workflow Configuration"))

	orchestration := promptWithOptions(scanner, promptStyle.Render("  Orchestration mode"), []string{"leader_first", "round_robin"})

	var leader, reviewer string
	if orchestration == "leader_first" {
		// Show agent names for selection
		agentNames := make([]string, len(agents))
		for i, a := range agents {
			agentNames[i] = a.Name
		}
		leader = promptWithOptions(scanner, promptStyle.Render("  Leader agent"), agentNames)

		reviewOptions := append([]string{"(skip)"}, agentNames...)
		reviewerChoice := promptWithOptions(scanner, promptStyle.Render("  Reviewer agent"), reviewOptions)
		if reviewerChoice != "(skip)" {
			reviewer = reviewerChoice
		}
	}

	turnsStr := prompt(scanner, promptStyle.Render("  Turns per task (default: 1): "))
	turns := 1
	if turnsStr != "" {
		if parsed, err := strconv.Atoi(turnsStr); err == nil && parsed > 0 {
			turns = parsed
		}
	}

	budgetStr := prompt(scanner, promptStyle.Render("  Max tokens per run (0 = unlimited, default: 0): "))
	budget := 0
	if budgetStr != "" {
		if parsed, err := strconv.Atoi(budgetStr); err == nil && parsed >= 0 {
			budget = parsed
		}
	}

	// Build the team
	t := &team.Team{
		Name:        teamName,
		Description: teamDesc,
		Version:     "1.0",
		Agents:      agents,
		Workflow: team.Workflow{
			Orchestration:   orchestration,
			TurnsPerTask:    turns,
			Leader:          leader,
			Reviewer:        reviewer,
			MaxTokens:       4096,
			MaxTokensPerRun: budget,
		},
	}

	// Validate
	if err := t.Validate(); err != nil {
		renderer.PrintError(fmt.Sprintf("Team validation failed: %s", err))
		return err
	}

	// Save
	teamsDir, err := team.TeamsDir()
	if err != nil {
		return fmt.Errorf("cannot determine teams directory: %w", err)
	}

	savePath := filepath.Join(teamsDir, teamName+".yaml")
	if err := t.SaveToFile(savePath); err != nil {
		return fmt.Errorf("cannot save team: %w", err)
	}

	fmt.Println()
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10"))

	fmt.Println(successStyle.Render("✓ Team created successfully!"))
	fmt.Printf("  Saved to: %s\n\n", savePath)
	fmt.Println("  Next steps:")
	fmt.Printf("  1. Review: 2m team show %s\n", teamName)
	fmt.Printf("  2. Run:    2m run %s \"<your task>\"\n", teamName)
	fmt.Printf("  3. Chat:   2m chat %s\n", teamName)
	fmt.Println()

	return nil
}

// prompt displays a message and reads a line of input.
func prompt(scanner *bufio.Scanner, message string) string {
	fmt.Print(message)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// promptWithOptions shows numbered options and returns the selected value.
func promptWithOptions(scanner *bufio.Scanner, message string, options []string) string {
	fmt.Printf("%s:\n", message)
	for i, opt := range options {
		fmt.Printf("    %d. %s\n", i+1, opt)
	}
	fmt.Print("  Choice (number or name): ")

	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		// Try as number
		if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(options) {
			return options[num-1]
		}

		// Try as name match
		for _, opt := range options {
			if strings.EqualFold(input, opt) {
				return opt
			}
		}

		// Default to first option if empty
		if input == "" && len(options) > 0 {
			return options[0]
		}

		return input
	}
	return options[0]
}

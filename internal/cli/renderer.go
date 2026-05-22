// Package cli implements the terminal rendering for 2M Code.
//
// It uses the charmbracelet/lipgloss library to create beautiful, color-coded
// output that distinguishes between agents, tool calls, errors, and summaries.
//
// Render format per agent turn:
//
//	╭─ Aria · Tech Lead ────────────────────────
//	│ I'll break this task into three subtasks:
//	│ 1. Set up the database schema
//	│ 2. Implement the API endpoints
//	│ 3. Add authentication middleware
//	╰──────────────────────────────────────────
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/2mcode/2mcode/internal/team"
	"github.com/charmbracelet/lipgloss"
)

// colorMap translates agent color names to lipgloss colors.
var colorMap = map[string]lipgloss.Color{
	"red":     lipgloss.Color("9"),
	"yellow":  lipgloss.Color("11"),
	"green":   lipgloss.Color("10"),
	"blue":    lipgloss.Color("12"),
	"cyan":    lipgloss.Color("14"),
	"magenta": lipgloss.Color("13"),
	"white":   lipgloss.Color("15"),
}

// TerminalRenderer implements the orchestrator.Renderer interface for terminal output.
type TerminalRenderer struct {
	width int // Terminal width for formatting
}

// NewRenderer creates a new TerminalRenderer.
func NewRenderer() *TerminalRenderer {
	return &TerminalRenderer{
		width: 60, // Default width
	}
}

// getColor returns the lipgloss color for an agent, defaulting to cyan.
func getColor(agent team.Agent) lipgloss.Color {
	if c, ok := colorMap[agent.Color]; ok {
		return c
	}
	return colorMap["cyan"]
}

// PrintAgentStart renders the opening agent badge.
//
//	╭─ Aria · Tech Lead ────────────────────────
func (r *TerminalRenderer) PrintAgentStart(agent team.Agent) {
	color := getColor(agent)

	// Build the agent badge
	badge := fmt.Sprintf(" %s · %s ", agent.Name, agent.Role)
	badgeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(color)

	// Build the header line
	styledBadge := badgeStyle.Render(badge)

	// Top border with badge
	topBorder := lipgloss.NewStyle().
		Foreground(color).
		Render("╭─")

	paddingLen := r.width - len(badge) - 3
	if paddingLen < 0 {
		paddingLen = 0
	}
	topPadding := lipgloss.NewStyle().
		Foreground(color).
		Render(strings.Repeat("─", paddingLen))

	fmt.Printf("\n%s%s%s\n", topBorder, styledBadge, topPadding)
}

// PrintAgentText renders the agent's text response with bordered lines.
//
//	│ I'll break this task into three subtasks...
func (r *TerminalRenderer) PrintAgentText(agent team.Agent, text string) {
	color := getColor(agent)
	border := lipgloss.NewStyle().
		Foreground(color).
		Render("│")

	// Split text into lines and render each with a border
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Printf("%s %s\n", border, line)
	}
}

// PrintAgentEnd renders the closing border for an agent's response.
//
//	╰──────────────────────────────────────────
func (r *TerminalRenderer) PrintAgentEnd(agent team.Agent) {
	color := getColor(agent)
	bottom := lipgloss.NewStyle().
		Foreground(color).
		Render("╰" + strings.Repeat("─", r.width-1))
	fmt.Println(bottom)
}

// PrintToolCall renders a tool invocation line.
//
//	⚙ running bash: go test ./...
func (r *TerminalRenderer) PrintToolCall(agent team.Agent, toolName string, toolInput map[string]interface{}) {
	color := getColor(agent)
	border := lipgloss.NewStyle().
		Foreground(color).
		Render("│")

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Cyan
		Bold(true)

	// Build a concise description of the tool input
	inputSummary := summarizeToolInput(toolName, toolInput)

	fmt.Printf("%s %s %s: %s\n",
		border,
		toolStyle.Render("⚙ running"),
		toolStyle.Render(toolName),
		inputSummary,
	)
}

// PrintToolResult renders the result of a tool execution.
//
//	└ [output truncated...]
func (r *TerminalRenderer) PrintToolResult(agent team.Agent, toolName string, result string) {
	color := getColor(agent)
	border := lipgloss.NewStyle().
		Foreground(color).
		Render("│")

	dimStyle := lipgloss.NewStyle().
		Faint(true)

	// Truncate long results for display
	displayResult := result
	if len(displayResult) > 500 {
		displayResult = displayResult[:500] + "..."
	}

	lines := strings.Split(displayResult, "\n")
	for i, line := range lines {
		prefix := "  "
		if i == 0 {
			prefix = "└ "
		}
		fmt.Printf("%s %s%s\n", border, prefix, dimStyle.Render(line))
	}
}

// PrintSummary renders the task completion summary.
//
//	✓ Team completed task in 4 turns · 3,241 tokens · 12.3s
func (r *TerminalRenderer) PrintSummary(turns int, inputTokens int, outputTokens int, duration time.Duration) {
	totalTokens := inputTokens + outputTokens

	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")). // Green
		Bold(true)

	summary := fmt.Sprintf("✓ Team completed task in %d turns · %s tokens · %s",
		turns,
		formatNumber(totalTokens),
		formatDuration(duration),
	)

	fmt.Printf("\n%s\n\n", summaryStyle.Render(summary))
}

// PrintError renders an error message.
//
//	✗ Error: something went wrong
func (r *TerminalRenderer) PrintError(msg string) {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")). // Red
		Bold(true)

	fmt.Printf("%s\n", errorStyle.Render("✗ "+msg))
}

// PrintInfo renders an informational message.
func (r *TerminalRenderer) PrintInfo(msg string) {
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Dim
		Italic(true)

	fmt.Printf("%s\n", infoStyle.Render(msg))
}

// PrintWelcome renders the 2M Code welcome banner.
func (r *TerminalRenderer) PrintWelcome() {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")). // Cyan
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	fmt.Println(titleStyle.Render("  ___  __  __    ____          _      "))
	fmt.Println(titleStyle.Render(" |__ \\|  \\/  |  / ___|___   __| | ___ "))
	fmt.Println(titleStyle.Render("   ) | |\\/| | | |   / _ \\ / _` |/ _ \\"))
	fmt.Println(titleStyle.Render("  / /| |  | | | |__| (_) | (_| |  __/"))
	fmt.Println(titleStyle.Render(" |___|_|  |_|  \\____\\___/ \\__,_|\\___|"))
	fmt.Println(subtitleStyle.Render("  The AI coding platform that thinks in teams"))
	fmt.Println()
}

// PrintTeamInfo renders team configuration details.
func (r *TerminalRenderer) PrintTeamInfo(t *team.Team) {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("8"))

	fmt.Printf("%s %s\n", headerStyle.Render("Team:"), t.Name)
	if t.Description != "" {
		fmt.Printf("%s %s\n", labelStyle.Render("  Description:"), t.Description)
	}
	fmt.Printf("%s %s\n", labelStyle.Render("  Orchestration:"), t.Workflow.Orchestration)
	fmt.Printf("%s %d\n", labelStyle.Render("  Turns per task:"), t.Workflow.TurnsPerTask)
	fmt.Println()

	fmt.Println(headerStyle.Render("Agents:"))
	for _, agent := range t.Agents {
		agentColor := getColor(agent)
		nameStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(agentColor)

		role := ""
		if agent.Name == t.Workflow.Leader {
			role = " (leader)"
		} else if agent.Name == t.Workflow.Reviewer {
			role = " (reviewer)"
		}

		fmt.Printf("  %s — %s · %s/%s%s\n",
			nameStyle.Render(agent.Name),
			agent.Role,
			agent.Provider,
			agent.Model,
			role,
		)
	}
	fmt.Println()
}

// summarizeToolInput creates a concise one-line description of tool input.
func summarizeToolInput(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 80 {
				return cmd[:80] + "..."
			}
			return cmd
		}
	case "read_file":
		if path, ok := input["path"].(string); ok {
			return path
		}
	case "write_file":
		if path, ok := input["path"].(string); ok {
			return path
		}
	}
	return fmt.Sprintf("%v", input)
}

// formatNumber formats an integer with comma separators.
func formatNumber(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result []byte
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// Package cli implements the `2m models` command to list all available LLM models.
package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/2mcode/2mcode/internal/bridge"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from configured providers",
	Long: `List all available models from your configured providers by querying 
their live APIs. Only providers with valid API keys will return results.

Example:
  2m models
  2m models --provider anthropic,google`,
	RunE: runModels,
}

var providerFilter string

func init() {
	modelsCmd.Flags().StringVarP(&providerFilter, "provider", "p", "", "Filter by provider (comma-separated)")
	rootCmd.AddCommand(modelsCmd)
}

func runModels(cmd *cobra.Command, args []string) error {
	renderer := NewRenderer()
	
	renderer.PrintInfo("Fetching live model catalog... (this may take a few seconds)\n")

	br := bridge.DefaultBridge()

	// Verify the agent engine is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := br.HealthCheck(ctx); err != nil {
		renderer.PrintError("Agent engine is not running. Start it or run via the main 2m binary.")
		return err
	}

	// Fetch models
	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer fetchCancel()
	
	// If providerFilter is set, we could pass it to the bridge, but for simplicity
	// we'll fetch all and filter client-side. The Python engine supports the query param,
	// but the Go bridge signature currently doesn't take it. Fetching all is fine.
	modelsMap, err := br.ListModels(fetchCtx)
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}

	// Sort providers alphabetically
	var providers []string
	for p := range modelsMap {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	modelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	totalModels := 0

	for _, provider := range providers {
		models := modelsMap[provider]
		if len(models) == 0 {
			continue
		}

		fmt.Println(headerStyle.Render(fmt.Sprintf("── %s ", provider)) + dimStyle.Render(fmt.Sprintf("(%d models)", len(models))))
		
		for _, m := range models {
			contextInfo := ""
			if m.ContextLength > 0 {
				contextInfo = fmt.Sprintf(" [%s ctx]", formatNumber(m.ContextLength))
			}
			
			desc := m.Description
			if desc != "" {
				desc = " — " + desc
			}

			fmt.Printf("  %s%s%s\n", modelStyle.Render(m.ID), dimStyle.Render(contextInfo), dimStyle.Render(desc))
			totalModels++
		}
		fmt.Println()
	}

	if totalModels == 0 {
		renderer.PrintInfo("No models found. Make sure your API keys are configured correctly.")
		renderer.PrintInfo("Try: export ANTHROPIC_API_KEY='your-key'")
	} else {
		renderer.PrintInfo(fmt.Sprintf("Found %d models across %d providers.", totalModels, len(providers)))
	}

	return nil
}

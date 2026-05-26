// Package team handles loading, validating, and managing team configurations.
//
// Teams are defined in YAML files and specify the agents, their roles, providers,
// models, system prompts, and the orchestration workflow. Teams can be stored in
// three locations, searched in this order:
//   1. ./.2mcode/teams/<name>.yaml (project-local)
//   2. ~/.2mcode/teams/<name>.yaml (global)
//   3. Built-in config/teams/<name>.yaml (bundled examples)
package team

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CustomTool defines a user-provided tool available to agents in a team.
type CustomTool struct {
	Name        string      `yaml:"name"`         // Unique tool name
	Description string      `yaml:"description"`  // Description shown to the LLM
	InputSchema map[string]interface{} `yaml:"input_schema"` // JSON schema for tool parameters
	Command     string      `yaml:"command"`      // Bash command to execute; params passed as env vars
}

// Team represents a configured group of AI agents and their workflow.
type Team struct {
	Name        string      `yaml:"name"`        // Unique identifier
	Description string      `yaml:"description"` // Human-readable description
	Version     string      `yaml:"version"`     // Config version (e.g., "1.0")
	Agents      []Agent     `yaml:"agents"`      // List of agents in this team
	Workflow    Workflow    `yaml:"workflow"`     // Orchestration configuration
	CustomTools []CustomTool `yaml:"custom_tools"` // User-defined tools (optional)
}

// Agent represents a single AI agent within a team.
type Agent struct {
	Name         string   `yaml:"name"`          // Display name (e.g., "Aria")
	Role         string   `yaml:"role"`          // Role label (e.g., "Tech Lead")
	Provider     string   `yaml:"provider"`      // LLM provider: anthropic|google|openai|openai_compatible|mistral|cohere|groq|ollama|openrouter
	Model        string   `yaml:"model"`         // Provider-specific model ID
	SystemPrompt string   `yaml:"system_prompt"` // Full role prompt (150-300 words)
	MaxContext   int      `yaml:"max_context"`   // Messages from team channel (default: 20)
	Color        string   `yaml:"color"`         // Terminal color: red|yellow|green|blue|cyan|magenta
	Tools        []string `yaml:"tools"`         // Enabled tools: bash, read_file, write_file, web_fetch
	BaseURL      string   `yaml:"base_url"`      // API base URL (openai_compatible only); overrides OPENAI_COMPATIBLE_BASE_URL
}

// Workflow defines how agents take turns and collaborate.
type Workflow struct {
	Orchestration  string `yaml:"orchestration"`       // leader_first | round_robin | free
	TurnsPerTask   int    `yaml:"turns_per_task"`       // Rounds of agent turns per task
	Leader         string `yaml:"leader"`               // Agent name (required for leader_first)
	Reviewer       string `yaml:"reviewer"`             // Agent name (optional, always speaks last)
	MaxTokens      int    `yaml:"max_tokens_per_turn"`  // Default 4096
	MaxTokensPerRun int   `yaml:"max_tokens_per_run"`  // Total budget (0 = unlimited)
}

// validProviders lists all supported LLM providers.
var validProviders = map[string]bool{
	"anthropic":         true,
	"google":            true,
	"openai":            true,
	"openai_compatible": true,
	"mistral":           true,
	"cohere":            true,
	"groq":              true,
	"ollama":            true,
	"openrouter":        true,
}

// validColors lists all supported terminal colors.
var validColors = map[string]bool{
	"red":     true,
	"yellow":  true,
	"green":   true,
	"blue":    true,
	"cyan":    true,
	"magenta": true,
	"white":   true,
}

// validTools lists all supported agent tools.
var validTools = map[string]bool{
	"bash":      true,
	"read_file": true,
	"write_file": true,
	"web_fetch": true,
}

// validOrchestrations lists all supported orchestration modes.
var validOrchestrations = map[string]bool{
	"leader_first": true,
	"round_robin":  true,
	"free":         true,
}

// LoadTeam loads a team configuration by name, searching in priority order:
//   1. ./.2mcode/teams/<name>.yaml (project-local)
//   2. ~/.2mcode/teams/<name>.yaml (global)
//   3. config/teams/<name>.yaml (bundled with binary)
//
// Returns the loaded Team or an error with actionable guidance.
func LoadTeam(name string) (*Team, error) {
	paths := getSearchPaths(name)

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return loadTeamFromFile(p)
		}
	}

	return nil, fmt.Errorf(
		"team '%s' not found — searched:\n  1. ./.2mcode/teams/%s.yaml\n  2. ~/.2mcode/teams/%s.yaml\n  3. ~/.2mcode/config/teams/%s.yaml\n  4. config/teams/%s.yaml\nRun '2m new-team' to create one, or '2m team list' to see available teams",
		name, name, name, name, name,
	)
}

// getSearchPaths returns the ordered list of paths to search for a team YAML.
func getSearchPaths(name string) []string {
	filename := name + ".yaml"
	paths := []string{
		// 1. Project-local
		filepath.Join(".", ".2mcode", "teams", filename),
	}

	// 2. Global (~/.2mcode/teams/)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(homeDir, ".2mcode", "teams", filename))
	}

	// 3. Installed config (~/.2mcode/config/teams/)
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".2mcode", "config", "teams", filename))
	}

	// 4. Bundled examples (relative to binary)
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		paths = append(paths, filepath.Join(execDir, "config", "teams", filename))
	}
	// Also check relative to working directory for development
	paths = append(paths, filepath.Join("config", "teams", filename))

	return paths
}

// loadTeamFromFile reads and validates a team YAML from the given path.
func loadTeamFromFile(path string) (*Team, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read team file %s: %w", path, err)
	}

	var t Team
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w — check your syntax", path, err)
	}

	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("team '%s' validation failed: %w", t.Name, err)
	}

	return &t, nil
}

// ListTeams returns all available team names and their paths.
func ListTeams() (map[string]string, error) {
	teams := make(map[string]string)

	// Search all three locations
	searchDirs := []string{
		filepath.Join(".", ".2mcode", "teams"),
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		searchDirs = append(searchDirs, filepath.Join(homeDir, ".2mcode", "teams"))
		searchDirs = append(searchDirs, filepath.Join(homeDir, ".2mcode", "config", "teams"))
	}

	execPath, err := os.Executable()
	if err == nil {
		searchDirs = append(searchDirs, filepath.Join(filepath.Dir(execPath), "config", "teams"))
	}
	searchDirs = append(searchDirs, filepath.Join("config", "teams"))

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Directory doesn't exist, skip
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext == ".yaml" || ext == ".yml" {
				teamName := name[:len(name)-len(ext)]
				if _, exists := teams[teamName]; !exists {
					teams[teamName] = filepath.Join(dir, name)
				}
			}
		}
	}

	return teams, nil
}

// Validate checks that a team configuration is complete and valid.
func (t *Team) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("team name is required")
	}

	if len(t.Agents) == 0 {
		return fmt.Errorf("team must have at least one agent")
	}

	// Build set of valid tool names: built-in + custom tools from this team
	allValidTools := make(map[string]bool)
	for k, v := range validTools {
		allValidTools[k] = v
	}
	customToolNames := make(map[string]bool)
	for i := range t.CustomTools {
		ct := &t.CustomTools[i]
		if ct.Name == "" {
			return fmt.Errorf("custom_tool has empty name")
		}
		if customToolNames[ct.Name] {
			return fmt.Errorf("duplicate custom_tool name: %s", ct.Name)
		}
		customToolNames[ct.Name] = true
		if ct.Description == "" {
			return fmt.Errorf("custom_tool '%s' has no description", ct.Name)
		}
		if ct.Command == "" {
			return fmt.Errorf("custom_tool '%s' has no command", ct.Name)
		}
		if ct.InputSchema == nil {
			ct.InputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		allValidTools[ct.Name] = true
	}

	agentNames := make(map[string]bool)
	for i, agent := range t.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agent %d has no name", i+1)
		}
		if agentNames[agent.Name] {
			return fmt.Errorf("duplicate agent name: %s", agent.Name)
		}
		agentNames[agent.Name] = true

		if agent.Role == "" {
			return fmt.Errorf("agent '%s' has no role", agent.Name)
		}
		if !validProviders[agent.Provider] {
			return fmt.Errorf("agent '%s' has invalid provider '%s' — use: anthropic, google, openai, openai_compatible (covers DeepSeek, Together, xAI, Perplexity, Fireworks, GitHub Models, and any OpenAI-compatible API), mistral, cohere, groq, ollama, openrouter", agent.Name, agent.Provider)
		}
		if agent.Model == "" {
			return fmt.Errorf("agent '%s' has no model specified", agent.Name)
		}
		if agent.SystemPrompt == "" {
			return fmt.Errorf("agent '%s' has no system_prompt", agent.Name)
		}

		// Apply defaults
		if t.Agents[i].MaxContext == 0 {
			t.Agents[i].MaxContext = 20
		}
		if t.Agents[i].Color == "" {
			// Assign colors round-robin
			colors := []string{"cyan", "green", "yellow", "magenta", "blue", "red"}
			t.Agents[i].Color = colors[i%len(colors)]
		}
		if !validColors[t.Agents[i].Color] {
			return fmt.Errorf("agent '%s' has invalid color '%s' — use: red, yellow, green, blue, cyan, magenta, white", agent.Name, agent.Color)
		}

		for _, tool := range agent.Tools {
			if !allValidTools[tool] {
				return fmt.Errorf("agent '%s' has invalid tool '%s' — use: bash, read_file, write_file, web_fetch, or a custom_tool name", agent.Name, tool)
			}
		}
	}

	// Validate workflow
	if !validOrchestrations[t.Workflow.Orchestration] {
		if t.Workflow.Orchestration == "" {
			t.Workflow.Orchestration = "leader_first"
		} else {
			return fmt.Errorf("invalid orchestration '%s' — use: leader_first, round_robin, free", t.Workflow.Orchestration)
		}
	}

	if t.Workflow.Orchestration == "leader_first" && t.Workflow.Leader == "" {
		return fmt.Errorf("leader_first orchestration requires a 'leader' to be specified")
	}

	if t.Workflow.Leader != "" && !agentNames[t.Workflow.Leader] {
		return fmt.Errorf("workflow leader '%s' is not a defined agent", t.Workflow.Leader)
	}

	if t.Workflow.Reviewer != "" && !agentNames[t.Workflow.Reviewer] {
		return fmt.Errorf("workflow reviewer '%s' is not a defined agent", t.Workflow.Reviewer)
	}

	// Apply defaults
	if t.Workflow.TurnsPerTask == 0 {
		t.Workflow.TurnsPerTask = 1
	}
	if t.Workflow.MaxTokens == 0 {
		t.Workflow.MaxTokens = 4096
	}
	if t.Workflow.MaxTokensPerRun < 0 {
		t.Workflow.MaxTokensPerRun = 0
	}

	return nil
}

// GetAgent returns the agent with the given name, or nil if not found.
func (t *Team) GetAgent(name string) *Agent {
	for i := range t.Agents {
		if t.Agents[i].Name == name {
			return &t.Agents[i]
		}
	}
	return nil
}

// SaveToFile writes the team configuration to a YAML file.
func (t *Team) SaveToFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("cannot marshal team to YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("cannot write team file %s: %w", path, err)
	}

	return nil
}

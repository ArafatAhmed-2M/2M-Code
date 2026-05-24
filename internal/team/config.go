// Package team provides global configuration management for 2M Code.
//
// Global config lives at ~/.2mcode/config.yaml and stores default settings
// and provider API key references. API keys themselves are always read from
// environment variables — never stored in the config file.
package team

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the user's global 2M Code configuration.
type GlobalConfig struct {
	// DefaultTeam is the team used when none is specified.
	DefaultTeam string `yaml:"default_team"`

	// DefaultProvider is the provider used when creating new teams.
	DefaultProvider string `yaml:"default_provider"`

	// SessionsDir overrides the default sessions directory (~/.2mcode/sessions/).
	SessionsDir string `yaml:"sessions_dir"`

	// Verbose enables verbose logging output.
	Verbose bool `yaml:"verbose"`
}

// configDir returns the path to the 2M Code config directory (~/.2mcode/).
func configDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w — set HOME environment variable", err)
	}
	return filepath.Join(homeDir, ".2mcode"), nil
}

// ConfigPath returns the full path to the global config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// SessionsPath returns the directory where session databases are stored.
// Uses the configured sessions_dir if set, otherwise defaults to ~/.2mcode/sessions/.
func SessionsPath(teamName string) (string, error) {
	cfg, _ := LoadConfig()
	if cfg != nil && cfg.SessionsDir != "" {
		return filepath.Join(cfg.SessionsDir, teamName), nil
	}

	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sessions", teamName), nil
}

// TeamsDir returns the global teams directory (~/.2mcode/teams/).
func TeamsDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "teams"), nil
}

// LoadConfig loads the global configuration from ~/.2mcode/config.yaml.
// Returns a default config if the file doesn't exist.
func LoadConfig() (*GlobalConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return &GlobalConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if no config file exists
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config YAML in %s: %w", path, err)
	}

	return &cfg, nil
}

// SaveConfig writes the global configuration to ~/.2mcode/config.yaml.
// Creates the config directory if it doesn't exist.
func SaveConfig(cfg *GlobalConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config file %s: %w", path, err)
	}

	return nil
}

// EnsureConfigDir creates the ~/.2mcode/ directory structure if it doesn't exist.
// This is called on first run to set up the environment.
func EnsureConfigDir() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	// Create main directories
	dirs := []string{
		dir,
		filepath.Join(dir, "teams"),
		filepath.Join(dir, "sessions"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0750); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", d, err)
		}
	}

	return nil
}

// GetProviderAPIKey returns the API key for a provider from environment variables.
// Never reads keys from config files — keys are always from env vars.
//
// Provider to env var mapping:
//   - anthropic          → ANTHROPIC_API_KEY
//   - google             → GOOGLE_API_KEY
//   - openai             → OPENAI_API_KEY
//   - openai_compatible  → OPENAI_COMPATIBLE_API_KEY
//   - mistral            → MISTRAL_API_KEY
//   - cohere             → COHERE_API_KEY
//   - groq               → GROQ_API_KEY
//   - openrouter         → OPENROUTER_API_KEY
//   - ollama             → (none — local, no API key needed)
func GetProviderAPIKey(provider string) (string, error) {
	// Ollama is local — no API key needed
	if provider == "ollama" {
		return "", nil
	}

	envVars := map[string]string{
		"anthropic":         "ANTHROPIC_API_KEY",
		"google":            "GOOGLE_API_KEY",
		"openai":            "OPENAI_API_KEY",
		"openai_compatible": "OPENAI_COMPATIBLE_API_KEY",
		"mistral":           "MISTRAL_API_KEY",
		"cohere":            "COHERE_API_KEY",
		"groq":              "GROQ_API_KEY",
		"openrouter":        "OPENROUTER_API_KEY",
	}

	envVar, ok := envVars[provider]
	if !ok {
		return "", fmt.Errorf("unknown provider '%s' — supported: anthropic, google, openai, mistral, cohere, groq, ollama, openrouter", provider)
	}

	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf(
			"%s is not set — set it with:\n  export %s='your-key-here'",
			envVar, envVar,
		)
	}

	return key, nil
}

// ValidateProviderKeys checks that all providers used in a team have API keys set.
// When OPENROUTER_API_KEY is available, provider-specific keys are not required
// since OpenRouter can proxy any model through its unified API.
// Returns a list of missing keys.
func ValidateProviderKeys(t *Team) []string {
	openRouterAvailable := os.Getenv("OPENROUTER_API_KEY") != ""

	seen := make(map[string]bool)
	var missing []string

	for _, agent := range t.Agents {
		if seen[agent.Provider] {
			continue
		}
		seen[agent.Provider] = true

		// Ollama runs locally — no API key needed
		if agent.Provider == "ollama" {
			continue
		}

		// When OpenRouter key is set, it can proxy any provider's models —
		// no need to require per-provider keys
		if openRouterAvailable {
			continue
		}

		if _, err := GetProviderAPIKey(agent.Provider); err != nil {
			missing = append(missing, agent.Provider)
		}
	}

	return missing
}

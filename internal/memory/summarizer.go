package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/2mcode/2mcode/internal/bridge"
)

// Default summarization models per provider — chosen for speed + cost.
var summarizationModels = []struct {
	EnvVar string
	Provider string
	Model    string
}{
	{"OPENROUTER_API_KEY", "openrouter", "qwen/qwen3-coder:free"},
	{"ANTHROPIC_API_KEY", "anthropic", "claude-sonnet-4-6"},
	{"OPENAI_API_KEY", "openai", "gpt-4o-mini"},
	{"GOOGLE_API_KEY", "google", "gemini-1.5-flash"},
	{"MISTRAL_API_KEY", "mistral", "mistral-small-latest"},
	{"GROQ_API_KEY", "groq", "llama3-8b-8192"},
	{"COHERE_API_KEY", "cohere", "command-r"},
}

// Summarizer creates memory entries from session transcripts by calling an LLM.
// Auto-detects which provider to use based on available API keys.
type Summarizer struct {
	bridge   *bridge.Bridge
	store    Store
	provider string
	model    string
}

// NewSummarizer creates a Summarizer backed by the given bridge and store.
// Auto-detects the best available provider for summarization.
// If no provider key is found, summarization will be disabled (returns nil).
func NewSummarizer(br *bridge.Bridge, store Store) *Summarizer {
	s := &Summarizer{
		bridge: br,
		store:  store,
	}
	s.detectProvider()
	return s
}

// detectProvider finds the first available provider from the env.
func (s *Summarizer) detectProvider() {
	for _, p := range summarizationModels {
		if os.Getenv(p.EnvVar) != "" {
			s.provider = p.Provider
			s.model = p.Model
			return
		}
	}
	// No provider key found — memory will be disabled
}

// Store returns the underlying Store used by this Summarizer.
func (s *Summarizer) Store() Store {
	return s.store
}

// Enabled returns true if a provider was detected for summarization.
func (s *Summarizer) Enabled() bool {
	return s.provider != ""
}

// Provider returns the detected provider name.
func (s *Summarizer) Provider() string {
	return s.provider
}

// Model returns the detected model name.
func (s *Summarizer) Model() string {
	return s.model
}

// SummarizeSession sends the conversation transcript to the LLM, saves
// the resulting summary as a memory entry, and returns the entry. Errors
// are returned but the caller may safely ignore them (memory is best-effort).
// Returns nil entry if no provider was detected.
func (s *Summarizer) SummarizeSession(
	ctx context.Context,
	teamName, sessionID, task, transcript string,
) (*Entry, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("memory: no provider API key found — set any LLM provider env var")
	}

	systemPrompt := `You are a memory summarizer for 2M Code, an AI coding assistant.

Your role is to read a conversation transcript and extract the most important
information that will be useful in future sessions. Focus on:

1. What was accomplished — the main outcome
2. Key decisions — architecture, design, technology choices
3. Code patterns — naming conventions, project structure, testing style
4. User preferences — anything the user specifically asked for or prefers
5. Unfinished work — items that were identified but not completed

Return ONLY a valid JSON object with these exact fields (no markdown, no
backticks, no extra text):
{
  "summary": "2-3 sentence summary of what was done",
  "key_decisions": ["decision 1", "decision 2"],
  "code_patterns": ["pattern 1"],
  "unfinished": ["item 1"]
}`

	req := bridge.AgentRequest{
		Provider:  s.provider,
		Model:     s.model,
		System:    systemPrompt,
		Messages:  []bridge.MessagePayload{
			{Role: "user", Content: transcript},
		},
		MaxTokens: 2048,
	}

	resp, err := s.bridge.Call(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("memory summarization call failed: %w", err)
	}

	entry := &Entry{
		ID:        fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		SessionID: sessionID,
		TeamName:  teamName,
		Task:      task,
		Summary:   resp.Content,
		CreatedAt: time.Now(),
	}

	if err := s.store.Save(*entry); err != nil {
		return nil, fmt.Errorf("memory save failed: %w", err)
	}

	return entry, nil
}

// BuildContext loads recent memory entries and formats them into a context
// string that can be injected into agent system prompts.
func (s *Summarizer) BuildContext(teamName string, limit int) (string, error) {
	entries, err := s.store.LoadRecent(teamName, limit)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("[PAST SESSION MEMORY]\n")
	sb.WriteString("The following are summaries from previous sessions with this team.\n")
	sb.WriteString("Use this context to maintain continuity across sessions.\n\n")

	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("--- Session %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Task: %s\n", e.Task))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", e.Summary))
		sb.WriteString("\n")
	}

	sb.WriteString("[/PAST SESSION MEMORY]")
	return sb.String(), nil
}

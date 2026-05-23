package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/2mcode/2mcode/internal/bridge"
)

// Summarizer creates memory entries from session transcripts by calling an LLM.
// It uses whatever provider+model the caller provides — no hardcoded defaults.
type Summarizer struct {
	bridge *bridge.Bridge
	store  Store
}

// NewSummarizer creates a Summarizer backed by the given bridge and store.
func NewSummarizer(br *bridge.Bridge, store Store) *Summarizer {
	return &Summarizer{
		bridge: br,
		store:  store,
	}
}

// Store returns the underlying Store used by this Summarizer.
func (s *Summarizer) Store() Store {
	return s.store
}

// SummarizeSession sends the conversation transcript to the given provider/model,
// saves the resulting summary as a memory entry, and returns the entry. Errors
// are returned but the caller may safely ignore them (memory is best-effort).
func (s *Summarizer) SummarizeSession(
	ctx context.Context,
	teamName, sessionID, task, transcript, provider, model string,
) (*Entry, error) {
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
		Provider:  provider,
		Model:     model,
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

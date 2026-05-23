// Package orchestrator implements the core engine that manages agent turns,
// tool execution, and the collaborative workflow.
//
// The orchestrator is the heart of 2M Code: it takes a team, a task, and a
// renderer, then coordinates agents through their turns — posting messages to
// the event bus, calling the Python agent engine via the bridge, handling tool
// use loops, and rendering output to the terminal.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/2mcode/2mcode/internal/bridge"
	"github.com/2mcode/2mcode/internal/bus"
	"github.com/2mcode/2mcode/internal/memory"
	"github.com/2mcode/2mcode/internal/team"
)

// Renderer is the interface for displaying agent output to the user.
// The CLI renderer implements this interface.
type Renderer interface {
	// PrintAgentStart shows the agent name badge before their response.
	PrintAgentStart(agent team.Agent)

	// PrintAgentText streams text content from an agent's response.
	PrintAgentText(agent team.Agent, text string)

	// PrintAgentEnd closes the agent's response section.
	PrintAgentEnd(agent team.Agent)

	// PrintToolCall shows a tool being invoked.
	PrintToolCall(agent team.Agent, toolName string, toolInput map[string]interface{})

	// PrintToolResult shows the result of a tool invocation.
	PrintToolResult(agent team.Agent, toolName string, result string)

	// PrintSummary shows the task completion summary.
	PrintSummary(turns int, totalInputTokens int, totalOutputTokens int, costUSD float64, duration time.Duration)

	// PrintError shows an error message.
	PrintError(msg string)

	// PrintInfo shows an informational message.
	PrintInfo(msg string)
}

// Orchestrator coordinates agent turns for a team task.
type Orchestrator struct {
	eventBus         *bus.Bus
	bridge           *bridge.Bridge
	renderer         Renderer
	memorySummarizer *memory.Summarizer // nil = memory disabled
}

// New creates a new Orchestrator with the given dependencies.
func New(eventBus *bus.Bus, br *bridge.Bridge, renderer Renderer) *Orchestrator {
	return &Orchestrator{
		eventBus: eventBus,
		bridge:   br,
		renderer: renderer,
	}
}

// WithMemory enables persistent memory for the orchestrator.
// If set, the orchestrator will save session summaries after RunTask
// and inject relevant past context into agent system prompts.
func (o *Orchestrator) WithMemory(s *memory.Summarizer) *Orchestrator {
	o.memorySummarizer = s
	return o
}

// RunTask executes a complete task with the given team.
//
// This is the main entry point for the `2m run` command. It:
//   1. Creates a new session and posts the user's task
//   2. Runs the leader agent (if leader_first orchestration)
//   3. Iterates through worker agents for the configured number of turns
//   4. Runs the reviewer agent (if configured)
//   5. Prints a completion summary
func (o *Orchestrator) RunTask(ctx context.Context, t *team.Team, sessionID, task string) error {
	startTime := time.Now()
	totalInputTokens := 0
	totalOutputTokens := 0
	turnCount := 0

	// Create session
	if err := o.eventBus.CreateSession(sessionID, t.Name); err != nil {
		return fmt.Errorf("cannot create session: %w", err)
	}

	// Post the user's task to the team channel
	if err := o.eventBus.Post(sessionID, "user", "user", task); err != nil {
		return fmt.Errorf("cannot post task: %w", err)
	}

	// Get the turn schedule
	schedule := BuildSchedule(t)

	// Execute each turn
	budgetExceeded := false
	for _, agentName := range schedule {
		if budgetExceeded {
			break
		}

		agent := t.GetAgent(agentName)
		if agent == nil {
			o.renderer.PrintError(fmt.Sprintf("Agent '%s' not found in team — skipping", agentName))
			continue
		}

		// Check token budget before this turn
		if t.Workflow.MaxTokensPerRun > 0 && (totalInputTokens+totalOutputTokens) >= t.Workflow.MaxTokensPerRun {
			o.renderer.PrintInfo(fmt.Sprintf("Token budget of %s exceeded — ending run early", formatNumber(t.Workflow.MaxTokensPerRun)))
			budgetExceeded = true
			break
		}

		input, output, err := o.runAgentTurn(ctx, t, sessionID, *agent)
		if err != nil {
			o.renderer.PrintError(fmt.Sprintf("Agent '%s' failed: %s", agent.Name, err))
			continue
		}

		totalInputTokens += input
		totalOutputTokens += output
		turnCount++
	}

	// Print completion summary with cost estimate
	duration := time.Since(startTime)
	costUSD := EstimateCost(t.Agents[0].Model, totalInputTokens, totalOutputTokens)
	o.renderer.PrintSummary(turnCount, totalInputTokens, totalOutputTokens, costUSD, duration)

	// Save memory for this session (best-effort)
	if o.memorySummarizer != nil && turnCount > 0 {
		o.saveSessionMemory(ctx, t, sessionID, task)
	}

	return nil
}

// RunChatTurn executes a single chat turn: posts the user message, then runs
// all agents in schedule order. Used by the `2m chat` interactive REPL.
func (o *Orchestrator) RunChatTurn(ctx context.Context, t *team.Team, sessionID, userMessage string) error {
	// Post the user's message
	if err := o.eventBus.Post(sessionID, "user", "user", userMessage); err != nil {
		return fmt.Errorf("cannot post user message: %w", err)
	}

	// Get the turn schedule
	schedule := BuildSchedule(t)

	// Execute each agent turn
	for _, agentName := range schedule {
		agent := t.GetAgent(agentName)
		if agent == nil {
			continue
		}

		_, _, err := o.runAgentTurn(ctx, t, sessionID, *agent)
		if err != nil {
			o.renderer.PrintError(fmt.Sprintf("Agent '%s' failed: %s", agent.Name, err))
			continue
		}
	}

	// Save memory after each chat turn (best-effort)
	if o.memorySummarizer != nil {
		o.saveSessionMemory(ctx, t, sessionID, userMessage)
	}

	return nil
}

// runAgentTurn executes a single agent's turn:
//   1. Get history from the event bus
//   2. Build the request with the agent's system prompt
//   3. Call the Python engine via the bridge (with streaming if available)
//   4. Handle tool use loops (up to 5 iterations)
//   5. Post the final response to the event bus
//   6. Render the output
func (o *Orchestrator) runAgentTurn(
	ctx context.Context,
	t *team.Team,
	sessionID string,
	agent team.Agent,
) (inputTokens int, outputTokens int, err error) {
	history, err := o.eventBus.GetHistory(sessionID, agent.MaxContext)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot get history: %w", err)
	}

	messages := o.formatMessages(t, history)
	customToolDefs := o.buildCustomToolDefs(t)

	// Inject memory context into system prompt if available
	systemPrompt := agent.SystemPrompt
	if o.memorySummarizer != nil {
		if ctx, err := o.memorySummarizer.BuildContext(t.Name, 5); err == nil && ctx != "" {
			systemPrompt = systemPrompt + "\n\n" + ctx
		}
	}

	req := bridge.AgentRequest{
		Provider:    agent.Provider,
		Model:       agent.Model,
		System:      systemPrompt,
		Messages:    messages,
		Tools:       agent.Tools,
		CustomTools: customToolDefs,
		MaxTokens:   t.Workflow.MaxTokens,
	}

	o.renderer.PrintAgentStart(agent)

	resp, err := o.callAgentWithStreaming(ctx, req, agent, t)
	if err != nil {
		o.renderer.PrintAgentEnd(agent)
		return 0, 0, fmt.Errorf("bridge call failed: %w", err)
	}

	inputTokens += resp.InputTokens
	outputTokens += resp.OutputTokens

	// 5. Handle tool use loop (max 5 iterations to prevent runaway)
	maxToolIterations := 5
	for iteration := 0; len(resp.ToolCalls) > 0 && iteration < maxToolIterations; iteration++ {
		for _, tc := range resp.ToolCalls {
			o.renderer.PrintToolCall(agent, tc.Name, tc.Input)

			result := ExecuteTool(tc.Name, tc.Input, t.CustomTools)
			o.renderer.PrintToolResult(agent, tc.Name, result)

			toolResultContent := fmt.Sprintf("[Tool Result - %s]: %s", tc.Name, result)
			if err := o.eventBus.Post(sessionID, agent.Name, "user", toolResultContent); err != nil {
				return inputTokens, outputTokens, fmt.Errorf("cannot post tool result: %w", err)
			}
		}

		history, err = o.eventBus.GetHistory(sessionID, agent.MaxContext)
		if err != nil {
			return inputTokens, outputTokens, fmt.Errorf("cannot get updated history: %w", err)
		}

		req.Messages = o.formatMessages(t, history)

		// Use non-streaming call for tool follow-ups (less text, more compact)
		resp, err = o.bridge.Call(ctx, req)
		if err != nil {
			return inputTokens, outputTokens, fmt.Errorf("bridge call after tools failed: %w", err)
		}

		inputTokens += resp.InputTokens
		outputTokens += resp.OutputTokens
	}

	// 6. Post the final response to the event bus (text already streamed)
	if resp.Content != "" {
		if err := o.eventBus.Post(sessionID, agent.Name, "assistant", resp.Content); err != nil {
			return inputTokens, outputTokens, fmt.Errorf("cannot post agent response: %w", err)
		}
	}

	o.renderer.PrintAgentEnd(agent)

	return inputTokens, outputTokens, nil
}

// callAgentWithStreaming calls the agent with streaming, rendering text chunks
// as they arrive. Returns the assembled AgentResponse.
func (o *Orchestrator) callAgentWithStreaming(
	ctx context.Context,
	req bridge.AgentRequest,
	agent team.Agent,
	t *team.Team,
) (*bridge.AgentResponse, error) {
	req.Stream = true
	var textBuffer strings.Builder

	resp, err := o.bridge.CallStream(ctx, req, func(ev bridge.StreamEvent) {
		switch ev.Type {
		case "text":
			o.renderer.PrintAgentText(agent, ev.Content)
			textBuffer.WriteString(ev.Content)
		case "tool_call":
			// Handled via CallStream return
		case "done":
			// Tokens tracked via response
		case "error":
			// Handled via error return
		}
	})
	if err != nil {
		return nil, err
	}

	// If streaming produced text but response has none, fill it in
	if resp.Content == "" && textBuffer.Len() > 0 {
		resp.Content = textBuffer.String()
	}

	return resp, nil
}

// formatMessages converts event bus history into bridge message payloads.
func (o *Orchestrator) formatMessages(t *team.Team, history []bus.Message) []bridge.MessagePayload {
	messages := make([]bridge.MessagePayload, len(history))
	for i, msg := range history {
		content := msg.Content
		if msg.AgentName != "user" {
			content = fmt.Sprintf("[%s · %s]: %s",
				msg.AgentName,
				getAgentRole(t, msg.AgentName),
				msg.Content,
			)
		}
		messages[i] = bridge.MessagePayload{
			Role:    msg.Role,
			Content: content,
			Name:    msg.AgentName,
		}
	}
	return messages
}

// buildCustomToolDefs converts team custom tools to bridge custom tool defs.
func (o *Orchestrator) buildCustomToolDefs(t *team.Team) []bridge.CustomToolDef {
	defs := make([]bridge.CustomToolDef, len(t.CustomTools))
	for i, ct := range t.CustomTools {
		defs[i] = bridge.CustomToolDef{
			Name:        ct.Name,
			Description: ct.Description,
			InputSchema: ct.InputSchema,
		}
	}
	return defs
}

// saveSessionMemory gets the full session transcript and saves a memory summary.
// Errors are logged via the renderer but do not propagate (best-effort).
func (o *Orchestrator) saveSessionMemory(ctx context.Context, t *team.Team, sessionID, task string) {
	messages, err := o.eventBus.GetAllMessages(sessionID)
	if err != nil {
		o.renderer.PrintInfo(fmt.Sprintf("Memory: cannot load session history: %s", err))
		return
	}

	transcript := o.formatTranscript(messages)
	if transcript == "" {
		return
	}

	entry, err := o.memorySummarizer.SummarizeSession(ctx, t.Name, sessionID, task, transcript)
	if err != nil {
		o.renderer.PrintInfo(fmt.Sprintf("Memory: summarization skipped: %s", err))
		return
	}

	o.renderer.PrintInfo(fmt.Sprintf("Memory: saved summary for this session (%.0f tokens)", float64(len(entry.Summary))/4))
}

// formatTranscript converts event bus messages into a plain-text transcript
// suitable for LLM summarization.
func (o *Orchestrator) formatTranscript(messages []bus.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		speaker := msg.AgentName
		if speaker == "" {
			speaker = msg.Role
		}
		b.WriteString(fmt.Sprintf("[%s]: %s\n", speaker, msg.Content))
	}
	return b.String()
}

// getAgentRole returns the role label for an agent by name, or "Agent" if not found.
func getAgentRole(t *team.Team, agentName string) string {
	agent := t.GetAgent(agentName)
	if agent != nil {
		return agent.Role
	}
	return "Agent"
}

// TokenStats tracks cumulative token usage across a session.
type TokenStats struct {
	InputTokens  int
	OutputTokens int
}

// formatNumber formats an integer with comma separators for display.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// MarshalToolCalls converts tool calls to JSON for storage in the event bus.
func MarshalToolCalls(toolCalls []bridge.ToolCall) (string, error) {
	if len(toolCalls) == 0 {
		return "", nil
	}
	data, err := json.Marshal(toolCalls)
	if err != nil {
		return "", fmt.Errorf("cannot marshal tool calls: %w", err)
	}
	return string(data), nil
}

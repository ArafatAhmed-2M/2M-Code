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
	"time"

	"github.com/2mcode/2mcode/internal/bridge"
	"github.com/2mcode/2mcode/internal/bus"
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
	PrintSummary(turns int, totalInputTokens int, totalOutputTokens int, duration time.Duration)

	// PrintError shows an error message.
	PrintError(msg string)

	// PrintInfo shows an informational message.
	PrintInfo(msg string)
}

// Orchestrator coordinates agent turns for a team task.
type Orchestrator struct {
	eventBus  *bus.Bus
	bridge    *bridge.Bridge
	renderer  Renderer
}

// New creates a new Orchestrator with the given dependencies.
func New(eventBus *bus.Bus, br *bridge.Bridge, renderer Renderer) *Orchestrator {
	return &Orchestrator{
		eventBus: eventBus,
		bridge:   br,
		renderer: renderer,
	}
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
	for _, agentName := range schedule {
		agent := t.GetAgent(agentName)
		if agent == nil {
			o.renderer.PrintError(fmt.Sprintf("Agent '%s' not found in team — skipping", agentName))
			continue
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

	// Print completion summary
	duration := time.Since(startTime)
	o.renderer.PrintSummary(turnCount, totalInputTokens, totalOutputTokens, duration)

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

	return nil
}

// runAgentTurn executes a single agent's turn:
//   1. Get history from the event bus
//   2. Build the request with the agent's system prompt
//   3. Call the Python engine via the bridge
//   4. Handle tool use loops (up to 5 iterations)
//   5. Post the final response to the event bus
//   6. Render the output
func (o *Orchestrator) runAgentTurn(
	ctx context.Context,
	t *team.Team,
	sessionID string,
	agent team.Agent,
) (inputTokens int, outputTokens int, err error) {
	// 1. Get conversation history from the bus
	history, err := o.eventBus.GetHistory(sessionID, agent.MaxContext)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot get history: %w", err)
	}

	// 2. Format messages for the LLM API
	// Prepend agent name to each message so the model knows who said what
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

	// 3. Build the agent request
	req := bridge.AgentRequest{
		Provider:  agent.Provider,
		Model:     agent.Model,
		System:    agent.SystemPrompt,
		Messages:  messages,
		Tools:     agent.Tools,
		MaxTokens: t.Workflow.MaxTokens,
	}

	// Render agent start
	o.renderer.PrintAgentStart(agent)

	// 4. Call the agent engine
	resp, err := o.bridge.Call(ctx, req)
	if err != nil {
		o.renderer.PrintAgentEnd(agent)
		return 0, 0, fmt.Errorf("bridge call failed: %w", err)
	}

	inputTokens += resp.InputTokens
	outputTokens += resp.OutputTokens

	// 5. Handle tool use loop (max 5 iterations to prevent runaway)
	maxToolIterations := 5
	for iteration := 0; len(resp.ToolCalls) > 0 && iteration < maxToolIterations; iteration++ {
		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			o.renderer.PrintToolCall(agent, tc.Name, tc.Input)

			// Execute the tool
			result := ExecuteTool(tc.Name, tc.Input)
			o.renderer.PrintToolResult(agent, tc.Name, result)

			// Post tool result as a message
			toolResultContent := fmt.Sprintf("[Tool Result - %s]: %s", tc.Name, result)
			if err := o.eventBus.Post(sessionID, agent.Name, "user", toolResultContent); err != nil {
				return inputTokens, outputTokens, fmt.Errorf("cannot post tool result: %w", err)
			}
		}

		// Get updated history after tool results
		history, err = o.eventBus.GetHistory(sessionID, agent.MaxContext)
		if err != nil {
			return inputTokens, outputTokens, fmt.Errorf("cannot get updated history: %w", err)
		}

		// Rebuild messages
		messages = make([]bridge.MessagePayload, len(history))
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

		// Call again with tool results
		req.Messages = messages
		resp, err = o.bridge.Call(ctx, req)
		if err != nil {
			return inputTokens, outputTokens, fmt.Errorf("bridge call after tools failed: %w", err)
		}

		inputTokens += resp.InputTokens
		outputTokens += resp.OutputTokens
	}

	// 6. Render and post the final text response
	if resp.Content != "" {
		o.renderer.PrintAgentText(agent, resp.Content)

		// Post to the event bus
		if err := o.eventBus.Post(sessionID, agent.Name, "assistant", resp.Content); err != nil {
			return inputTokens, outputTokens, fmt.Errorf("cannot post agent response: %w", err)
		}
	}

	// Render agent end
	o.renderer.PrintAgentEnd(agent)

	return inputTokens, outputTokens, nil
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

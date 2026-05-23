// Package bridge provides the HTTP client that communicates with the Python
// agent engine running on localhost:8765.
//
// The bridge sends agent requests (provider, model, system prompt, messages,
// tools) to the Python FastAPI server and returns the normalized response.
// All communication is over localhost HTTP — no external network calls.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AgentRequest is the JSON body sent to the Python agent engine's /call endpoint.
type AgentRequest struct {
	Provider  string            `json:"provider"`   // anthropic|google|openai|mistral|cohere|groq|ollama|openrouter
	Model     string            `json:"model"`      // Provider-specific model ID
	System    string            `json:"system"`     // System prompt
	Messages  []MessagePayload  `json:"messages"`   // Conversation history
	Tools     []string          `json:"tools"`      // Enabled tool names
	MaxTokens int               `json:"max_tokens"` // Max response tokens
}

// MessagePayload is a single message in the conversation sent to the engine.
type MessagePayload struct {
	Role    string `json:"role"`    // user | assistant
	Content string `json:"content"` // Message text (may include agent name prefix)
	Name    string `json:"name"`    // Agent name (for context)
}

// AgentResponse is the JSON body returned from the Python agent engine.
type AgentResponse struct {
	Content     string     `json:"content"`      // Text response
	ToolCalls   []ToolCall `json:"tool_calls"`   // Tool use requests
	InputTokens int        `json:"input_tokens"` // Tokens consumed
	OutputTokens int       `json:"output_tokens"` // Tokens generated
}

// ToolCall represents a single tool invocation requested by an agent.
type ToolCall struct {
	Name  string                 `json:"name"`  // Tool name
	Input map[string]interface{} `json:"input"` // Tool input parameters
	ID    string                 `json:"id"`    // Provider-specific tool call ID
}

// ModelInfo represents a single model returned by the /models endpoint.
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextLength int    `json:"context_length"`
}

// Bridge is the HTTP client for communicating with the Python agent engine.
type Bridge struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Bridge targeting the given base URL.
// Default timeout is 120 seconds (LLM calls can be slow).
func New(baseURL string) *Bridge {
	return &Bridge{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// DefaultBridge creates a Bridge targeting the default localhost:8765 engine.
func DefaultBridge() *Bridge {
	return New("http://127.0.0.1:8765")
}

// HealthCheck verifies the Python agent engine is running and healthy.
// Returns nil if healthy, error otherwise.
func (b *Bridge) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("cannot create health check request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent engine not responding at %s: %w — is the Python server running?", b.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent engine unhealthy: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Call sends an agent request to the Python engine and returns the response.
//
// The context is used for cancellation and timeouts. For typical LLM calls,
// expect latency of 2-30 seconds depending on the provider and model.
func (b *Bridge) Call(ctx context.Context, req AgentRequest) (*AgentResponse, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal agent request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/call", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create agent call request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agent engine call failed: %w — check if the Python server is running", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read agent engine response: %w", err)
	}

	// Handle error status codes
	if resp.StatusCode != http.StatusOK {
		// Try to extract error detail from JSON response
		var errDetail struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(respBody, &errDetail) == nil && errDetail.Detail != "" {
			return nil, fmt.Errorf("agent engine error (HTTP %d): %s", resp.StatusCode, errDetail.Detail)
		}
		return nil, fmt.Errorf("agent engine error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var agentResp AgentResponse
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		return nil, fmt.Errorf("cannot parse agent engine response: %w", err)
	}

	return &agentResp, nil
}

// ListModels fetches the list of available models from all providers.
func (b *Bridge) ListModels(ctx context.Context) (map[string][]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create models request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent engine not responding at %s: %w", b.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent engine error: HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read models response: %w", err)
	}

	var models map[string][]ModelInfo
	if err := json.Unmarshal(respBody, &models); err != nil {
		return nil, fmt.Errorf("cannot parse models response: %w", err)
	}

	return models, nil
}

// WaitForReady polls the health endpoint until the engine is ready or the
// context is cancelled. Polls every 200ms.
func (b *Bridge) WaitForReady(ctx context.Context) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for agent engine to start: %w — check Python installation and requirements.txt", ctx.Err())
		case <-ticker.C:
			if err := b.HealthCheck(ctx); err == nil {
				return nil
			}
		}
	}
}

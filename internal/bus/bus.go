// Package bus provides the event bus for inter-agent communication.
//
// The Bus type wraps the SQLite database and provides methods to post messages
// and retrieve conversation history. All SQL uses parameterized queries to
// prevent injection attacks.
package bus

import (
	"database/sql"
	"fmt"
	"time"
)

// Message represents a single message in the team channel.
type Message struct {
	ID        int64     // Auto-incremented message ID
	SessionID string    // Session this message belongs to
	AgentName string    // Name of the agent (or "user" for user input)
	Role      string    // "user", "assistant", or "system"
	Content   string    // Message text content
	ToolCalls string    // JSON-encoded tool calls (nullable)
	CreatedAt time.Time // When the message was created
}

// Bus is the team channel event bus backed by SQLite.
type Bus struct {
	db *sql.DB
}

// New creates a new Bus instance wrapping the given database connection.
func New(db *sql.DB) *Bus {
	return &Bus{db: db}
}

// CreateSession creates a new session record in the database.
// Sessions group messages by team and run.
func (b *Bus) CreateSession(sessionID, teamName string) error {
	_, err := b.db.Exec(
		"INSERT INTO sessions (id, team_name) VALUES (?, ?)",
		sessionID, teamName,
	)
	if err != nil {
		return fmt.Errorf("cannot create session %s: %w", sessionID, err)
	}
	return nil
}

// Post writes a message to the team channel.
//
// This is the primary way agents and users add messages to the shared context.
// All SQL uses parameterized queries — no string concatenation.
func (b *Bus) Post(sessionID, agentName, role, content string) error {
	_, err := b.db.Exec(
		"INSERT INTO messages (session_id, agent_name, role, content) VALUES (?, ?, ?, ?)",
		sessionID, agentName, role, content,
	)
	if err != nil {
		return fmt.Errorf("cannot post message from %s: %w", agentName, err)
	}
	return nil
}

// PostWithToolCalls writes a message with associated tool calls to the team channel.
func (b *Bus) PostWithToolCalls(sessionID, agentName, role, content, toolCalls string) error {
	_, err := b.db.Exec(
		"INSERT INTO messages (session_id, agent_name, role, content, tool_calls) VALUES (?, ?, ?, ?, ?)",
		sessionID, agentName, role, content, toolCalls,
	)
	if err != nil {
		return fmt.Errorf("cannot post message with tool calls from %s: %w", agentName, err)
	}
	return nil
}

// GetHistory retrieves the last `limit` messages from the team channel for the
// given session. Messages are ordered by creation time (oldest first) so they
// can be used directly as LLM conversation history.
//
// When formatting for the LLM API, the agent name is prepended to the content
// so the model knows who said what:
// "[Aria · Tech Lead]: I will break this into three subtasks..."
func (b *Bus) GetHistory(sessionID string, limit int) ([]Message, error) {
	// Subquery to get the last N messages, then order them chronologically
	query := `
		SELECT id, session_id, agent_name, role, content, COALESCE(tool_calls, ''), created_at
		FROM (
			SELECT id, session_id, agent_name, role, content, tool_calls, created_at
			FROM messages
			WHERE session_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, id ASC`

	rows, err := b.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("cannot query message history: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.AgentName, &msg.Role,
			&msg.Content, &msg.ToolCalls, &msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("cannot scan message row: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}

// GetAllMessages retrieves all messages for a session, ordered chronologically.
// Used for the `2m history` command.
func (b *Bus) GetAllMessages(sessionID string) ([]Message, error) {
	query := `
		SELECT id, session_id, agent_name, role, content, COALESCE(tool_calls, ''), created_at
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at ASC, id ASC`

	rows, err := b.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("cannot query all messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.AgentName, &msg.Role,
			&msg.Content, &msg.ToolCalls, &msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("cannot scan message row: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}

// GetLatestSessionID returns the most recent session ID for a team.
// Returns empty string if no sessions exist.
func (b *Bus) GetLatestSessionID(teamName string) (string, error) {
	var sessionID string
	err := b.db.QueryRow(
		"SELECT id FROM sessions WHERE team_name = ? ORDER BY created_at DESC LIMIT 1",
		teamName,
	).Scan(&sessionID)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot query latest session: %w", err)
	}
	return sessionID, nil
}

// MessageCount returns the total number of messages in a session.
func (b *Bus) MessageCount(sessionID string) (int, error) {
	var count int
	err := b.db.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE session_id = ?",
		sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("cannot count messages: %w", err)
	}
	return count, nil
}

// Close closes the underlying database connection.
func (b *Bus) Close() error {
	return b.db.Close()
}

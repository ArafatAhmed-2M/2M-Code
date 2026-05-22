// Package bus provides the SQLite-backed event bus (team channel) for 2M Code.
//
// The event bus is the core communication mechanism between agents. All agent
// messages — including user input — are stored in a shared SQLite database.
// Each agent reads the last N messages as conversation history before generating
// a response. This is what makes agents "see" each other's work.
package bus

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// sessionsSchema defines the SQL for the sessions table.
const sessionsSchema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    team_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

// messagesSchema defines the SQL for the messages table.
const messagesSchema = `
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('user','assistant','system')),
    content TEXT NOT NULL,
    tool_calls TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);`

// messagesIndex creates an index on session_id and created_at for efficient
// history queries.
const messagesIndex = `
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);`

// InitDB opens or creates the SQLite database at the given path and runs
// schema migrations. The database file and its parent directories are created
// if they do not exist.
//
// Returns an open *sql.DB connection or an error with actionable context.
func InitDB(dbPath string) (*sql.DB, error) {
	// Create parent directories if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("cannot create database directory %s: %w — check write permissions", dir, err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database at %s: %w — ensure SQLite is supported on this platform", dbPath, err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot set WAL journal mode: %w", err)
	}

	// Enable foreign key enforcement
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot enable foreign keys: %w", err)
	}

	// Run schema migrations
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return db, nil
}

// migrate executes all schema creation statements.
func migrate(db *sql.DB) error {
	statements := []string{sessionsSchema, messagesSchema, messagesIndex}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("schema migration error on statement: %w", err)
		}
	}
	return nil
}

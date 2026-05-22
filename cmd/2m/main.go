// Package main is the entry point for the 2M Code CLI.
//
// It starts the Python agent engine subprocess, waits for it to be ready
// via health check, then hands off to the Cobra CLI. On exit, the Python
// process is killed.
//
// Usage:
//
//	2m new-team              Create a new team interactively
//	2m run <team> "<task>"   Run a one-shot task with a team
//	2m chat <team>           Start an interactive REPL
//	2m team list             List available teams
//	2m team show <name>      Show team configuration
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/2mcode/2mcode/internal/cli"
	"github.com/2mcode/2mcode/internal/bridge"
	"github.com/2mcode/2mcode/internal/team"
)

func main() {
	// Ensure config directory exists
	if err := team.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot create config directory: %s\n", err)
	}

	// Find the agent engine server.py
	enginePath := findEngineScript()
	if enginePath == "" {
		fmt.Fprintf(os.Stderr, "Error: cannot find agent_engine/server.py\n")
		fmt.Fprintf(os.Stderr, "Make sure the agent_engine directory is in the same location as the 2m binary,\n")
		fmt.Fprintf(os.Stderr, "or set the 2M_ENGINE_PATH environment variable.\n")
		os.Exit(1)
	}

	// Find Python interpreter
	pythonPath := findPython()
	if pythonPath == "" {
		fmt.Fprintf(os.Stderr, "Error: Python 3 is required but not found.\n")
		fmt.Fprintf(os.Stderr, "Install Python 3.11+ and ensure 'python3' or 'python' is in your PATH.\n")
		os.Exit(1)
	}

	// Start the Python agent engine
	engineCmd := exec.Command(pythonPath, enginePath)
	engineCmd.Stdout = os.Stderr // Engine logs go to stderr
	engineCmd.Stderr = os.Stderr
	engineCmd.Dir = filepath.Dir(enginePath)

	// Set up environment
	engineCmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	if err := engineCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot start agent engine: %s\n", err)
		fmt.Fprintf(os.Stderr, "Check that Python dependencies are installed: pip install -r requirements.txt\n")
		os.Exit(1)
	}

	// Ensure the Python process is killed on exit
	defer func() {
		if engineCmd.Process != nil {
			engineCmd.Process.Kill()
			engineCmd.Wait()
		}
	}()

	// Handle OS signals to clean up the Python process
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if engineCmd.Process != nil {
			engineCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	// Wait for the agent engine to be ready
	br := bridge.DefaultBridge()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := br.WaitForReady(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: agent engine failed to start within 10 seconds.\n")
		fmt.Fprintf(os.Stderr, "Check Python installation and requirements.txt.\n")
		fmt.Fprintf(os.Stderr, "Detail: %s\n", err)
		if engineCmd.Process != nil {
			engineCmd.Process.Kill()
		}
		os.Exit(1)
	}

	// Hand off to Cobra CLI
	cli.Execute()
}

// findEngineScript searches for the agent_engine/server.py script.
// Search order:
//  1. 2M_ENGINE_PATH environment variable
//  2. Relative to the executable
//  3. Relative to the current working directory
func findEngineScript() string {
	// 1. Environment variable override
	if envPath := os.Getenv("2M_ENGINE_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// 2. Relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, "agent_engine", "server.py")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Also check parent directory (for bin/ layout)
		candidate = filepath.Join(filepath.Dir(execDir), "agent_engine", "server.py")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// 3. Relative to current working directory
	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "agent_engine", "server.py")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// findPython searches for a Python 3 interpreter.
// Checks python3 first, then python, then py (Windows).
func findPython() string {
	candidates := []string{"python3", "python", "py"}
	for _, name := range candidates {
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}
	}
	return ""
}

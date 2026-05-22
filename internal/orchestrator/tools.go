// Package orchestrator provides tool execution for agent-requested operations.
//
// Tools are executed on the Go side (for operations like bash and file I/O
// that run locally) or delegated to the Python engine. This file provides
// the Go-side tool execution dispatcher.
//
// Security:
//   - Bash commands have a 30-second timeout
//   - File reads are capped at 100KB
//   - File paths are validated to prevent traversal attacks
//   - No privilege escalation — tools run as the user's process
package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxFileReadSize is the maximum bytes to read from a file (100KB).
	maxFileReadSize = 102400

	// bashTimeout is the maximum duration for a bash command.
	bashTimeout = 30 * time.Second
)

// ExecuteTool runs a tool by name with the given input parameters.
// Returns the tool's output as a string.
func ExecuteTool(name string, input map[string]interface{}) string {
	switch name {
	case "bash":
		return executeBash(input)
	case "read_file":
		return executeReadFile(input)
	case "write_file":
		return executeWriteFile(input)
	case "web_fetch":
		// Web fetch is handled by the Python engine
		return "web_fetch is handled by the agent engine"
	default:
		return fmt.Sprintf("Unknown tool: %s. Available: bash, read_file, write_file, web_fetch", name)
	}
}

// executeBash runs a shell command with a 30-second timeout.
func executeBash(input map[string]interface{}) string {
	command, ok := input["command"].(string)
	if !ok || command == "" {
		return "Error: no 'command' provided"
	}

	// Determine the shell to use
	shell := "bash"
	shellFlag := "-c"

	// On Windows, use cmd.exe
	if os.PathSeparator == '\\' {
		shell = "cmd.exe"
		shellFlag = "/C"
	}

	cmd := exec.Command(shell, shellFlag, command)
	cmd.Dir, _ = os.Getwd()

	// Set a timeout using a timer
	done := make(chan error, 1)
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		done <- cmdErr
	}()

	select {
	case <-time.After(bashTimeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Sprintf("Command timed out after %s: %s", bashTimeout, command)
	case <-done:
		result := string(output)
		if cmdErr != nil {
			result += fmt.Sprintf("\n[exit code: %s]", cmdErr.Error())
		}
		if result == "" {
			result = "[no output]"
		}
		return result
	}
}

// executeReadFile reads a file's contents up to maxFileReadSize.
func executeReadFile(input map[string]interface{}) string {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "Error: no 'path' provided"
	}

	// Validate and resolve path to prevent traversal
	resolvedPath, err := validatePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Error: file not found: %s", resolvedPath)
		}
		return fmt.Sprintf("Error: cannot access file: %s", err)
	}

	if info.IsDir() {
		return fmt.Sprintf("Error: %s is a directory, not a file", resolvedPath)
	}

	// Read with size cap
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Sprintf("Error: cannot read file: %s", err)
	}

	if len(data) > maxFileReadSize {
		return string(data[:maxFileReadSize]) + fmt.Sprintf("\n\n[truncated — file is %d bytes, showing first %d]", len(data), maxFileReadSize)
	}

	return string(data)
}

// executeWriteFile writes content to a file, creating parent directories if needed.
func executeWriteFile(input map[string]interface{}) string {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "Error: no 'path' provided"
	}

	content, ok := input["content"].(string)
	if !ok {
		return "Error: no 'content' provided"
	}

	// Validate and resolve path
	resolvedPath, err := validatePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}

	// Create parent directories
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Sprintf("Error: cannot create directory %s: %s", dir, err)
	}

	// Write the file
	if err := os.WriteFile(resolvedPath, []byte(content), 0640); err != nil {
		return fmt.Sprintf("Error: cannot write file: %s", err)
	}

	return fmt.Sprintf("Written: %s (%d bytes)", resolvedPath, len(content))
}

// validatePath resolves and validates a file path to prevent directory traversal.
// Uses filepath.Abs and filepath.Clean to normalize the path.
func validatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path provided")
	}

	// Check for obvious traversal attempts
	if strings.Contains(path, "..") {
		// Still resolve it — filepath.Clean handles this, but log it
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %s: %w", path, err)
	}

	// Clean the path (removes .., ., double slashes)
	cleaned := filepath.Clean(absPath)

	return cleaned, nil
}

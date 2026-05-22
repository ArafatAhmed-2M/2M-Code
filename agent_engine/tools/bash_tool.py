"""
2M Code — Bash Execution Tool

Executes shell commands with a 30-second timeout.
Returns stdout + stderr combined.

Security: Commands run as the user's own process with no privilege escalation.
"""

import logging
import subprocess

logger = logging.getLogger("2mcode.tools.bash")

BASH_TOOL_DEFINITION = {
    "name": "bash",
    "description": "Execute a bash command. Returns stdout and stderr. Timeout: 30 seconds.",
    "input_schema": {
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "description": "The bash command to run",
            }
        },
        "required": ["command"],
    },
}


def execute_bash(tool_input: dict) -> str:
    """
    Execute a bash command and return stdout + stderr.

    Args:
        tool_input: Dict with "command" key containing the shell command.

    Returns:
        Combined stdout and stderr output as a string.

    Raises:
        TimeoutError: If the command exceeds 30 seconds.
    """
    command = tool_input.get("command", "")
    if not command:
        return "Error: No command provided."

    logger.info("Executing bash command (first 100 chars): %s", command[:100])

    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=30,
        )
        output = result.stdout + result.stderr
        if result.returncode != 0:
            output += f"\n[exit code: {result.returncode}]"
        return output if output else "[no output]"
    except subprocess.TimeoutExpired:
        raise TimeoutError(f"Command timed out after 30 seconds: {command[:100]}")

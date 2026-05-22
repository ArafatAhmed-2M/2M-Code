"""
2M Code — Tool Definitions and Execution

Defines the tools available to agents (bash, read_file, write_file, web_fetch)
and provides a central execution function.

Security notes:
- bash: 30s timeout, captures stdout+stderr
- read_file: max 100KB, validates path against traversal
- write_file: validates path against traversal
- web_fetch: max 50KB response, HTTPS preferred
"""

import logging
import os

from tools.bash_tool import execute_bash, BASH_TOOL_DEFINITION
from tools.file_tool import execute_read_file, execute_write_file, READ_FILE_DEFINITION, WRITE_FILE_DEFINITION
from tools.web_tool import execute_web_fetch, WEB_FETCH_DEFINITION

logger = logging.getLogger("2mcode.tools")

# Central tool definition registry
TOOL_DEFINITIONS = {
    "bash": BASH_TOOL_DEFINITION,
    "read_file": READ_FILE_DEFINITION,
    "write_file": WRITE_FILE_DEFINITION,
    "web_fetch": WEB_FETCH_DEFINITION,
}

# Execution dispatcher
_EXECUTORS = {
    "bash": execute_bash,
    "read_file": execute_read_file,
    "write_file": execute_write_file,
    "web_fetch": execute_web_fetch,
}


def get_tool_definitions(tool_names: list[str]) -> list[dict]:
    """
    Get tool definition dicts for the requested tool names.

    Args:
        tool_names: List of tool names to include (e.g., ["bash", "read_file"]).

    Returns:
        List of tool definition dicts suitable for LLM API calls.
    """
    definitions = []
    for name in tool_names:
        if name in TOOL_DEFINITIONS:
            definitions.append(TOOL_DEFINITIONS[name])
        else:
            logger.warning("Unknown tool requested: %s (skipping)", name)
    return definitions


def execute_tool(name: str, tool_input: dict) -> str:
    """
    Execute a tool by name with the given input.

    Args:
        name: Tool name (bash, read_file, write_file, web_fetch).
        tool_input: Tool-specific input parameters.

    Returns:
        String result from the tool execution.
    """
    if name not in _EXECUTORS:
        return f"Unknown tool: {name}. Available tools: {', '.join(_EXECUTORS.keys())}"

    logger.info("Executing tool: %s", name)
    try:
        result = _EXECUTORS[name](tool_input)
        logger.info("Tool %s completed successfully", name)
        return result
    except TimeoutError as e:
        logger.error("Tool %s timed out: %s", name, str(e))
        return f"Tool '{name}' timed out after 30 seconds."
    except PermissionError as e:
        logger.error("Tool %s permission denied: %s", name, str(e))
        return f"Tool '{name}' permission denied: {e}"
    except FileNotFoundError as e:
        logger.error("Tool %s file not found: %s", name, str(e))
        return f"Tool '{name}' file not found: {e}"
    except OSError as e:
        logger.error("Tool %s OS error: %s", name, str(e))
        return f"Tool '{name}' error: {e}"

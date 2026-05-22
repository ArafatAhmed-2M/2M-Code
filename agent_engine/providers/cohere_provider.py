"""
2M Code — Cohere Provider Adapter

Adapts the Cohere SDK (Command models) to the unified 2M Code response format.
Supports all command-r and command models via live model listing.

API Key env var: COHERE_API_KEY
Get a key at: https://dashboard.cohere.com/api-keys
"""

import logging
import os

import cohere

logger = logging.getLogger("2mcode.providers.cohere")

_client = None


def _get_client() -> cohere.Client:
    """
    Lazily initialize the Cohere client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("COHERE_API_KEY")
    if not api_key:
        raise ValueError(
            "COHERE_API_KEY environment variable is not set. "
            "Set it with: export COHERE_API_KEY='your-key-here'\n"
            "Get a free key at: https://dashboard.cohere.com/api-keys"
        )

    _client = cohere.Client(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available Cohere models from the live API.

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        client = _get_client()
        resp = client.models.list()
        models = []
        for m in resp.models:
            # Only return chat-capable models
            endpoints = getattr(m, "endpoints", []) or []
            if "chat" not in endpoints and "generate" not in endpoints:
                continue
            models.append({
                "id": m.name,
                "name": m.name,
                "description": getattr(m, "description", ""),
                "context_length": getattr(m, "context_length", 0),
            })
        return models
    except Exception as e:
        logger.warning("Could not fetch Cohere models from API: %s — using defaults", e)
        return [
            {"id": "command-r-plus", "name": "command-r-plus", "description": "Most capable Command model", "context_length": 128000},
            {"id": "command-r", "name": "command-r", "description": "Balanced Command model", "context_length": 128000},
            {"id": "command", "name": "command", "description": "Fast Command model", "context_length": 4096},
            {"id": "command-light", "name": "command-light", "description": "Lightest Command model", "context_length": 4096},
        ]


def _convert_tools(tools: list[dict]) -> list:
    """Convert 2M Code tool definitions to Cohere tool format."""
    cohere_tools = []
    for tool in tools:
        schema = tool.get("input_schema", {})
        properties = schema.get("properties", {})
        required = schema.get("required", [])

        parameter_definitions = {}
        for name, prop in properties.items():
            parameter_definitions[name] = cohere.ToolParameterDefinitionsValue(
                description=prop.get("description", ""),
                type=prop.get("type", "str"),
                required=name in required,
            )

        cohere_tools.append(cohere.Tool(
            name=tool["name"],
            description=tool["description"],
            parameter_definitions=parameter_definitions,
        ))
    return cohere_tools


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the Cohere API and return a normalized response.

    Args:
        model: Cohere model ID (e.g., "command-r-plus").
        system: System prompt for the agent's identity.
        messages: Conversation history as OpenAI-compatible message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Convert message history to Cohere chat history format
    # Cohere uses "USER" and "CHATBOT" roles
    chat_history = []
    user_message = ""

    for i, msg in enumerate(messages):
        role = "USER" if msg["role"] == "user" else "CHATBOT"
        if i == len(messages) - 1 and msg["role"] == "user":
            user_message = msg["content"]
        else:
            chat_history.append({"role": role, "message": msg["content"]})

    if not user_message and messages:
        user_message = messages[-1]["content"]

    cohere_tools = _convert_tools(tools) if tools else []

    logger.info("Calling Cohere API: model=%s max_tokens=%d tools=%d",
                model, max_tokens, len(cohere_tools))

    try:
        kwargs = {
            "model": model,
            "message": user_message,
            "preamble": system,
            "chat_history": chat_history,
            "max_tokens": max_tokens,
        }
        if cohere_tools:
            kwargs["tools"] = cohere_tools

        resp = client.chat(**kwargs)

    except cohere.core.api_error.ApiError as e:
        if "401" in str(e) or "unauthorized" in str(e).lower():
            raise ValueError(
                "Cohere API key is invalid. Check your COHERE_API_KEY."
            ) from e
        if "429" in str(e) or "rate" in str(e).lower():
            raise ConnectionError(
                "Cohere API rate limit exceeded. Wait a moment and try again."
            ) from e
        raise ConnectionError(f"Cohere API error: {e}") from e

    # Extract text content
    text_content = resp.text or ""

    # Extract tool calls (Cohere calls them "tool_calls" on finish reason TOOL_CALL)
    tool_calls = []
    if resp.tool_calls:
        for tc in resp.tool_calls:
            tool_calls.append({
                "name": tc.name,
                "input": tc.parameters or {},
                "id": f"cohere_{tc.name}_{len(tool_calls)}",
            })

    # Token usage
    input_tokens = 0
    output_tokens = 0
    if resp.meta and resp.meta.tokens:
        input_tokens = resp.meta.tokens.input_tokens or 0
        output_tokens = resp.meta.tokens.output_tokens or 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

"""
2M Code — Anthropic Provider Adapter

Adapts the Anthropic SDK (Claude models) to the unified 2M Code response format.
Use list_models() to fetch the current live model catalog from the Anthropic API.
"""

import logging
import os

logger = logging.getLogger("2mcode.providers.anthropic")

# API key is read from environment — never hardcoded
_client = None


def _get_client():
    """
    Lazily initialize the Anthropic client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    import anthropic
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        raise ValueError(
            "ANTHROPIC_API_KEY environment variable is not set. "
            "Set it with: export ANTHROPIC_API_KEY='your-key-here'"
        )

    import anthropic
    _client = anthropic.Anthropic(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available Anthropic models from the live API.

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        client = _get_client()
        resp = client.models.list()
        models = []
        for m in resp.data:
            models.append({
                "id": m.id,
                "name": m.display_name if hasattr(m, "display_name") else m.id,
                "description": "",
                "context_length": 0,
            })
        return models
    except Exception as e:
        logger.warning("Could not fetch Anthropic models from API: %s — using defaults", e)
        return [
            {"id": "claude-opus-4-5", "name": "Claude Opus 4.5", "description": "Most capable Claude model", "context_length": 200000},
            {"id": "claude-sonnet-4-6", "name": "Claude Sonnet 4.6", "description": "Balanced Claude model", "context_length": 200000},
            {"id": "claude-haiku-4-5", "name": "Claude Haiku 4.5", "description": "Fastest Claude model", "context_length": 200000},
        ]


def _convert_tools(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to Anthropic tool format."""
    anthropic_tools = []
    for tool in tools:
        anthropic_tools.append({
            "name": tool["name"],
            "description": tool["description"],
            "input_schema": tool["input_schema"],
        })
    return anthropic_tools


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the Anthropic API and return a normalized response.

    Args:
        model: Anthropic model ID (e.g., "claude-opus-4-5").
        system: System prompt for the agent's identity.
        messages: Conversation history as OpenAI-compatible message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Build the API request kwargs
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "system": system,
        "messages": messages,
    }

    # Only include tools if we have them
    anthropic_tools = _convert_tools(tools) if tools else []
    if anthropic_tools:
        kwargs["tools"] = anthropic_tools

    logger.info("Calling Anthropic API: model=%s max_tokens=%d tools=%d", model, max_tokens, len(anthropic_tools))

    import anthropic
    try:
        resp = client.messages.create(**kwargs)
    except anthropic.AuthenticationError as e:
        raise ValueError(
            "Anthropic API key is invalid. Check your ANTHROPIC_API_KEY."
        ) from e
    except anthropic.RateLimitError as e:
        raise ConnectionError(
            "Anthropic API rate limit exceeded. Wait a moment and try again."
        ) from e
    except anthropic.APIConnectionError as e:
        raise ConnectionError(
            "Cannot connect to Anthropic API. Check your network connection."
        ) from e

    # Extract text content
    text_content = next(
        (block.text for block in resp.content if block.type == "text"),
        "",
    )

    # Extract tool calls
    tool_calls = [
        {"name": block.name, "input": block.input, "id": block.id}
        for block in resp.content
        if block.type == "tool_use"
    ]

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": resp.usage.input_tokens,
        "output_tokens": resp.usage.output_tokens,
    }

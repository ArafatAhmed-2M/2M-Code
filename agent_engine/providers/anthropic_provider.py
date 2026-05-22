"""
2M Code — Anthropic Provider Adapter

Adapts the Anthropic SDK (Claude models) to the unified 2M Code response format.
Supports: claude-opus-4-5, claude-sonnet-4-6, claude-haiku-4-5
"""

import logging
import os

import anthropic

logger = logging.getLogger("2mcode.providers.anthropic")

# API key is read from environment — never hardcoded
_api_key = os.environ.get("ANTHROPIC_API_KEY")
_client = None


def _get_client() -> anthropic.Anthropic:
    """
    Lazily initialize the Anthropic client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        raise ValueError(
            "ANTHROPIC_API_KEY environment variable is not set. "
            "Set it with: export ANTHROPIC_API_KEY='your-key-here'"
        )

    _client = anthropic.Anthropic(api_key=api_key)
    return _client


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

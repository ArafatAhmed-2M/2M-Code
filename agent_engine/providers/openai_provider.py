"""
2M Code — OpenAI Provider Adapter

Adapts the OpenAI SDK (GPT models) to the unified 2M Code response format.
Use list_models() to fetch the current live model catalog from the OpenAI API.
"""

import json
import logging
import os

import openai

logger = logging.getLogger("2mcode.providers.openai")

_client = None


def _get_client() -> openai.OpenAI:
    """
    Lazily initialize the OpenAI client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        raise ValueError(
            "OPENAI_API_KEY environment variable is not set. "
            "Set it with: export OPENAI_API_KEY='your-key-here'"
        )

    _client = openai.OpenAI(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available OpenAI models from the live API.
    Filters to only chat-capable models (gpt-*, o1-*, o3-*).

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        client = _get_client()
        resp = client.models.list()
        # Only include chat models, sorted alphabetically
        chat_prefixes = ("gpt-", "o1", "o3", "chatgpt")
        models = [
            {
                "id": m.id,
                "name": m.id,
                "description": "",
                "context_length": 0,
            }
            for m in sorted(resp.data, key=lambda x: x.id)
            if any(m.id.startswith(p) for p in chat_prefixes)
        ]
        return models
    except Exception as e:
        logger.warning("Could not fetch OpenAI models from API: %s — using defaults", e)
        return [
            {"id": "gpt-4o", "name": "GPT-4o", "description": "Most capable GPT model", "context_length": 128000},
            {"id": "gpt-4o-mini", "name": "GPT-4o mini", "description": "Fast and affordable GPT model", "context_length": 128000},
            {"id": "o1-preview", "name": "o1 Preview", "description": "Reasoning model", "context_length": 128000},
            {"id": "o1-mini", "name": "o1 Mini", "description": "Fast reasoning model", "context_length": 128000},
        ]


def _convert_tools_to_openai(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to OpenAI function calling format."""
    if not tools:
        return []

    openai_tools = []
    for tool in tools:
        openai_tools.append({
            "type": "function",
            "function": {
                "name": tool["name"],
                "description": tool["description"],
                "parameters": tool["input_schema"],
            },
        })
    return openai_tools


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the OpenAI API and return a normalized response.

    Args:
        model: OpenAI model ID (e.g., "gpt-4o").
        system: System prompt for the agent's identity.
        messages: Conversation history as message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Build messages with system prompt prepended
    openai_messages = [{"role": "system", "content": system}]
    openai_messages.extend(messages)

    # Build the API request kwargs
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": openai_messages,
    }

    # Add tools if present
    openai_tools = _convert_tools_to_openai(tools)
    if openai_tools:
        kwargs["tools"] = openai_tools

    logger.info("Calling OpenAI API: model=%s max_tokens=%d tools=%d", model, max_tokens, len(openai_tools))

    try:
        resp = client.chat.completions.create(**kwargs)
    except openai.AuthenticationError as e:
        raise ValueError(
            "OpenAI API key is invalid. Check your OPENAI_API_KEY."
        ) from e
    except openai.RateLimitError as e:
        raise ConnectionError(
            "OpenAI API rate limit exceeded. Wait a moment and try again."
        ) from e
    except openai.APIConnectionError as e:
        raise ConnectionError(
            "Cannot connect to OpenAI API. Check your network connection."
        ) from e

    # Extract the first choice
    choice = resp.choices[0] if resp.choices else None
    if not choice:
        return {
            "content": "",
            "tool_calls": [],
            "input_tokens": 0,
            "output_tokens": 0,
        }

    # Extract text content
    text_content = choice.message.content or ""

    # Extract tool calls
    tool_calls = []
    if choice.message.tool_calls:
        for tc in choice.message.tool_calls:
            tool_calls.append({
                "name": tc.function.name,
                "input": json.loads(tc.function.arguments),
                "id": tc.id,
            })

    # Extract token usage
    input_tokens = resp.usage.prompt_tokens if resp.usage else 0
    output_tokens = resp.usage.completion_tokens if resp.usage else 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

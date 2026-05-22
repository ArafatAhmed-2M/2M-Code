"""
2M Code — Mistral Provider Adapter

Adapts the Mistral SDK to the unified 2M Code response format.
Use list_models() to fetch the current live model catalog from the Mistral API.
"""

import json
import logging
import os

logger = logging.getLogger("2mcode.providers.mistral")

_client = None


def _get_client():
    """
    Lazily initialize the Mistral client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    try:
        from mistralai.client import Mistral
    except ImportError as e:
        raise RuntimeError(
            "The mistralai package is outdated or not installed correctly. "
            "Please run: pip install mistralai>=1.0.0 --upgrade"
        ) from e

    api_key = os.environ.get("MISTRAL_API_KEY")
    if not api_key:
        raise ValueError(
            "MISTRAL_API_KEY environment variable is not set. "
            "Set it with: export MISTRAL_API_KEY='your-key-here'"
        )

    _client = Mistral(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available Mistral models from the live API.

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        client = _get_client()
        resp = client.models.list()
        models = []
        for m in (resp.data or []):
            models.append({
                "id": m.id,
                "name": m.id,
                "description": getattr(m, "description", ""),
                "context_length": 0,
            })
        models.sort(key=lambda x: x["id"])
        return models
    except Exception as e:
        logger.warning("Could not fetch Mistral models from API: %s — using defaults", e)
        return [
            {"id": "mistral-large-latest", "name": "Mistral Large", "description": "Most capable Mistral model", "context_length": 131072},
            {"id": "mistral-medium-latest", "name": "Mistral Medium", "description": "Balanced Mistral model", "context_length": 32000},
            {"id": "mistral-small-latest", "name": "Mistral Small", "description": "Fast Mistral model", "context_length": 32000},
            {"id": "codestral-latest", "name": "Codestral", "description": "Code-specialized Mistral model", "context_length": 32000},
            {"id": "open-mixtral-8x22b", "name": "Mixtral 8x22B", "description": "Open-source mixture of experts", "context_length": 65536},
        ]


def _convert_tools_to_mistral(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to Mistral function calling format."""
    if not tools:
        return []

    mistral_tools = []
    for tool in tools:
        mistral_tools.append({
            "type": "function",
            "function": {
                "name": tool["name"],
                "description": tool["description"],
                "parameters": tool["input_schema"],
            },
        })
    return mistral_tools


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the Mistral API and return a normalized response.

    Args:
        model: Mistral model ID (e.g., "mistral-large").
        system: System prompt for the agent's identity.
        messages: Conversation history as message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Build messages with system prompt prepended
    mistral_messages = [{"role": "system", "content": system}]
    mistral_messages.extend(messages)

    # Build the API request kwargs
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": mistral_messages,
    }

    # Add tools if present
    mistral_tools = _convert_tools_to_mistral(tools)
    if mistral_tools:
        kwargs["tools"] = mistral_tools

    logger.info("Calling Mistral API: model=%s max_tokens=%d tools=%d", model, max_tokens, len(mistral_tools))

    try:
        resp = client.chat.complete(**kwargs)
    except Exception as e:
        error_msg = str(e).lower()
        if "authentication" in error_msg or "unauthorized" in error_msg or "api_key" in error_msg:
            raise ValueError(
                "Mistral API key is invalid. Check your MISTRAL_API_KEY."
            ) from e
        if "rate" in error_msg or "limit" in error_msg:
            raise ConnectionError(
                "Mistral API rate limit exceeded. Wait a moment and try again."
            ) from e
        raise ConnectionError(
            f"Mistral API error: {e}. Check your network and API key."
        ) from e

    # Extract the first choice
    if not resp or not resp.choices:
        return {
            "content": "",
            "tool_calls": [],
            "input_tokens": 0,
            "output_tokens": 0,
        }

    choice = resp.choices[0]

    # Extract text content
    text_content = choice.message.content or ""

    # Extract tool calls
    tool_calls = []
    if choice.message.tool_calls:
        for tc in choice.message.tool_calls:
            arguments = tc.function.arguments
            if isinstance(arguments, str):
                arguments = json.loads(arguments)
            tool_calls.append({
                "name": tc.function.name,
                "input": arguments,
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

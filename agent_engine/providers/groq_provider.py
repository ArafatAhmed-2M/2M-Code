"""
2M Code — Groq Provider Adapter

Adapts the Groq SDK (ultra-fast inference) to the unified 2M Code response format.
Groq uses an OpenAI-compatible API so the conversion is straightforward.
Supports Llama 3, Mixtral, Gemma models with the fastest token generation available.

API Key env var: GROQ_API_KEY
Get a free key at: https://console.groq.com
"""

import logging
import os

from groq import Groq, AuthenticationError, RateLimitError, APIConnectionError

logger = logging.getLogger("2mcode.providers.groq")

_client = None


def _get_client() -> Groq:
    """
    Lazily initialize the Groq client.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("GROQ_API_KEY")
    if not api_key:
        raise ValueError(
            "GROQ_API_KEY environment variable is not set. "
            "Set it with: export GROQ_API_KEY='your-key-here'\n"
            "Get a free key at: https://console.groq.com"
        )

    _client = Groq(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available Groq models from the live API.

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
                "name": m.id,
                "description": f"Context: {getattr(m, 'context_window', 'unknown')} tokens",
                "context_length": getattr(m, "context_window", 0),
            })
        # Sort by name for readability
        models.sort(key=lambda x: x["id"])
        return models
    except Exception as e:
        logger.warning("Could not fetch Groq models from API: %s — using defaults", e)
        return [
            {"id": "llama3-70b-8192", "name": "llama3-70b-8192", "description": "Llama 3 70B (8K context)", "context_length": 8192},
            {"id": "llama3-8b-8192", "name": "llama3-8b-8192", "description": "Llama 3 8B (8K context) — fastest", "context_length": 8192},
            {"id": "mixtral-8x7b-32768", "name": "mixtral-8x7b-32768", "description": "Mixtral 8x7B (32K context)", "context_length": 32768},
            {"id": "gemma2-9b-it", "name": "gemma2-9b-it", "description": "Gemma 2 9B Instruction Tuned", "context_length": 8192},
            {"id": "llama-3.1-70b-versatile", "name": "llama-3.1-70b-versatile", "description": "Llama 3.1 70B Versatile (128K)", "context_length": 131072},
            {"id": "llama-3.1-8b-instant", "name": "llama-3.1-8b-instant", "description": "Llama 3.1 8B Instant (128K) — fastest", "context_length": 131072},
        ]


def _convert_tools(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to OpenAI-compatible format (Groq uses same schema)."""
    return [
        {
            "type": "function",
            "function": {
                "name": tool["name"],
                "description": tool["description"],
                "parameters": tool["input_schema"],
            },
        }
        for tool in tools
    ]


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the Groq API and return a normalized response.

    Groq provides OpenAI-compatible inference at extremely high speed
    (typically 300-700 tokens/sec vs 50-100 for standard APIs).

    Args:
        model: Groq model ID (e.g., "llama3-70b-8192").
        system: System prompt for the agent's identity.
        messages: Conversation history as OpenAI-compatible message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Build message list with system prompt
    groq_messages = [{"role": "system", "content": system}] + messages

    groq_tools = _convert_tools(tools) if tools else []

    logger.info("Calling Groq API: model=%s max_tokens=%d tools=%d",
                model, max_tokens, len(groq_tools))

    try:
        kwargs = {
            "model": model,
            "messages": groq_messages,
            "max_tokens": max_tokens,
        }
        if groq_tools:
            kwargs["tools"] = groq_tools
            kwargs["tool_choice"] = "auto"

        resp = client.chat.completions.create(**kwargs)

    except AuthenticationError as e:
        raise ValueError(
            "Groq API key is invalid. Check your GROQ_API_KEY."
        ) from e
    except RateLimitError as e:
        raise ConnectionError(
            "Groq API rate limit exceeded. Wait a moment and try again."
        ) from e
    except APIConnectionError as e:
        raise ConnectionError(
            "Cannot connect to Groq API. Check your network connection."
        ) from e

    choice = resp.choices[0]
    message = choice.message

    # Extract text content
    text_content = message.content or ""

    # Extract tool calls
    tool_calls = []
    if message.tool_calls:
        for tc in message.tool_calls:
            import json
            try:
                args = json.loads(tc.function.arguments)
            except (json.JSONDecodeError, TypeError):
                args = {}
            tool_calls.append({
                "name": tc.function.name,
                "input": args,
                "id": tc.id,
            })

    # Token usage
    input_tokens = resp.usage.prompt_tokens if resp.usage else 0
    output_tokens = resp.usage.completion_tokens if resp.usage else 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

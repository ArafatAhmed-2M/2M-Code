"""
2M Code — OpenRouter Provider Adapter

Adapts OpenRouter (which acts as a unified router for 200+ models) to the unified 2M Code response format.
OpenRouter uses an OpenAI-compatible API. We use the standard openai SDK pointed at their base_url.

API Key env var: OPENROUTER_API_KEY
Get a key at: https://openrouter.ai/keys
"""

import json
import logging
import os

import openai

logger = logging.getLogger("2mcode.providers.openrouter")

_client = None


def _get_client() -> openai.OpenAI:
    """
    Lazily initialize the OpenRouter client using the OpenAI SDK.
    Raises ValueError if the API key is not set.
    """
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("OPENROUTER_API_KEY")
    if not api_key:
        raise ValueError(
            "OPENROUTER_API_KEY environment variable is not set. "
            "Set it with: export OPENROUTER_API_KEY='your-key-here'\n"
            "Get a key at: https://openrouter.ai/keys"
        )

    # OpenRouter recommends sending these headers for rankings
    headers = {
        "HTTP-Referer": "https://github.com/2mcode/2mcode",
        "X-Title": "2M Code",
    }

    _client = openai.OpenAI(
        base_url="https://openrouter.ai/api/v1",
        api_key=api_key,
        default_headers=headers,
    )
    return _client


def list_models() -> list[dict]:
    """
    Fetch the list of available models from OpenRouter's live API.

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        client = _get_client()
        resp = client.models.list()
        
        models = [
            {
                "id": m.id,
                "name": m.name or m.id,
                "description": getattr(m, "description", ""),
                "context_length": getattr(m, "context_length", getattr(m, "top_p", 0)), # some endpoints put context len in different places
            }
            for m in sorted(resp.data, key=lambda x: x.id)
        ]
        return models
    except Exception as e:
        logger.warning("Could not fetch OpenRouter models from API: %s — using defaults", e)
        return [
            {"id": "anthropic/claude-3.5-sonnet", "name": "Claude 3.5 Sonnet", "description": "Most intelligent Claude", "context_length": 200000},
            {"id": "meta-llama/llama-3.1-70b-instruct", "name": "Llama 3.1 70B", "description": "Powerful open source", "context_length": 131072},
            {"id": "google/gemini-pro-1.5", "name": "Gemini 1.5 Pro", "description": "Massive context", "context_length": 2000000},
            {"id": "openai/gpt-4o", "name": "GPT-4o", "description": "OpenAI flagship", "context_length": 128000},
        ]


def _convert_tools_to_openrouter(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to OpenAI/OpenRouter function calling format."""
    if not tools:
        return []

    openrouter_tools = []
    for tool in tools:
        openrouter_tools.append({
            "type": "function",
            "function": {
                "name": tool["name"],
                "description": tool["description"],
                "parameters": tool["input_schema"],
            },
        })
    return openrouter_tools


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the OpenRouter API and return a normalized response.

    Args:
        model: OpenRouter model ID (e.g., "anthropic/claude-3.5-sonnet").
        system: System prompt for the agent's identity.
        messages: Conversation history as message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    client = _get_client()

    # Build messages with system prompt prepended
    openrouter_messages = [{"role": "system", "content": system}]
    openrouter_messages.extend(messages)

    # Build the API request kwargs
    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": openrouter_messages,
    }

    # Add tools if present
    openrouter_tools = _convert_tools_to_openrouter(tools)
    if openrouter_tools:
        kwargs["tools"] = openrouter_tools

    logger.info("Calling OpenRouter API: model=%s max_tokens=%d tools=%d", model, max_tokens, len(openrouter_tools))

    try:
        resp = client.chat.completions.create(**kwargs)
    except openai.AuthenticationError as e:
        raise ValueError(
            "OpenRouter API key is invalid. Check your OPENROUTER_API_KEY."
        ) from e
    except openai.RateLimitError as e:
        raise ConnectionError(
            "OpenRouter API rate limit exceeded or insufficient credits. Wait a moment and try again."
        ) from e
    except openai.APIConnectionError as e:
        raise ConnectionError(
            "Cannot connect to OpenRouter API. Check your network connection."
        ) from e
    except openai.APIError as e:
        # OpenRouter sometimes returns 502/downstream errors if the chosen model is offline
        raise ConnectionError(
            f"OpenRouter upstream provider error: {e}. The selected model might be offline."
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
            # Some models via OpenRouter might return strings, but standard is JSON
            try:
                args = json.loads(tc.function.arguments)
            except (json.JSONDecodeError, TypeError):
                args = {}
                
            tool_calls.append({
                "name": tc.function.name,
                "input": args,
                "id": tc.id,
            })

    # Extract token usage (OpenRouter usually passes this through)
    input_tokens = resp.usage.prompt_tokens if resp.usage else 0
    output_tokens = resp.usage.completion_tokens if resp.usage else 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

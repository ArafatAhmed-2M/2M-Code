"""
2M Code — Agent Router

Routes incoming agent requests to the correct provider adapter.
All providers return the same normalized response shape:
{content, tool_calls, input_tokens, output_tokens}

Also exposes list_all_models() which queries every configured provider
for its live model catalog and returns a unified list.
"""

import asyncio
import logging
import os

from providers import (
    anthropic_provider,
    google_provider,
    openai_provider,
    mistral_provider,
    cohere_provider,
    groq_provider,
    ollama_provider,
    openrouter_provider,
)
from tools import get_tool_definitions

logger = logging.getLogger("2mcode.agent")

# Provider registry — maps provider name to its module
PROVIDERS = {
    "anthropic":  anthropic_provider,
    "google":     google_provider,
    "openai":     openai_provider,
    "mistral":    mistral_provider,
    "cohere":     cohere_provider,
    "groq":       groq_provider,
    "ollama":     ollama_provider,
    "openrouter": openrouter_provider,
}

# Maps each provider to its required env var
_PROVIDER_ENV_VARS = {
    "anthropic":  "ANTHROPIC_API_KEY",
    "google":     "GOOGLE_API_KEY",
    "openai":     "OPENAI_API_KEY",
    "mistral":    "MISTRAL_API_KEY",
    "cohere":     "COHERE_API_KEY",
    "groq":       "GROQ_API_KEY",
    "openrouter": "OPENROUTER_API_KEY",
}


def _resolve_provider(provider_name: str):
    """
    Return the provider module for the given name.
    If the provider's API key is missing but OPENROUTER_API_KEY is set,
    fall back to the OpenRouter provider so the user only needs one key.
    """
    # Ollama runs locally — no key needed
    if provider_name == "ollama":
        return ollama_provider, provider_name

    env_var = _PROVIDER_ENV_VARS.get(provider_name)
    if env_var and not os.environ.get(env_var):
        # Provider key is missing — check if OpenRouter can be used instead
        if provider_name != "openrouter" and os.environ.get("OPENROUTER_API_KEY"):
            logger.warning(
                "%s not set — falling back to OpenRouter for provider '%s'",
                env_var, provider_name,
            )
            return openrouter_provider, provider_name
        raise ValueError(
            f"{env_var} environment variable is not set. "
            f"Set it with: export {env_var}='your-key-here'\n"
            f"Or set OPENROUTER_API_KEY to use OpenRouter as a universal provider."
        )

    return PROVIDERS[provider_name], provider_name


async def run_agent(req) -> dict:
    """
    Route an agent request to the correct provider and return the response.

    Args:
        req: AgentRequest with provider, model, system, messages, tools, max_tokens.

    Returns:
        dict with keys: content (str), tool_calls (list), input_tokens (int), output_tokens (int).

    Raises:
        KeyError: If the provider is not supported.
        ConnectionError: If the provider API is unreachable.
        ValueError: If the request parameters are invalid.
    """
    if req.provider not in PROVIDERS:
        supported = ", ".join(sorted(PROVIDERS.keys()))
        raise KeyError(
            f"Unknown provider: '{req.provider}'. "
            f"Supported providers: {supported}"
        )

    provider, actual_provider = _resolve_provider(req.provider)

    # Build tool definitions for the requested tools (built-in + custom)
    tools = get_tool_definitions(req.tools)
    # Merge custom tool definitions into the tool list
    for ct in req.custom_tools:
        tools.append(ct)

    # Convert message objects to dicts for the provider
    messages = [
        {"role": msg.role, "content": msg.content}
        for msg in req.messages
    ]

    logger.info(
        "Routing request: provider=%s model=%s tools=%s message_count=%d",
        actual_provider,
        req.model,
        req.tools,
        len(messages),
    )

    result = await provider.call(
        model=req.model,
        system=req.system,
        messages=messages,
        tools=tools,
        max_tokens=req.max_tokens,
    )

    logger.info(
        "Response received: provider=%s input_tokens=%d output_tokens=%d tool_calls=%d",
        req.provider,
        result.get("input_tokens", 0),
        result.get("output_tokens", 0),
        len(result.get("tool_calls", [])),
    )

    return result


async def run_agent_stream(req):
    """
    Stream an agent response, yielding (event_type, data) tuples.

    Event types:
      - "text": data is a text chunk
      - "tool_call": data is {name, input, id}
      - "done": data is {input_tokens, output_tokens}

    Falls back to non-streaming if the provider doesn't support streaming.
    """
    if req.provider not in PROVIDERS:
        supported = ", ".join(sorted(PROVIDERS.keys()))
        raise KeyError(f"Unknown provider: '{req.provider}'. Supported providers: {supported}")

    provider, actual_provider = _resolve_provider(req.provider)

    # Check if provider has streaming support
    call_stream_fn = getattr(provider, "call_stream", None)
    has_streaming = getattr(provider, "has_streaming", False)

    # Build tool definitions
    tools = get_tool_definitions(req.tools)
    for ct in req.custom_tools:
        tools.append(ct)

    messages = [
        {"role": msg.role, "content": msg.content}
        for msg in req.messages
    ]

    logger.info(
        "Streaming request: provider=%s model=%s tools=%s message_count=%d",
        actual_provider, req.model, req.tools, len(messages),
    )

    if has_streaming and call_stream_fn:
        async for event_type, data in call_stream_fn(
            model=req.model,
            system=req.system,
            messages=messages,
            tools=tools,
            max_tokens=req.max_tokens,
        ):
            yield (event_type, data)
    else:
        # Fallback: non-streaming, yield entire response as one chunk
        result = await provider.call(
            model=req.model,
            system=req.system,
            messages=messages,
            tools=tools,
            max_tokens=req.max_tokens,
        )
        yield ("text", result.get("content", ""))
        for tc in result.get("tool_calls", []):
            yield ("tool_call", tc)
        yield ("done", {
            "input_tokens": result.get("input_tokens", 0),
            "output_tokens": result.get("output_tokens", 0),
        })


async def list_all_models(providers_filter: list[str] | None = None) -> dict:
    """
    Fetch available models from all (or a subset of) providers concurrently.

    Each provider's list_models() is called in parallel. Failures for individual
    providers are caught and reported as empty lists — so one bad API key doesn't
    block the entire listing.

    Args:
        providers_filter: If set, only query these providers. Otherwise query all.

    Returns:
        Dict mapping provider name to list of model dicts:
        {
          "anthropic": [{id, name, description, context_length}, ...],
          "google":    [...],
          ...
        }
    """
    target_providers = providers_filter or list(PROVIDERS.keys())

    async def fetch_for_provider(provider_name: str) -> tuple[str, list]:
        """Fetch models for a single provider, catching all errors."""
        provider = PROVIDERS.get(provider_name)
        if provider is None:
            return provider_name, []

        try:
            list_fn = getattr(provider, "list_models", None)
            if list_fn is None:
                return provider_name, []

            # Handle both async and sync list_models()
            if asyncio.iscoroutinefunction(list_fn):
                models = await list_fn()
            else:
                # Run sync function in thread pool to avoid blocking event loop
                loop = asyncio.get_event_loop()
                models = await loop.run_in_executor(None, list_fn)

            return provider_name, models
        except Exception as e:
            logger.warning("Could not list models for provider '%s': %s", provider_name, e)
            return provider_name, []

    # Fetch all providers concurrently
    tasks = [fetch_for_provider(p) for p in target_providers]
    results = await asyncio.gather(*tasks)

    return dict(results)

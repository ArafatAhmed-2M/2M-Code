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

    provider = PROVIDERS[req.provider]

    # Build tool definitions for the requested tools
    tools = get_tool_definitions(req.tools)

    # Convert message objects to dicts for the provider
    messages = [
        {"role": msg.role, "content": msg.content}
        for msg in req.messages
    ]

    logger.info(
        "Routing request: provider=%s model=%s tools=%s message_count=%d",
        req.provider,
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

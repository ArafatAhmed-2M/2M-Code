"""
2M Code — Agent Router

Routes incoming agent requests to the correct provider adapter.
All providers return the same normalized response shape:
{content, tool_calls, input_tokens, output_tokens}
"""

import logging

from providers import anthropic_provider, google_provider, openai_provider, mistral_provider
from tools import get_tool_definitions

logger = logging.getLogger("2mcode.agent")

# Provider registry — maps provider name to its module
PROVIDERS = {
    "anthropic": anthropic_provider,
    "google": google_provider,
    "openai": openai_provider,
    "mistral": mistral_provider,
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
        raise KeyError(f"Unknown provider: {req.provider}")

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

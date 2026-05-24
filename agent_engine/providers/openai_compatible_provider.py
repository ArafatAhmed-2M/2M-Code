"""
2M Code — OpenAI-Compatible Provider Adapter

Generic adapter for any OpenAI-compatible API. Supports DeepSeek, Together AI,
xAI Grok, Perplexity, Fireworks, GitHub Models, and any other provider that
exposes an OpenAI-compatible chat completions endpoint.

Usage (set in team YAML):
  provider: openai_compatible

Environment variables:
  OPENAI_COMPATIBLE_API_KEY  — API key for the provider
  OPENAI_COMPATIBLE_BASE_URL — Base URL (default: https://api.openai.com/v1)

Examples:
  DeepSeek:    OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com
  Together:    OPENAI_COMPATIBLE_BASE_URL=https://api.together.xyz/v1
  xAI Grok:   OPENAI_COMPATIBLE_BASE_URL=https://api.x.ai/v1
  Perplexity: OPENAI_COMPATIBLE_BASE_URL=https://api.perplexity.ai
  Fireworks:  OPENAI_COMPATIBLE_BASE_URL=https://api.fireworks.ai/inference/v1
  GitHub:     OPENAI_COMPATIBLE_BASE_URL=https://models.inference.ai.azure.com
"""

import json
import logging
import os

from openai import OpenAI, AuthenticationError, RateLimitError, APIConnectionError

logger = logging.getLogger("2mcode.providers.openai_compatible")

_client = None


def _get_client() -> OpenAI:
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("OPENAI_COMPATIBLE_API_KEY")
    if not api_key:
        raise ValueError(
            "OPENAI_COMPATIBLE_API_KEY environment variable is not set. "
            "Set it with: export OPENAI_COMPATIBLE_API_KEY='your-key-here'"
        )

    base_url = os.environ.get("OPENAI_COMPATIBLE_BASE_URL", "https://api.openai.com/v1")
    base_url = base_url.rstrip("/")

    _client = OpenAI(api_key=api_key, base_url=base_url)
    return _client


def list_models() -> list[dict]:
    try:
        client = _get_client()
        resp = client.models.list()
        models = [
            {
                "id": m.id,
                "name": m.id,
                "description": "",
                "context_length": 0,
            }
            for m in sorted(resp.data, key=lambda x: x.id)
        ]
        return models
    except Exception as e:
        logger.warning("Could not fetch models from API: %s", e)
        return []


def _convert_tools(tools: list[dict]) -> list[dict]:
    if not tools:
        return []
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


has_streaming = True


async def call_stream(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
):
    client = _get_client()

    openai_messages = [{"role": "system", "content": system}]
    openai_messages.extend(messages)

    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": openai_messages,
        "stream": True,
    }

    openai_tools = _convert_tools(tools)
    if openai_tools:
        kwargs["tools"] = openai_tools
        kwargs["tool_choice"] = "auto"

    collected_tool_calls = {}
    try:
        stream = client.chat.completions.create(**kwargs)
        for chunk in stream:
            choice = chunk.choices[0] if chunk.choices else None
            if not choice:
                continue

            delta = choice.delta

            if delta.content:
                yield ("text", delta.content)

            if delta.tool_calls:
                for tc in delta.tool_calls:
                    idx = tc.index
                    if idx not in collected_tool_calls:
                        collected_tool_calls[idx] = {"name": "", "arguments": "", "id": ""}
                    if tc.id:
                        collected_tool_calls[idx]["id"] = tc.id
                    if tc.function and tc.function.name:
                        collected_tool_calls[idx]["name"] = tc.function.name
                    if tc.function and tc.function.arguments:
                        collected_tool_calls[idx]["arguments"] += tc.function.arguments

            if chunk.usage:
                yield ("done", {
                    "input_tokens": chunk.usage.prompt_tokens or 0,
                    "output_tokens": chunk.usage.completion_tokens or 0,
                })

        for tc in collected_tool_calls.values():
            if tc["name"]:
                try:
                    args = json.loads(tc["arguments"]) if tc["arguments"] else {}
                except json.JSONDecodeError:
                    args = {}
                yield ("tool_call", {
                    "name": tc["name"],
                    "input": args,
                    "id": tc["id"] or f"oci_{tc['name']}_0",
                })

    except AuthenticationError as e:
        raise ValueError("API key is invalid. Check OPENAI_COMPATIBLE_API_KEY.") from e
    except RateLimitError as e:
        raise ConnectionError("API rate limit exceeded. Wait and try again.") from e
    except APIConnectionError as e:
        raise ConnectionError("Cannot connect to API. Check OPENAI_COMPATIBLE_BASE_URL and network.") from e


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    client = _get_client()

    openai_messages = [{"role": "system", "content": system}]
    openai_messages.extend(messages)

    kwargs = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": openai_messages,
    }

    openai_tools = _convert_tools(tools)
    if openai_tools:
        kwargs["tools"] = openai_tools

    logger.info("Calling OpenAI-compatible API: model=%s max_tokens=%d base_url=%s",
                model, max_tokens, client.base_url)

    try:
        resp = client.chat.completions.create(**kwargs)
    except AuthenticationError as e:
        raise ValueError("API key is invalid. Check OPENAI_COMPATIBLE_API_KEY.") from e
    except RateLimitError as e:
        raise ConnectionError("API rate limit exceeded. Wait and try again.") from e
    except APIConnectionError as e:
        raise ConnectionError("Cannot connect to API. Check OPENAI_COMPATIBLE_BASE_URL and network.") from e

    choice = resp.choices[0] if resp.choices else None
    if not choice:
        return {"content": "", "tool_calls": [], "input_tokens": 0, "output_tokens": 0}

    text_content = choice.message.content or ""

    tool_calls = []
    if choice.message.tool_calls:
        for tc in choice.message.tool_calls:
            try:
                args = json.loads(tc.function.arguments)
            except (json.JSONDecodeError, TypeError):
                args = {}
            tool_calls.append({
                "name": tc.function.name,
                "input": args,
                "id": tc.id,
            })

    input_tokens = resp.usage.prompt_tokens if resp.usage else 0
    output_tokens = resp.usage.completion_tokens if resp.usage else 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

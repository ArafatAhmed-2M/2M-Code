"""
2M Code — Google Gemini Provider Adapter

Adapts the Google Gen AI SDK (Gemini models) to the unified 2M Code response format.
Use list_models() to fetch the current live model catalog from the Google API.
"""

import logging
import os

from google import genai
from google.genai import types

logger = logging.getLogger("2mcode.providers.google")

_client = None


def _get_client():
    global _client
    if _client is not None:
        return _client

    api_key = os.environ.get("GOOGLE_API_KEY")
    if not api_key:
        raise ValueError(
            "GOOGLE_API_KEY environment variable is not set. "
            "Set it with: export GOOGLE_API_KEY='your-key-here'"
        )

    _client = genai.Client(api_key=api_key)
    return _client


def list_models() -> list[dict]:
    try:
        client = _get_client()
        models = []
        for m in client.models.list():
            if "generateContent" not in (m.supported_actions or []):
                continue
            model_id = m.name.replace("models/", "")
            models.append({
                "id": model_id,
                "name": model_id,
                "description": m.description or "",
                "context_length": getattr(m, "input_token_limit", 0) or 0,
            })
        models.sort(key=lambda x: x["id"])
        return models
    except Exception as e:
        logger.warning("Could not fetch Google models from API: %s \u2014 using defaults", e)
        return [
            {"id": "gemini-1.5-pro", "name": "Gemini 1.5 Pro", "description": "Most capable Gemini model", "context_length": 1000000},
            {"id": "gemini-1.5-flash", "name": "Gemini 1.5 Flash", "description": "Fast Gemini model", "context_length": 1000000},
            {"id": "gemini-2.0-flash", "name": "Gemini 2.0 Flash", "description": "Latest fast Gemini model", "context_length": 1000000},
            {"id": "gemini-2.0-flash-lite", "name": "Gemini 2.0 Flash Lite", "description": "Lightest Gemini model", "context_length": 1000000},
        ]


def _convert_tools_to_gemini(tools: list[dict]) -> list[types.Tool]:
    if not tools:
        return []

    function_declarations = []
    for tool in tools:
        properties = tool.get("input_schema", {}).get("properties", {})
        required = tool.get("input_schema", {}).get("required", [])

        parameters = {
            "type": "object",
            "properties": {},
            "required": required,
        }

        for prop_name, prop_def in properties.items():
            param_type = prop_def.get("type", "string").upper()
            parameters["properties"][prop_name] = {
                "type": param_type,
                "description": prop_def.get("description", ""),
            }

        function_declarations.append(
            types.FunctionDeclaration(
                name=tool["name"],
                description=tool["description"],
                parameters=parameters,
            )
        )

    return [types.Tool(function_declarations=function_declarations)]


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    client = _get_client()

    gemini_tools = _convert_tools_to_gemini(tools)

    history = []
    for msg in messages[:-1]:
        role = "user" if msg["role"] == "user" else "model"
        history.append(types.Content(
            role=role,
            parts=[types.Part.from_text(msg["content"])],
        ))

    last_message = messages[-1]["content"] if messages else "Hello"

    config_kwargs = {}
    if system:
        config_kwargs["system_instruction"] = system
    if gemini_tools:
        config_kwargs["tools"] = gemini_tools
    if max_tokens:
        config_kwargs["max_output_tokens"] = max_tokens

    generation_config = types.GenerateContentConfig(**config_kwargs) if config_kwargs else None

    logger.info("Calling Google Gemini API: model=%s max_tokens=%d", model, max_tokens)

    try:
        chat = client.chats.create(
            model=model,
            history=history,
            config=generation_config,
        )
        resp = chat.send_message(last_message)
    except Exception as e:
        error_msg = str(e).lower()
        if "api_key" in error_msg or "authentication" in error_msg:
            raise ValueError(
                "Google API key is invalid. Check your GOOGLE_API_KEY."
            ) from e
        if "quota" in error_msg or "rate" in error_msg:
            raise ConnectionError(
                "Google API rate limit exceeded. Wait a moment and try again."
            ) from e
        raise ConnectionError(
            f"Google Gemini API error: {e}. Check your network and API key."
        ) from e

    text_content = ""
    tool_calls = []

    if resp.candidates:
        for candidate in resp.candidates:
            if candidate.content and candidate.content.parts:
                for part in candidate.content.parts:
                    if part.text:
                        text_content += part.text
                    elif part.function_call:
                        fc = part.function_call
                        tool_calls.append({
                            "name": fc.name,
                            "input": dict(fc.args),
                            "id": f"gemini_{fc.name}_{len(tool_calls)}",
                        })

    input_tokens = 0
    output_tokens = 0
    if resp.usage_metadata:
        input_tokens = resp.usage_metadata.prompt_token_count or 0
        output_tokens = resp.usage_metadata.candidates_token_count or 0

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

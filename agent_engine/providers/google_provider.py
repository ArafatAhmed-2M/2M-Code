"""
2M Code — Google Gemini Provider Adapter

Adapts the Google Generative AI SDK (Gemini models) to the unified 2M Code response format.
Use list_models() to fetch the current live model catalog from the Google API.
"""

import logging
import os
import warnings

# Suppress the deprecation warning for google.generativeai so it doesn't spam the terminal
warnings.filterwarnings("ignore", category=FutureWarning, module="google.generativeai")

import google.generativeai as genai

logger = logging.getLogger("2mcode.providers.google")

_configured = False


def _ensure_configured():
    """
    Configure the Google Generative AI SDK with the API key.
    Raises ValueError if the API key is not set.
    """
    global _configured
    if _configured:
        return

    api_key = os.environ.get("GOOGLE_API_KEY")
    if not api_key:
        raise ValueError(
            "GOOGLE_API_KEY environment variable is not set. "
            "Set it with: export GOOGLE_API_KEY='your-key-here'"
        )

    genai.configure(api_key=api_key)
    _configured = True


def list_models() -> list[dict]:
    """
    Fetch the list of available Google Gemini models from the live API.
    Only returns models that support generateContent (chat-capable).

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Falls back to hardcoded defaults if the API call fails.
    """
    try:
        _ensure_configured()
        models = []
        for m in genai.list_models():
            # Only include models that can generate content
            if "generateContent" not in (m.supported_generation_methods or []):
                continue
            # Strip the "models/" prefix for cleaner IDs
            model_id = m.name.replace("models/", "")
            models.append({
                "id": model_id,
                "name": m.display_name if hasattr(m, "display_name") else model_id,
                "description": m.description if hasattr(m, "description") else "",
                "context_length": getattr(m, "input_token_limit", 0),
            })
        models.sort(key=lambda x: x["id"])
        return models
    except Exception as e:
        logger.warning("Could not fetch Google models from API: %s — using defaults", e)
        return [
            {"id": "gemini-1.5-pro", "name": "Gemini 1.5 Pro", "description": "Most capable Gemini model", "context_length": 1000000},
            {"id": "gemini-1.5-flash", "name": "Gemini 1.5 Flash", "description": "Fast Gemini model", "context_length": 1000000},
            {"id": "gemini-2.0-flash", "name": "Gemini 2.0 Flash", "description": "Latest fast Gemini model", "context_length": 1000000},
            {"id": "gemini-2.0-flash-lite", "name": "Gemini 2.0 Flash Lite", "description": "Lightest Gemini model", "context_length": 1000000},
        ]


def _convert_tools_to_gemini(tools: list[dict]) -> list:
    """
    Convert 2M Code tool definitions to Gemini function declarations.

    Gemini uses a different format for tool/function definitions.
    This converts from the OpenAI-like schema to Gemini's format.
    """
    if not tools:
        return []

    function_declarations = []
    for tool in tools:
        # Convert JSON Schema properties to Gemini parameter format
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
            genai.protos.FunctionDeclaration(
                name=tool["name"],
                description=tool["description"],
                parameters=parameters,
            )
        )

    return [genai.protos.Tool(function_declarations=function_declarations)]


def _build_gemini_messages(system: str, messages: list[dict]) -> tuple:
    """
    Convert OpenAI-style messages to Gemini's content format.

    Returns:
        (system_instruction, gemini_history, last_user_message)
    """
    gemini_history = []
    last_message = None

    for msg in messages:
        role = "user" if msg["role"] == "user" else "model"
        content = msg["content"]

        if msg == messages[-1]:
            last_message = content
        else:
            gemini_history.append({"role": role, "parts": [content]})

    return system, gemini_history, last_message


async def call(
    model: str,
    system: str,
    messages: list[dict],
    tools: list[dict],
    max_tokens: int,
) -> dict:
    """
    Call the Google Gemini API and return a normalized response.

    Args:
        model: Gemini model ID (e.g., "gemini-1.5-pro").
        system: System prompt for the agent's identity.
        messages: Conversation history as OpenAI-compatible message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    _ensure_configured()

    # Convert tools to Gemini format
    gemini_tools = _convert_tools_to_gemini(tools)

    # Build the model with system instruction
    generation_config = genai.types.GenerationConfig(
        max_output_tokens=max_tokens,
    )

    model_kwargs = {
        "model_name": model,
        "generation_config": generation_config,
        "system_instruction": system,
    }

    if gemini_tools:
        model_kwargs["tools"] = gemini_tools

    gemini_model = genai.GenerativeModel(**model_kwargs)

    # Build conversation history
    _, history, last_message = _build_gemini_messages(system, messages)

    logger.info("Calling Google Gemini API: model=%s max_tokens=%d", model, max_tokens)

    try:
        # Start a chat session with history
        chat = gemini_model.start_chat(history=history)
        resp = chat.send_message(last_message or "Hello")
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

    # Extract text content
    text_content = ""
    tool_calls = []

    for candidate in resp.candidates:
        for part in candidate.content.parts:
            if hasattr(part, "text") and part.text:
                text_content += part.text
            elif hasattr(part, "function_call") and part.function_call:
                fc = part.function_call
                tool_calls.append({
                    "name": fc.name,
                    "input": dict(fc.args),
                    "id": f"gemini_{fc.name}_{len(tool_calls)}",
                })

    # Extract token usage (Gemini provides this via usage_metadata)
    input_tokens = 0
    output_tokens = 0
    if hasattr(resp, "usage_metadata") and resp.usage_metadata:
        input_tokens = getattr(resp.usage_metadata, "prompt_token_count", 0)
        output_tokens = getattr(resp.usage_metadata, "candidates_token_count", 0)

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

"""
2M Code — Ollama Provider Adapter

Adapts the Ollama local inference server to the unified 2M Code response format.
Ollama runs open-source models (Llama, Mistral, Phi, Qwen, CodeLlama, etc.)
locally on your machine — completely free, fully private, no API key needed.

Requirements:
  - Ollama installed and running: https://ollama.com
  - Pull a model first: ollama pull llama3
  
Config env var: OLLAMA_HOST (default: http://localhost:11434)
"""

import json
import logging
import os

import httpx

logger = logging.getLogger("2mcode.providers.ollama")

# Default Ollama host — can be overridden for remote Ollama instances
OLLAMA_HOST = os.environ.get("OLLAMA_HOST", "http://localhost:11434")


def _get_host() -> str:
    """Return the Ollama host URL, checking env var each time."""
    return os.environ.get("OLLAMA_HOST", "http://localhost:11434")


async def list_models() -> list[dict]:
    """
    Fetch the list of locally available Ollama models.

    Queries the Ollama /api/tags endpoint to get all pulled models.

    Returns:
        List of dicts: [{id, name, description, context_length}]
        Returns empty list with warning if Ollama is not running.
    """
    host = _get_host()
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(f"{host}/api/tags")
            resp.raise_for_status()
            data = resp.json()

        models = []
        for m in data.get("models", []):
            size_bytes = m.get("size", 0)
            size_gb = f"{size_bytes / 1e9:.1f}GB" if size_bytes else "unknown size"
            models.append({
                "id": m["name"],
                "name": m["name"],
                "description": f"Local model — {size_gb} — {m.get('details', {}).get('parameter_size', 'unknown')} params",
                "context_length": 0,  # Ollama doesn't expose this via /api/tags
            })
        return models
    except httpx.ConnectError:
        logger.warning(
            "Ollama is not running at %s. "
            "Start it with: ollama serve (or install from https://ollama.com)", host
        )
        return []
    except Exception as e:
        logger.warning("Could not fetch Ollama models: %s", e)
        return []


def _convert_tools_to_ollama(tools: list[dict]) -> list[dict]:
    """Convert 2M Code tool definitions to Ollama tool format (OpenAI-compatible)."""
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
    Call a local Ollama model and return a normalized response.

    Uses Ollama's /api/chat endpoint (OpenAI-compatible).
    Models must be pulled first with: ollama pull <model>

    Args:
        model: Ollama model name (e.g., "llama3", "mistral", "codellama").
        system: System prompt for the agent's identity.
        messages: Conversation history as OpenAI-compatible message dicts.
        tools: Tool definitions in 2M Code format.
        max_tokens: Maximum tokens for the response.

    Returns:
        Normalized dict: {content, tool_calls, input_tokens, output_tokens}
    """
    host = _get_host()

    # Build message list with system prompt
    ollama_messages = [{"role": "system", "content": system}] + messages

    ollama_tools = _convert_tools_to_ollama(tools) if tools else []

    payload = {
        "model": model,
        "messages": ollama_messages,
        "stream": False,
        "options": {
            "num_predict": max_tokens,
        },
    }
    if ollama_tools:
        payload["tools"] = ollama_tools

    logger.info("Calling Ollama API: host=%s model=%s max_tokens=%d tools=%d",
                host, model, max_tokens, len(ollama_tools))

    try:
        # Use a generous timeout — local models can be slow
        async with httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(f"{host}/api/chat", json=payload)
            resp.raise_for_status()
    except httpx.ConnectError as e:
        raise ConnectionError(
            f"Cannot connect to Ollama at {host}. "
            "Is Ollama running? Start it with: ollama serve"
        ) from e
    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            raise ValueError(
                f"Model '{model}' not found in Ollama. "
                f"Pull it first with: ollama pull {model}"
            ) from e
        raise ConnectionError(f"Ollama API error: {e.response.text}") from e
    except httpx.TimeoutException as e:
        raise TimeoutError(
            f"Ollama model '{model}' timed out. "
            "Try a smaller model or increase your system RAM."
        ) from e

    data = resp.json()
    message = data.get("message", {})

    # Extract text content
    text_content = message.get("content", "")

    # Extract tool calls (Ollama supports them for compatible models)
    tool_calls = []
    raw_tool_calls = message.get("tool_calls", [])
    if raw_tool_calls:
        for i, tc in enumerate(raw_tool_calls):
            func = tc.get("function", {})
            args = func.get("arguments", {})
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except json.JSONDecodeError:
                    args = {}
            tool_calls.append({
                "name": func.get("name", ""),
                "input": args,
                "id": f"ollama_{func.get('name', 'tool')}_{i}",
            })

    # Token usage — Ollama provides this in the response
    input_tokens = data.get("prompt_eval_count", 0)
    output_tokens = data.get("eval_count", 0)

    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
    }

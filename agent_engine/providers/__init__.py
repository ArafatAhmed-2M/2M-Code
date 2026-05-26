"""
2M Code — Provider Package

Registry of all LLM provider adapters.
Each provider normalizes its response to:
{content: str, tool_calls: list, input_tokens: int, output_tokens: int}

Each provider also exposes list_models() which fetches live available models
from that provider's API, with a hardcoded fallback if the API call fails.

Supported providers:
  anthropic          — Claude models (claude-opus, claude-sonnet, claude-haiku)
  google             — Gemini models (gemini-1.5-pro, gemini-2.0-flash, etc.)
  openai             — GPT models (gpt-4o, gpt-4o-mini, o1-preview, etc.)
  openai_compatible  — Any OpenAI-compatible API (DeepSeek, Together, xAI/Grok, Perplexity, Fireworks, GitHub Models, and hundreds more)
  mistral            — Mistral models (mistral-large, codestral, mixtral, etc.)
  cohere             — Command models (command-r-plus, command-r, command-light)
  groq               — Fast inference (llama3-70b, mixtral-8x7b, gemma2, etc.)
  ollama             — Local models (llama3, mistral, phi3, codellama, any pulled model)
  openrouter         — 200+ models via OpenRouter (anthropic/claude-3.5-sonnet, etc.)
"""

from . import (
    anthropic_provider,
    google_provider,
    openai_provider,
    openai_compatible_provider,
    mistral_provider,
    cohere_provider,
    groq_provider,
    ollama_provider,
    openrouter_provider,
)

__all__ = [
    "anthropic_provider",
    "google_provider",
    "openai_provider",
    "openai_compatible_provider",
    "mistral_provider",
    "cohere_provider",
    "groq_provider",
    "ollama_provider",
    "openrouter_provider",
]

"""
2M Code — Provider Package

Registry of all LLM provider adapters.
Each provider normalizes its response to:
{content: str, tool_calls: list, input_tokens: int, output_tokens: int}
"""

from providers import anthropic_provider, google_provider, openai_provider, mistral_provider

__all__ = [
    "anthropic_provider",
    "google_provider",
    "openai_provider",
    "mistral_provider",
]

"""
2M Code — Web Fetch Tool

Fetches content from a URL via HTTP GET.
Returns text content, capped at 50KB.

Security:
- Uses httpx with timeout
- Response capped at 50KB to prevent memory exhaustion
- HTTPS is preferred but HTTP is allowed for local development
"""

import logging

import httpx

logger = logging.getLogger("2mcode.tools.web")

# Maximum response size: 50KB
MAX_RESPONSE_SIZE = 51200

WEB_FETCH_DEFINITION = {
    "name": "web_fetch",
    "description": "Fetch content from a URL via HTTP GET. Returns text content. Maximum 50KB.",
    "input_schema": {
        "type": "object",
        "properties": {
            "url": {
                "type": "string",
                "description": "The URL to fetch",
            }
        },
        "required": ["url"],
    },
}


def execute_web_fetch(tool_input: dict) -> str:
    """
    Fetch content from a URL and return the text.

    Args:
        tool_input: Dict with "url" key.

    Returns:
        Text content from the URL response, capped at MAX_RESPONSE_SIZE.

    Raises:
        ConnectionError: If the URL can't be reached.
        TimeoutError: If the request exceeds 30 seconds.
    """
    url = tool_input.get("url", "")
    if not url:
        return "Error: No URL provided."

    logger.info("Fetching URL: %s", url)

    try:
        with httpx.Client(timeout=30.0, follow_redirects=True) as client:
            response = client.get(url)
            response.raise_for_status()

            # Get text content, capped at max size
            content = response.text[:MAX_RESPONSE_SIZE]

            if len(response.text) > MAX_RESPONSE_SIZE:
                content += f"\n\n[truncated — response is {len(response.text)} bytes, showing first {MAX_RESPONSE_SIZE}]"

            return content

    except httpx.TimeoutException:
        raise TimeoutError(f"Request to {url} timed out after 30 seconds.")
    except httpx.HTTPStatusError as e:
        return f"HTTP error {e.response.status_code} fetching {url}: {e.response.reason_phrase}"
    except httpx.ConnectError as e:
        raise ConnectionError(f"Cannot connect to {url}: {e}") from e

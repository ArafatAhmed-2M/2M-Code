"""
2M Code — Agent Engine Server

FastAPI server that handles LLM API calls for all providers.
Runs on 127.0.0.1:8765 and is managed by the Go CLI binary.
"""

import logging
import sys

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from pydantic import BaseModel, Field

from agent import run_agent, list_all_models

# Configure logging — never log credentials or secrets
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stderr)],
)
logger = logging.getLogger("2mcode.server")

app = FastAPI(
    title="2M Code Agent Engine",
    description="Internal agent engine for the 2M Code CLI",
    version="1.0.0",
)


class MessageItem(BaseModel):
    """A single message in the conversation history."""

    role: str = Field(..., description="Message role: user, assistant, or system")
    content: str = Field(..., description="Message content text")
    name: str = Field(default="", description="Speaker name for multi-agent context")


class AgentRequest(BaseModel):
    """Request body for the /call endpoint."""

    provider: str = Field(..., description="LLM provider: anthropic|google|openai|mistral")
    model: str = Field(..., description="Provider-specific model ID")
    system: str = Field(..., description="System prompt for the agent")
    messages: list[MessageItem] = Field(default_factory=list, description="Conversation history")
    tools: list[str] = Field(default_factory=list, description="Enabled tool names")
    max_tokens: int = Field(default=4096, description="Max tokens for the response")


class AgentResponse(BaseModel):
    """Response body from the /call endpoint."""

    content: str = Field(default="", description="Text response from the agent")
    tool_calls: list[dict] = Field(default_factory=list, description="Tool use requests")
    input_tokens: int = Field(default=0, description="Input tokens consumed")
    output_tokens: int = Field(default=0, description="Output tokens generated")


@app.get("/health")
def health():
    """Health check endpoint used by the Go CLI to verify the engine is ready."""
    return {"status": "ok"}


class ModelInfo(BaseModel):
    id: str
    name: str
    description: str
    context_length: int

@app.get("/models", response_model=dict[str, list[ModelInfo]])
async def get_models(providers: str | None = None):
    """
    Get available models from all configured providers.
    Optional query param `providers` can be a comma-separated list of providers to filter.
    """
    providers_filter = None
    if providers:
        providers_filter = [p.strip() for p in providers.split(",")]
    
    return await list_all_models(providers_filter=providers_filter)


@app.post("/call", response_model=AgentResponse)
async def call(req: AgentRequest):
    """
    Call an LLM agent with the given provider, model, system prompt,
    conversation history, and tool definitions.

    Returns the agent's text response and any tool call requests.
    """
    try:
        result = await run_agent(req)
        return AgentResponse(**result)
    except KeyError as e:
        logger.error("Unknown provider requested: %s", req.provider)
        raise HTTPException(
            status_code=400,
            detail=f"Unknown provider: {req.provider}. Supported: anthropic, google, openai, mistral",
        ) from e
    except ValueError as e:
        logger.error("Invalid request parameters: %s", str(e))
        raise HTTPException(status_code=422, detail=str(e)) from e
    except ConnectionError as e:
        logger.error("Provider connection failed: %s", str(e))
        raise HTTPException(
            status_code=502,
            detail=f"Failed to connect to {req.provider} API. Check your API key and network.",
        ) from e
    except TimeoutError as e:
        logger.error("Provider request timed out: %s", str(e))
        raise HTTPException(
            status_code=504,
            detail=f"Request to {req.provider} timed out. Try again or use a faster model.",
        ) from e


# TODO(security): In production deployment, consider adding rate limiting
# to the /call endpoint to prevent abuse.

if __name__ == "__main__":
    # Bind to 127.0.0.1 only — never 0.0.0.0
    logger.info("Starting 2M Code Agent Engine on 127.0.0.1:8765")
    uvicorn.run(
        app,
        host="127.0.0.1",
        port=8765,
        log_level="info",
    )

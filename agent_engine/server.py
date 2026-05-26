"""
2M Code — Agent Engine Server

FastAPI server that handles LLM API calls for all providers.
Runs on 127.0.0.1:8765 and is managed by the Go CLI binary.
"""

import logging
import sys

import uvicorn
import json

from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from agent import run_agent, run_agent_stream, list_all_models, init_plugins, shutdown_plugins
from plugin_base import Plugin
from skill_loader import list_skills, get_skill

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
    on_startup=[lambda: init_plugins(server_app=app)],
    on_shutdown=[shutdown_plugins],
)


class MessageItem(BaseModel):
    """A single message in the conversation history."""

    role: str = Field(..., description="Message role: user, assistant, or system")
    content: str = Field(..., description="Message content text")
    name: str = Field(default="", description="Speaker name for multi-agent context")


class AgentRequest(BaseModel):
    """Request body for the /call endpoint."""

    provider: str = Field(..., description="LLM provider: anthropic|google|openai|openai_compatible|mistral|cohere|groq|ollama|openrouter")
    model: str = Field(..., description="Provider-specific model ID")
    system: str = Field(..., description="System prompt for the agent")
    messages: list[MessageItem] = Field(default_factory=list, description="Conversation history")
    tools: list[str] = Field(default_factory=list, description="Enabled tool names")
    custom_tools: list[dict] = Field(default_factory=list, description="User-defined tool definitions (name, description, input_schema)")
    max_tokens: int = Field(default=4096, description="Max tokens for the response")
    stream: bool = Field(default=False, description="If true, stream response via SSE")
    base_url: str = Field(default="", description="API base URL (openai_compatible only); overrides env var")


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


@app.post("/call")
async def call(req: AgentRequest):
    """
    Call an LLM agent. If req.stream is True, returns Server-Sent Events.
    Otherwise, returns a JSON AgentResponse.

    SSE events:
      event: text\ndata: {"content": "..."}
      event: tool_call\ndata: {"name": "...", "input": {...}, "id": "..."}
      event: done\ndata: {"input_tokens": N, "output_tokens": N}
    """
    if not req.stream:
        return await _call_non_streaming(req)

    return await _call_streaming(req)


async def _call_non_streaming(req: AgentRequest) -> AgentResponse:
    """Non-streaming call: return a complete AgentResponse."""
    try:
        result = await run_agent(req)
        return AgentResponse(**result)
    except KeyError as e:
        logger.error("Unknown provider requested: %s", req.provider)
        raise HTTPException(
            status_code=400,
            detail=f"Unknown provider: {req.provider}. Supported: anthropic, google, openai, openai_compatible (covers DeepSeek, Together, xAI, Perplexity, Fireworks, GitHub Models, and any OpenAI-compatible API), mistral, cohere, groq, ollama, openrouter",
        ) from e
    except ValueError as e:
        logger.error("Invalid request parameters: %s", str(e))
        raise HTTPException(status_code=422, detail=str(e)) from e
    except ConnectionError as e:
        logger.error("Provider connection failed: %s", str(e))
        status_code = 502
        detail = str(e)
        if "rate" in detail.lower() or "quota" in detail.lower() or "credit" in detail.lower():
            status_code = 429
        raise HTTPException(status_code=status_code, detail=detail) from e
    except TimeoutError as e:
        logger.error("Provider request timed out: %s", str(e))
        raise HTTPException(
            status_code=504,
            detail=f"Request to {req.provider} timed out. Try again or use a faster model.",
        ) from e


async def _call_streaming(req: AgentRequest):
    """Streaming call: return a StreamingResponse with SSE events."""
    async def event_stream():
        try:
            async for event_type, data in run_agent_stream(req):
                if event_type == "done":
                    yield f"event: done\ndata: {json.dumps(data)}\n\n"
                    return
                yield f"event: {event_type}\ndata: {json.dumps(data)}\n\n"
        except KeyError as e:
            yield f"event: error\ndata: {json.dumps({'detail': str(e)})}\n\n"
        except ValueError as e:
            yield f"event: error\ndata: {json.dumps({'detail': str(e)})}\n\n"
        except ConnectionError as e:
            yield f"event: error\ndata: {json.dumps({'detail': str(e)})}\n\n"
        except TimeoutError as e:
            yield f"event: error\ndata: {json.dumps({'detail': str(e)})}\n\n"

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


@app.get("/skills")
async def get_skills():
    """List all available skills with name, description, and license."""
    skills = list_skills()
    return {
        "skills": [
            {"name": s["name"], "description": s["description"], "license": s["license"]}
            for s in skills
        ]
    }


@app.get("/skills/{skill_name}")
async def get_skill_endpoint(skill_name: str):
    """Get a single skill by name, including its full content."""
    skill = get_skill(skill_name)
    if skill is None:
        raise HTTPException(status_code=404, detail=f"Skill '{skill_name}' not found")
    return {"skill": skill}


@app.get("/plugins")
async def get_plugins():
    """List all loaded plugins with their name and available hooks."""
    from agent import _plugins
    return {
        "plugins": [
            {
                "name": p.name,
                "hooks": [
                    hook
                    for hook in ["on_startup", "on_shutdown", "on_agent_turn_start", "on_agent_turn_end", "on_tool_exec"]
                    if getattr(type(p), hook) is not getattr(Plugin, hook)
                ],
            }
            for p in _plugins
        ]
    }


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

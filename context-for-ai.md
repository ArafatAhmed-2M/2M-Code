# 2M Code — AI Session Context
**Saved:** 2026-05-24  
**Purpose:** Allows AI to resume work after context restart without losing state.

---

## Session Summary

This session fixed **8 bugs** and added **1 new provider** (`openai_compatible`). All changes are committed and pushed to GitHub (`main`).

---

## Project State

| Aspect | Status |
|--------|--------|
| Go build (`go build ./cmd/2m`) | ✅ Passes |
| Go vet (`go vet ./...`) | ✅ Passes |
| Python syntax (all .py files) | ✅ Passes |
| Last commit | `a977424` — "Update all docs with 9-provider list, bug tracker, and issue log" |
| All commits pushed | ✅ Yes |
| Branch | `main` |

---

## What Was Done This Session

### Bugs Fixed (commit `e72b8c2`)

| # | What | Where |
|---|------|-------|
| 1 | Fixed circular import in `providers/__init__.py` — `from providers` → `from .` | `agent_engine/providers/__init__.py` |
| 2 | Removed duplicate `import anthropic` | `agent_engine/providers/anthropic_provider.py` |
| 3 | Cleaned up unused loop var `_ = i` hack | `internal/orchestrator/cost.go` |
| 4 | Fixed cost estimate — was using 1st agent's model for all tokens; now per-agent | `internal/orchestrator/orchestrator.go`, `cost.go` |
| 5 | CustomTool InputSchema default was applied to range-copy, not actual struct | `internal/team/team.go:242-244` |
| 6 | Team/task name swap on not-found fallback in `2m run` | `internal/cli/run.go:77-78` |
| 7 | `top_p` (sampling param, value 0-1) used as `context_length` fallback | `agent_engine/providers/openrouter_provider.py:70` |
| 8 | Custom tool `{param}` placeholders never substituted in command template | `internal/orchestrator/tools.go:50-108` |

### Provider Added (commit `f0f3a1c`)

- **`openai_compatible`** — generic adapter for any OpenAI-compatible API endpoint
- Env vars: `OPENAI_COMPATIBLE_API_KEY`, `OPENAI_COMPATIBLE_BASE_URL` (default: `https://api.openai.com/v1`)
- Supports: DeepSeek, Together AI, xAI Grok, Perplexity, Fireworks, GitHub Models, etc.
- File: `agent_engine/providers/openai_compatible_provider.py`
- Registered in: `agent.py` (PROVIDERS + _PROVIDER_ENV_VARS), `team.go` (validProviders), `config.go` (env var map)

### Docs Updated (commit `a977424`)

- `PRD.md` — provider lists, architecture diagram, goals
- `README.md` — description, API keys, requirements, roadmap
- `SETUP.md` — API key examples
- `agent.md` — V2 description, Definition of Done, repo structure
- `issue.md` — all 8 bugs + provider addition logged

---

## All 9 Providers

| Provider name (in YAML) | Env var | Notes |
|-------------------------|---------|-------|
| `anthropic` | `ANTHROPIC_API_KEY` | Claude models |
| `google` | `GOOGLE_API_KEY` | Gemini models |
| `openai` | `OPENAI_API_KEY` | GPT models |
| `openai_compatible` | `OPENAI_COMPATIBLE_API_KEY` + `OPENAI_COMPATIBLE_BASE_URL` | DeepSeek, Together, xAI, Perplexity, etc. |
| `mistral` | `MISTRAL_API_KEY` | Mistral models |
| `cohere` | `COHERE_API_KEY` | Command models |
| `groq` | `GROQ_API_KEY` | Fast LPU inference |
| `ollama` | (none) | Local models |
| `openrouter` | `OPENROUTER_API_KEY` | 200+ models via unified API |

---

## What's Still Needed (Unfinished V2 work)

These are the items documented in `agent.md` under "What's Still Needed":

1. **Tests** — No test files exist in Go or Python. The project needs a test suite.
2. **`2m history` command** — Only a stub exists in `internal/cli/team.go:173-186`. Needs to load the latest session DB and print messages with agent badges.
3. **`web_fetch` tool** — Go-side `ExecuteTool` returns a stub string "web_fetch is handled by the agent engine" instead of actually fetching a URL. Needs actual HTTP fetch logic or delegation to Python.
4. **Streaming renderer** — `PrintAgentText` prints every SSE chunk on a new line; small chunks produce fragmented output. Should buffer by newline or use a character accumulator.
5. **Chat token budget** — `RunTask` enforces `MaxTokensPerRun` but `RunChatTurn` does not.

---

## Architecture Quick Reference

### Go CLI (`cmd/2m/main.go`)
- Starts Python agent engine subprocess, health-checks it, runs Cobra CLI, kills Python on exit
- Searches for `server.py` in: `2M_ENGINE_PATH` → `~/.2mcode/agent_engine/` → relative to binary → relative to CWD
- On Windows, uses `taskkill` to free port 8765

### Go Orchestrator (`internal/orchestrator/orchestrator.go`)
- `RunTask()` — creates session, posts task, runs agents in schedule order, prints summary with per-agent cost, saves memory
- `RunChatTurn()` — same but interactive, no budget enforcement
- `runAgentTurn()` — gets history, builds request with memory context, streams via bridge, tool use loop (max 5 iterations), posts response to bus

### Go Bridge (`internal/bridge/bridge.go`)
- `Call()` — HTTP POST to `/call` (non-streaming)
- `CallStream()` — HTTP POST to `/call` with SSE reading, `onEvent` callback for each chunk
- `ListModels()` — GET `/models`
- `WaitForReady()` — polls `/health` every 200ms

### Python Agent Engine (`agent_engine/server.py`)
- FastAPI on `127.0.0.1:8765`
- `POST /call` — non-streaming returns JSON, streaming returns SSE
- `GET /health` — returns `{"status": "ok"}`
- `GET /models` — returns `{provider: [model, ...]}`

### Python Agent Router (`agent_engine/agent.py`)
- `_resolve_provider()` — returns provider module, falls back to OpenRouter if provider's env var is missing but `OPENROUTER_API_KEY` is set
- `run_agent()` — builds tool defs, calls provider's `call()`
- `run_agent_stream()` — checks `has_streaming` and `call_stream_fn`, falls back to non-streaming

### Team Loading (`internal/team/team.go`)
- Search order: `./.2mcode/teams/` → `~/.2mcode/teams/` → `~/.2mcode/config/teams/` → relative to binary → `config/teams/` (relative to CWD)
- `Validate()` checks agents, tools, workflow, sets defaults

### Memory (`internal/memory/`)
- `FileStore` — JSONL files at `~/.2mcode/memory/<team>.jsonl`, thread-safe (RWMutex)
- `Summarizer` — calls LLM via bridge to summarize session, saves entry
- `BuildContext()` — loads last 5 entries, formats as `[PAST SESSION MEMORY]` block

---

## Key File Locations

```
agent_engine/
├── server.py                        ← FastAPI server
├── agent.py                         ← Agent router
├── providers/
│   ├── __init__.py
│   ├── anthropic_provider.py
│   ├── google_provider.py
│   ├── openai_provider.py
│   ├── openai_compatible_provider.py  ← NEW (2026-05-24)
│   ├── mistral_provider.py
│   ├── cohere_provider.py
│   ├── groq_provider.py
│   ├── ollama_provider.py
│   └── openrouter_provider.py
├── tools/
│   ├── __init__.py
│   ├── bash_tool.py
│   ├── file_tool.py
│   └── web_tool.py

internal/
├── bus/
│   ├── bus.go
│   └── schema.go
├── bridge/
│   └── bridge.go
├── cli/
│   ├── root.go
│   ├── run.go
│   ├── chat.go
│   ├── team.go
│   ├── newteam.go
│   ├── models.go
│   └── renderer.go
├── memory/
│   ├── store.go
│   └── summarizer.go
├── orchestrator/
│   ├── orchestrator.go
│   ├── scheduler.go
│   ├── tools.go
│   └── cost.go
└── team/
    ├── team.go
    └── config.go

config/teams/
├── fullstack.yaml
├── code-review.yaml
└── data-science.yaml
```
